package handler

import (
	"context"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AvailableChannelHandler 处理用户侧「可用渠道」查询。
//
// 用户侧接口委托 ChannelService.ListAvailable，并在返回前做三层过滤：
//  1. 行过滤：只保留状态为 Active 且与当前用户可访问分组有交集的渠道；
//  2. 分组过滤：渠道的 Groups 只保留用户可访问的那些；
//  3. 平台过滤：渠道的 SupportedModels 只保留平台在用户可见 Groups 中出现过的模型，
//     防止"渠道同时挂在 antigravity / anthropic 两个平台的分组上，用户只访问
//     antigravity，却看到 anthropic 模型"这类跨平台信息泄漏；
//  4. 字段白名单：仅返回用户需要的字段（省略 BillingModelSource / RestrictModels
//     / 内部 ID / Status 等管理字段）。
type AvailableChannelHandler struct {
	channelService *service.ChannelService
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	resolver       *service.ModelPricingResolver
}

// NewAvailableChannelHandler 创建用户侧可用渠道 handler。
func NewAvailableChannelHandler(
	channelService *service.ChannelService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	resolver *service.ModelPricingResolver,
) *AvailableChannelHandler {
	return &AvailableChannelHandler{
		channelService: channelService,
		apiKeyService:  apiKeyService,
		settingService: settingService,
		resolver:       resolver,
	}
}

// featureEnabled 返回 available-channels 开关是否启用。默认关闭（opt-in）。
func (h *AvailableChannelHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.GetAvailableChannelsRuntime(c.Request.Context()).Enabled
}

// userAvailableGroup 用户可见的分组概要（白名单字段）。
//
// 前端据此区分专属 vs 公开分组（IsExclusive）、订阅 vs 标准分组（SubscriptionType，
// 订阅视觉加深），并展示默认倍率与高峰倍率规则；用户专属倍率前端走
// /groups/rates，和 API 密钥页面保持一致。
type userAvailableGroup struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Platform             string  `json:"platform"`
	SubscriptionType     string  `json:"subscription_type"`
	RateMultiplier       float64 `json:"rate_multiplier"`
	ImageRateIndependent bool    `json:"-"`
	ImageRateMultiplier  float64 `json:"-"`
	PeakRateEnabled      bool    `json:"peak_rate_enabled"`
	PeakStart            string  `json:"peak_start"`
	PeakEnd              string  `json:"peak_end"`
	PeakRateMultiplier   float64 `json:"peak_rate_multiplier"`
	IsExclusive          bool    `json:"is_exclusive"`
}

// userSupportedModelPricing 用户可见的定价字段白名单。
type userSupportedModelPricing struct {
	BillingMode      string                   `json:"billing_mode"`
	InputPrice       *float64                 `json:"input_price"`
	OutputPrice      *float64                 `json:"output_price"`
	CacheWritePrice  *float64                 `json:"cache_write_price"`
	CacheReadPrice   *float64                 `json:"cache_read_price"`
	ImageInputPrice  *float64                 `json:"image_input_price"`
	ImageOutputPrice *float64                 `json:"image_output_price"`
	PerRequestPrice  *float64                 `json:"per_request_price"`
	Intervals        []userPricingIntervalDTO `json:"intervals"`
}

// userPricingIntervalDTO 定价区间白名单（去掉内部 ID、SortOrder 等前端不渲染的字段）。
type userPricingIntervalDTO struct {
	MinTokens       int      `json:"min_tokens"`
	MaxTokens       *int     `json:"max_tokens"`
	TierLabel       string   `json:"tier_label,omitempty"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	PerRequestPrice *float64 `json:"per_request_price"`
}

// userSupportedModel 用户可见的支持模型条目。
type userSupportedModel struct {
	Name     string                     `json:"name"`
	Platform string                     `json:"platform"`
	Pricing  *userSupportedModelPricing `json:"pricing"`
}

// userBillingRateGroup is the group view used by the user-facing billing
// transparency endpoint. It exposes only the multiplier inputs that affect the
// user's bill: the group default and the optional per-user override.
type userBillingRateGroup struct {
	ID                       int64    `json:"id"`
	Name                     string   `json:"name"`
	Platform                 string   `json:"platform"`
	SubscriptionType         string   `json:"subscription_type"`
	DefaultRateMultiplier    float64  `json:"default_rate_multiplier"`
	CustomRateMultiplier     *float64 `json:"custom_rate_multiplier,omitempty"`
	EffectiveMultiplier      float64  `json:"effective_multiplier"`
	ImageRateIndependent     bool     `json:"image_rate_independent"`
	ImageRateMultiplier      float64  `json:"image_rate_multiplier"`
	EffectiveImageMultiplier float64  `json:"effective_image_multiplier"`
	IsExclusive              bool     `json:"is_exclusive"`
}

// userBillingRateModel is a flattened model row: one channel + one platform +
// one accessible group + one model. Flattening makes the frontend display the
// unit-price formula directly: base pricing * effective multiplier = multiplied
// unit pricing.
type userBillingRateModel struct {
	ChannelName        string                     `json:"channel_name"`
	ChannelDescription string                     `json:"channel_description"`
	Platform           string                     `json:"platform"`
	Group              userBillingRateGroup       `json:"group"`
	Model              string                     `json:"model"`
	BasePricing        *userSupportedModelPricing `json:"base_pricing"`
	EffectivePricing   *userSupportedModelPricing `json:"effective_pricing"`
	PricingSource      string                     `json:"pricing_source"`
	PricingKind        string                     `json:"pricing_kind"`
	AppliedMultiplier  float64                    `json:"applied_multiplier"`
	MultiplierType     string                     `json:"multiplier_type"`
}

type userBillingRatesResponse struct {
	Groups []userBillingRateGroup `json:"groups"`
	Models []userBillingRateModel `json:"models"`
}

// userChannelPlatformSection 单渠道内某个平台的子视图：用户可见的分组 + 该平台
// 支持的模型。按 platform 聚合后让前端可以把渠道名作为 row-group 一次渲染，
// 后面的平台行按 sections 顺序铺开。
type userChannelPlatformSection struct {
	Platform        string               `json:"platform"`
	Groups          []userAvailableGroup `json:"groups"`
	SupportedModels []userSupportedModel `json:"supported_models"`
}

// userAvailableChannel 用户可见的渠道条目（白名单字段）。
//
// 每个渠道聚合为一条记录，内嵌 platforms 子数组：每个 section 对应一个平台，
// 包含该平台的 groups 和 supported_models。
type userAvailableChannel struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Platforms   []userChannelPlatformSection `json:"platforms"`
}

// List 列出当前用户可见的「可用渠道」。
// GET /api/v1/channels/available
func (h *AvailableChannelHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Feature 未启用时返回空数组（不暴露渠道信息）。检查放在认证之后，
	// 保持与未开关前的 401 行为一致：未登录先 401，登录后再按开关决定。
	if !h.featureEnabled(c) {
		response.Success(c, []userAvailableChannel{})
		return
	}

	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	for i := range userGroups {
		allowedGroupIDs[userGroups[i].ID] = struct{}{}
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]userAvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterUserVisibleGroups(ch.Groups, allowedGroupIDs)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		if len(sections) == 0 {
			continue
		}
		out = append(out, userAvailableChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}

	response.Success(c, out)
}

// ListBillingRates returns the current user's billable view: accessible groups,
// their effective multipliers, and model prices after applying those
// multipliers. It intentionally does not expose accounts, account cost
// multipliers, routing internals, channel IDs, or disabled groups.
// GET /api/v1/billing/rates
func (h *AvailableChannelHandler) ListBillingRates(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	for i := range userGroups {
		allowedGroupIDs[userGroups[i].ID] = struct{}{}
	}

	userRates, err := h.apiKeyService.GetUserGroupRates(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	groups := buildUserBillingRateGroups(userGroups, userRates)

	// Reuse the existing Available Channels exposure boundary: group multipliers
	// are already visible through /groups/available and /groups/rates, but channel
	// and model pricing remain opt-in via the available-channels runtime switch.
	if !h.featureEnabled(c) {
		response.Success(c, userBillingRatesResponse{Groups: groups, Models: []userBillingRateModel{}})
		return
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	_, models := buildUserBillingRates(c.Request.Context(), channels, allowedGroupIDs, userRates, h.resolver)
	response.Success(c, userBillingRatesResponse{Groups: groups, Models: models})
}

// buildPlatformSections 把一个渠道按 visibleGroups 的平台集合拆成有序的 section 列表：
// 每个 section 对应一个平台，只包含该平台的 groups 和 supported_models。
// 输出按 platform 字母序稳定排序，便于前端等效比较与回归测试。
func buildPlatformSections(
	ch service.AvailableChannel,
	visibleGroups []userAvailableGroup,
) []userChannelPlatformSection {
	groupsByPlatform := make(map[string][]userAvailableGroup, 4)
	for _, g := range visibleGroups {
		if g.Platform == "" {
			continue
		}
		groupsByPlatform[g.Platform] = append(groupsByPlatform[g.Platform], g)
	}
	if len(groupsByPlatform) == 0 {
		return nil
	}

	platforms := make([]string, 0, len(groupsByPlatform))
	for p := range groupsByPlatform {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	sections := make([]userChannelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		platformSet := map[string]struct{}{platform: {}}
		sections = append(sections, userChannelPlatformSection{
			Platform:        platform,
			Groups:          groupsByPlatform[platform],
			SupportedModels: toUserSupportedModels(ch.SupportedModels, platformSet),
		})
	}
	return sections
}

// filterUserVisibleGroups 仅保留用户可访问的分组。
func filterUserVisibleGroups(
	groups []service.AvailableGroupRef,
	allowed map[int64]struct{},
) []userAvailableGroup {
	visible := make([]userAvailableGroup, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[g.ID]; !ok {
			continue
		}
		visible = append(visible, userAvailableGroup{
			ID:                   g.ID,
			Name:                 g.Name,
			Platform:             g.Platform,
			SubscriptionType:     g.SubscriptionType,
			RateMultiplier:       g.RateMultiplier,
			ImageRateIndependent: g.ImageRateIndependent,
			ImageRateMultiplier:  g.ImageRateMultiplier,
			PeakRateEnabled:      g.PeakRateEnabled,
			PeakStart:            g.PeakStart,
			PeakEnd:              g.PeakEnd,
			PeakRateMultiplier:   g.PeakRateMultiplier,
			IsExclusive:          g.IsExclusive,
		})
	}
	return visible
}

func buildUserBillingRates(
	ctx context.Context,
	channels []service.AvailableChannel,
	allowed map[int64]struct{},
	userRates map[int64]float64,
	resolver *service.ModelPricingResolver,
) ([]userBillingRateGroup, []userBillingRateModel) {
	groupByID := make(map[int64]userBillingRateGroup)
	rows := make([]userBillingRateModel, 0)

	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterUserVisibleGroups(ch.Groups, allowed)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		for _, section := range sections {
			for _, group := range section.Groups {
				billingGroup := toUserBillingRateGroup(group, userRates)
				groupByID[billingGroup.ID] = billingGroup

				for _, model := range section.SupportedModels {
					base, source := resolveUserBillingPricing(ctx, resolver, group.ID, model.Name, model.Pricing)
					multiplier, multiplierType := billingMultiplierForPricing(base, billingGroup)
					rows = append(rows, userBillingRateModel{
						ChannelName:        ch.Name,
						ChannelDescription: ch.Description,
						Platform:           section.Platform,
						Group:              billingGroup,
						Model:              model.Name,
						BasePricing:        base,
						EffectivePricing:   multiplyUserPricing(base, multiplier),
						PricingSource:      source,
						PricingKind:        "unit_price_table",
						AppliedMultiplier:  multiplier,
						MultiplierType:     multiplierType,
					})
				}
			}
		}
	}

	groups := make([]userBillingRateGroup, 0, len(groupByID))
	for _, group := range groupByID {
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Platform != groups[j].Platform {
			return groups[i].Platform < groups[j].Platform
		}
		if groups[i].Name != groups[j].Name {
			return groups[i].Name < groups[j].Name
		}
		return groups[i].ID < groups[j].ID
	})
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Platform != rows[j].Platform {
			return rows[i].Platform < rows[j].Platform
		}
		if rows[i].ChannelName != rows[j].ChannelName {
			return rows[i].ChannelName < rows[j].ChannelName
		}
		if rows[i].Model != rows[j].Model {
			return rows[i].Model < rows[j].Model
		}
		return rows[i].Group.Name < rows[j].Group.Name
	})

	return groups, rows
}

func buildUserBillingRateGroups(groups []service.Group, userRates map[int64]float64) []userBillingRateGroup {
	out := make([]userBillingRateGroup, 0, len(groups))
	for i := range groups {
		g := groups[i]
		available := userAvailableGroup{
			ID:                   g.ID,
			Name:                 g.Name,
			Platform:             g.Platform,
			SubscriptionType:     g.SubscriptionType,
			RateMultiplier:       g.RateMultiplier,
			ImageRateIndependent: g.ImageRateIndependent,
			ImageRateMultiplier:  g.ImageRateMultiplier,
			IsExclusive:          g.IsExclusive,
		}
		out = append(out, toUserBillingRateGroup(available, userRates))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func toUserBillingRateGroup(group userAvailableGroup, userRates map[int64]float64) userBillingRateGroup {
	var custom *float64
	effective := group.RateMultiplier
	if rate, ok := userRates[group.ID]; ok {
		r := rate
		custom = &r
		effective = rate
	}
	imageMultiplier := effective
	if group.ImageRateIndependent {
		imageMultiplier = group.ImageRateMultiplier
		if imageMultiplier < 0 {
			imageMultiplier = 0
		}
	}
	return userBillingRateGroup{
		ID:                       group.ID,
		Name:                     group.Name,
		Platform:                 group.Platform,
		SubscriptionType:         group.SubscriptionType,
		DefaultRateMultiplier:    group.RateMultiplier,
		CustomRateMultiplier:     custom,
		EffectiveMultiplier:      effective,
		ImageRateIndependent:     group.ImageRateIndependent,
		ImageRateMultiplier:      group.ImageRateMultiplier,
		EffectiveImageMultiplier: imageMultiplier,
		IsExclusive:              group.IsExclusive,
	}
}

func resolveUserBillingPricing(
	ctx context.Context,
	resolver *service.ModelPricingResolver,
	groupID int64,
	model string,
	displayPricing *userSupportedModelPricing,
) (*userSupportedModelPricing, string) {
	if resolver == nil {
		return displayPricing, "display"
	}
	resolved := resolver.Resolve(ctx, service.PricingInput{Model: model, GroupID: &groupID})
	if resolved == nil {
		return nil, ""
	}
	if resolved.Source != service.PricingSourceChannel && isNonTokenPricing(displayPricing) {
		return displayPricing, "display"
	}
	pricing := toUserPricingFromResolved(resolved)
	if !userPricingHasPrice(pricing) {
		return nil, resolved.Source
	}
	return pricing, resolved.Source
}

func isNonTokenPricing(p *userSupportedModelPricing) bool {
	return p != nil && p.BillingMode != "" && p.BillingMode != string(service.BillingModeToken)
}

func toUserPricingFromResolved(resolved *service.ResolvedPricing) *userSupportedModelPricing {
	if resolved == nil {
		return nil
	}
	billingMode := string(resolved.Mode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	out := &userSupportedModelPricing{BillingMode: billingMode}

	switch resolved.Mode {
	case service.BillingModePerRequest, service.BillingModeImage:
		out.PerRequestPrice = positiveFloatPtr(resolved.DefaultPerRequestPrice)
		out.Intervals = toUserPricingIntervals(resolved.RequestTiers)
	default:
		out.Intervals = toUserPricingIntervals(resolved.Intervals)
		if resolved.BasePricing != nil {
			out.InputPrice = positiveFloatPtr(resolved.BasePricing.InputPricePerToken)
			out.OutputPrice = positiveFloatPtr(resolved.BasePricing.OutputPricePerToken)
			out.CacheWritePrice = firstPositiveFloatPtr(
				resolved.BasePricing.CacheCreationPricePerToken,
				resolved.BasePricing.CacheCreation5mPrice,
			)
			out.CacheReadPrice = positiveFloatPtr(resolved.BasePricing.CacheReadPricePerToken)
			if resolved.BasePricing.ImageOutputPriceExplicit {
				out.ImageOutputPrice = floatPtr(resolved.BasePricing.ImageOutputPricePerToken)
			} else {
				out.ImageOutputPrice = positiveFloatPtr(resolved.BasePricing.ImageOutputPricePerToken)
			}
		}
	}
	return out
}

func toUserPricingIntervals(intervals []service.PricingInterval) []userPricingIntervalDTO {
	out := make([]userPricingIntervalDTO, 0, len(intervals))
	for _, iv := range intervals {
		out = append(out, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      iv.InputPrice,
			OutputPrice:     iv.OutputPrice,
			CacheWritePrice: iv.CacheWritePrice,
			CacheReadPrice:  iv.CacheReadPrice,
			PerRequestPrice: iv.PerRequestPrice,
		})
	}
	return out
}

func billingMultiplierForPricing(
	pricing *userSupportedModelPricing,
	group userBillingRateGroup,
) (float64, string) {
	if pricing != nil && pricing.BillingMode == string(service.BillingModeImage) {
		return group.EffectiveImageMultiplier, "image"
	}
	return group.EffectiveMultiplier, "standard"
}

func userPricingHasPrice(p *userSupportedModelPricing) bool {
	if p == nil {
		return false
	}
	if p.InputPrice != nil || p.OutputPrice != nil ||
		p.CacheWritePrice != nil || p.CacheReadPrice != nil ||
		p.ImageOutputPrice != nil || p.PerRequestPrice != nil {
		return true
	}
	for _, iv := range p.Intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil {
			return true
		}
	}
	return false
}

func positiveFloatPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return floatPtr(v)
}

func firstPositiveFloatPtr(values ...float64) *float64 {
	for _, value := range values {
		if value != 0 {
			return floatPtr(value)
		}
	}
	return nil
}

func floatPtr(v float64) *float64 {
	out := v
	return &out
}

// toUserSupportedModels 将 service 层支持模型转换为用户 DTO（字段白名单）。
// 仅保留平台在 allowedPlatforms 中的条目，防止跨平台模型信息泄漏。
// allowedPlatforms 为 nil 时不做平台过滤（保留全部，供测试或明确无过滤场景使用）。
func toUserSupportedModels(
	src []service.SupportedModel,
	allowedPlatforms map[string]struct{},
) []userSupportedModel {
	out := make([]userSupportedModel, 0, len(src))
	for i := range src {
		m := src[i]
		if allowedPlatforms != nil {
			if _, ok := allowedPlatforms[m.Platform]; !ok {
				continue
			}
		}
		out = append(out, userSupportedModel{
			Name:     m.Name,
			Platform: m.Platform,
			Pricing:  toUserPricing(m.Pricing),
		})
	}
	return out
}

// toUserPricing 将 service 层定价转换为用户 DTO；入参为 nil 时返回 nil。
func toUserPricing(p *service.ChannelModelPricing) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(p.Intervals))
	for _, iv := range p.Intervals {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      iv.InputPrice,
			OutputPrice:     iv.OutputPrice,
			CacheWritePrice: iv.CacheWritePrice,
			CacheReadPrice:  iv.CacheReadPrice,
			PerRequestPrice: iv.PerRequestPrice,
		})
	}
	billingMode := string(p.BillingMode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	return &userSupportedModelPricing{
		BillingMode:      billingMode,
		InputPrice:       p.InputPrice,
		OutputPrice:      p.OutputPrice,
		CacheWritePrice:  p.CacheWritePrice,
		CacheReadPrice:   p.CacheReadPrice,
		ImageInputPrice:  p.ImageInputPrice,
		ImageOutputPrice: p.ImageOutputPrice,
		PerRequestPrice:  p.PerRequestPrice,
		Intervals:        intervals,
	}
}

func multiplyUserPricing(p *userSupportedModelPricing, multiplier float64) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	out := &userSupportedModelPricing{
		BillingMode:      p.BillingMode,
		InputPrice:       multiplyFloatPtr(p.InputPrice, multiplier),
		OutputPrice:      multiplyFloatPtr(p.OutputPrice, multiplier),
		CacheWritePrice:  multiplyFloatPtr(p.CacheWritePrice, multiplier),
		CacheReadPrice:   multiplyFloatPtr(p.CacheReadPrice, multiplier),
		ImageOutputPrice: multiplyFloatPtr(p.ImageOutputPrice, multiplier),
		PerRequestPrice:  multiplyFloatPtr(p.PerRequestPrice, multiplier),
		Intervals:        make([]userPricingIntervalDTO, 0, len(p.Intervals)),
	}
	for _, iv := range p.Intervals {
		out.Intervals = append(out.Intervals, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      multiplyFloatPtr(iv.InputPrice, multiplier),
			OutputPrice:     multiplyFloatPtr(iv.OutputPrice, multiplier),
			CacheWritePrice: multiplyFloatPtr(iv.CacheWritePrice, multiplier),
			CacheReadPrice:  multiplyFloatPtr(iv.CacheReadPrice, multiplier),
			PerRequestPrice: multiplyFloatPtr(iv.PerRequestPrice, multiplier),
		})
	}
	return out
}

func multiplyFloatPtr(v *float64, multiplier float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v * multiplier
	return &out
}
