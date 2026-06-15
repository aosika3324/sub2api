package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Veo 视频生成接入。
//
// Veo 与普通 generateContent 不同，是异步长任务模型：
//  1. 提交：POST /v1beta/models/{model}:predictLongRunning -> 返回 operation name（任务句柄）
//  2. 轮询：GET  /v1beta/{operationName}                    -> done:true 时返回视频 file URI
//  3. 下载：GET  {fileURI}（需带 api_key）                  -> 视频字节流
//
// 这条路径独立于 ForwardNative（后者是为同步 generateContent 设计的 500+ 行逻辑），
// 仅支持 API Key 账号（AI Studio，x-goog-api-key 鉴权）。计费在轮询完成时按视频秒数结算。

const (
	// veoActionPredict 是 Veo 提交视频生成任务的 action。
	veoActionPredict = "predictLongRunning"
	// veoOperationSessionPrefix 是 operation->account 粘性绑定的 sessionHash 前缀。
	veoOperationSessionPrefix = "veo_op:"
	// veoBilledSessionPrefix 是 operation 已计费标记的 sessionHash 前缀（防重复扣费）。
	veoBilledSessionPrefix = "veo_billed:"
	// veoOperationBindingTTL 是 operation->account 绑定的存活时间。
	// Veo 任务通常几分钟内完成，留足轮询窗口（geminiStickySessionTTL 为 1 小时，足够）。
	veoOperationBindingTTL = geminiStickySessionTTL
	// veoUpstreamFileReadLimit 限制单次代理下载/响应读取的字节数（512MB）。
	veoUpstreamFileReadLimit = 512 << 20
	// veoDefaultVideoDurationSeconds 是无法从响应解析时长时的兜底秒数。
	veoDefaultVideoDurationSeconds = 8.0
)

// IsVeoModel 判断模型名是否为 Veo 视频生成模型。
func IsVeoModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "veo")
}

// IsVeoAction 判断 action 是否为 Veo 提交任务的 action。
func IsVeoAction(action string) bool {
	return strings.TrimSpace(action) == veoActionPredict
}

// VeoSubmitResult 是 Veo 提交任务的结果。
type VeoSubmitResult struct {
	StatusCode    int
	OperationName string // 上游返回的 operation name（用于轮询绑定）
	Body          []byte // 透传给客户端的原始响应体
	Headers       http.Header
}

// VeoPollResult 是 Veo 轮询任务的结果。
type VeoPollResult struct {
	StatusCode      int
	Done            bool
	Success         bool
	DurationSeconds float64 // 完成时解析出的视频总时长（秒），用于计费
	Body            []byte  // 透传给客户端的（可能改写过 file URI 的）响应体
	Headers         http.Header
}

// buildVeoAPIKeyRequest 构造带 x-goog-api-key 的上游请求（仅 API Key 账号）。
func (s *GeminiMessagesCompatService) buildVeoAPIKeyRequest(
	ctx context.Context, account *Account, method, fullURL string, body []byte,
) (*http.Request, error) {
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, errors.New("gemini api_key not configured")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-goog-api-key", apiKey)
	return req, nil
}

// veoProxyURL 返回账号配置的代理 URL（无则空）。
func veoProxyURL(account *Account) string {
	if account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

// BindVeoOperation 将 operation name 绑定到提交时选中的账号，供后续轮询命中同一上游。
func (s *GeminiMessagesCompatService) BindVeoOperation(
	ctx context.Context, groupID *int64, operationName string, accountID int64,
) error {
	operationName = strings.TrimSpace(operationName)
	if operationName == "" {
		return errors.New("empty operation name")
	}
	return s.cache.SetSessionAccountID(
		ctx, derefGroupID(groupID), veoOperationSessionPrefix+operationName, accountID, veoOperationBindingTTL,
	)
}

// LookupVeoOperationAccount 根据 operation name 取回绑定的账号；未绑定返回 0。
func (s *GeminiMessagesCompatService) LookupVeoOperationAccount(
	ctx context.Context, groupID *int64, operationName string,
) (int64, error) {
	operationName = strings.TrimSpace(operationName)
	if operationName == "" {
		return 0, errors.New("empty operation name")
	}
	return s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), veoOperationSessionPrefix+operationName)
}

// GetVeoAccount 按账号 ID 加载账号（用于轮询/下载时取回提交阶段绑定的上游账号）。
func (s *GeminiMessagesCompatService) GetVeoAccount(ctx context.Context, accountID int64) (*Account, error) {
	return s.getSchedulableAccount(ctx, accountID)
}

// MarkVeoBilledIfFirst 尝试将 operation 标记为已计费，返回 true 表示本次是首次（应计费）。
// 用于防止同一 operation 在多次轮询完成后被重复扣费。
//
// 复用 int64 session 缓存做 get-then-set：非严格 SETNX，但 Veo 轮询不会高频并发，
// 足以挡住"客户端反复轮询已完成任务"造成的重复扣费。
func (s *GeminiMessagesCompatService) MarkVeoBilledIfFirst(
	ctx context.Context, groupID *int64, operationName string,
) bool {
	operationName = strings.TrimSpace(operationName)
	if operationName == "" {
		return false
	}
	key := veoBilledSessionPrefix + operationName
	gid := derefGroupID(groupID)
	if existing, err := s.cache.GetSessionAccountID(ctx, gid, key); err == nil && existing > 0 {
		return false // 已计费
	}
	if err := s.cache.SetSessionAccountID(ctx, gid, key, 1, veoOperationBindingTTL); err != nil {
		// 标记失败时保守放行计费（宁可漏挡也不漏扣），但记录便于排查。
		logger.LegacyPrintf("service.gemini_veo", "mark veo billed failed op=%s: %v", operationName, err)
	}
	return true
}

// SubmitVeo 提交 Veo 视频生成任务（predictLongRunning），仅支持 API Key 账号。
func (s *GeminiMessagesCompatService) SubmitVeo(
	ctx context.Context, account *Account, model string, body []byte,
) (*VeoSubmitResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("veo only supports api_key accounts, got: %s", account.Type)
	}

	mappedModel := account.GetMappedModel(model)
	baseURL, err := s.validateUpstreamBaseURL(account.GetGeminiBaseURL(geminicli.AIStudioBaseURL))
	if err != nil {
		return nil, err
	}
	fullURL := fmt.Sprintf("%s/v1beta/models/%s:%s", strings.TrimRight(baseURL, "/"), mappedModel, veoActionPredict)

	req, err := s.buildVeoAPIKeyRequest(ctx, account, http.MethodPost, fullURL, body)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpUpstream.Do(req, veoProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, veoUpstreamFileReadLimit))
	operationName := ""
	if resp.StatusCode < 400 {
		operationName = strings.TrimSpace(gjson.GetBytes(respBody, "name").String())
	}

	return &VeoSubmitResult{
		StatusCode:    resp.StatusCode,
		OperationName: operationName,
		Body:          respBody,
		Headers:       responseheaders.FilterHeaders(resp.Header, s.responseHeaderFilter),
	}, nil
}

// PollVeo 轮询 Veo 任务状态。operationName 形如 "models/veo-3.x/operations/abc" 或 "operations/abc"。
func (s *GeminiMessagesCompatService) PollVeo(
	ctx context.Context, account *Account, operationName string,
) (*VeoPollResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	operationName = strings.TrimSpace(strings.TrimPrefix(operationName, "/"))
	if operationName == "" {
		return nil, errors.New("empty operation name")
	}

	baseURL, err := s.validateUpstreamBaseURL(account.GetGeminiBaseURL(geminicli.AIStudioBaseURL))
	if err != nil {
		return nil, err
	}
	fullURL := fmt.Sprintf("%s/v1beta/%s", strings.TrimRight(baseURL, "/"), operationName)

	req, err := s.buildVeoAPIKeyRequest(ctx, account, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpUpstream.Do(req, veoProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, veoUpstreamFileReadLimit))

	result := &VeoPollResult{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    responseheaders.FilterHeaders(resp.Header, s.responseHeaderFilter),
	}
	if resp.StatusCode >= 400 {
		return result, nil
	}

	result.Done = gjson.GetBytes(respBody, "done").Bool()
	if result.Done {
		// done 但带 error 字段表示失败，不计费。
		result.Success = !gjson.GetBytes(respBody, "error").Exists()
		if result.Success {
			result.DurationSeconds = extractVeoDurationSeconds(respBody)
		}
	}
	return result, nil
}

// extractVeoDurationSeconds 从 Veo 完成响应中解析视频总时长（秒）。
// Veo 响应里每个生成样本可能带 duration（如 "8s" 或数值秒）；累加所有样本。
// 解析失败时回退到 veoDefaultVideoDurationSeconds。
func extractVeoDurationSeconds(body []byte) float64 {
	total := 0.0
	// 常见路径：response.generateVideoResponse.generatedSamples[].video.durationSeconds
	// 不同 API 版本字段名略有差异，这里宽松匹配多种 duration 字段。
	gjson.GetBytes(body, "response.generateVideoResponse.generatedSamples").ForEach(func(_, sample gjson.Result) bool {
		total += parseDurationField(sample)
		return true
	})
	if total <= 0 {
		// 兜底：扫描 response.videos[] 或 predictions[]
		gjson.GetBytes(body, "response.videos").ForEach(func(_, v gjson.Result) bool {
			total += parseDurationField(v)
			return true
		})
	}
	if total <= 0 {
		return veoDefaultVideoDurationSeconds
	}
	return total
}

// parseDurationField 从一个 JSON 节点尝试多种字段名提取秒数。
func parseDurationField(node gjson.Result) float64 {
	for _, path := range []string{"video.durationSeconds", "durationSeconds", "video.duration", "duration"} {
		r := node.Get(path)
		if !r.Exists() {
			continue
		}
		if r.Type == gjson.Number {
			return r.Float()
		}
		// 字符串形式如 "8s"
		if secs := parseSecondsString(r.String()); secs > 0 {
			return secs
		}
	}
	return 0
}

// parseSecondsString 解析 "8s" / "8" 形式的秒数字符串。
func parseSecondsString(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "s"))
	if s == "" {
		return 0
	}
	if d, err := time.ParseDuration(s + "s"); err == nil {
		return d.Seconds()
	}
	return 0
}

// veoFileURIPaths 是 Veo 完成响应里可能出现视频 file URI 的 gjson/sjson 路径模板，
// %d 处填入样本下标。覆盖不同 API 版本的字段命名差异。
var veoFileURIPaths = []string{
	"response.generateVideoResponse.generatedSamples.%d.video.uri",
	"response.generateVideoResponse.generatedSamples.%d.video.fileUri",
	"response.videos.%d.uri",
}

// veoSampleContainers 是承载样本数组的容器路径，用于按下标遍历。
var veoSampleContainers = []string{
	"response.generateVideoResponse.generatedSamples",
	"response.videos",
}

// RewriteVeoFileURIs 将完成响应里的所有视频 file URI 重写为网关代理 URL。
// rewrite(index) 返回第 index 个样本应替换成的 URL；返回空串表示该样本不重写。
// 这样客户端拿到的是网关地址（携带网关 key 即可下载），真实 Google api_key 不外泄。
func RewriteVeoFileURIs(body []byte, rewrite func(index int) string) []byte {
	out := body
	for _, container := range veoSampleContainers {
		arr := gjson.GetBytes(out, container)
		if !arr.IsArray() {
			continue
		}
		count := len(arr.Array())
		for i := 0; i < count; i++ {
			proxyURL := rewrite(i)
			if proxyURL == "" {
				continue
			}
			for _, tmpl := range veoFileURIPaths {
				path := fmt.Sprintf(tmpl, i)
				if !gjson.GetBytes(out, path).Exists() {
					continue
				}
				if updated, err := sjson.SetBytes(out, path, proxyURL); err == nil {
					out = updated
				}
			}
		}
	}
	return out
}

// ResolveVeoFileURI 重新轮询 operation，返回第 index 个样本的原始 file URI（已通过 host 校验）。
// 用于代理下载时在服务端取回权威 URI，避免信任客户端传入的 URI（消除 SSRF）。
func (s *GeminiMessagesCompatService) ResolveVeoFileURI(
	ctx context.Context, account *Account, operationName string, index int,
) (string, error) {
	poll, err := s.PollVeo(ctx, account, operationName)
	if err != nil {
		return "", err
	}
	if poll.StatusCode >= 400 {
		return "", fmt.Errorf("veo operation poll status %d", poll.StatusCode)
	}
	if !poll.Done || !poll.Success {
		return "", errors.New("veo operation not completed")
	}
	uri := veoFileURIAtIndex(poll.Body, index)
	if uri == "" {
		return "", fmt.Errorf("no veo file uri at index %d", index)
	}
	if _, err := s.validateUpstreamBaseURL(uri); err != nil {
		return "", fmt.Errorf("invalid veo file uri: %w", err)
	}
	return uri, nil
}

// veoFileURIAtIndex 取第 index 个样本的 file URI（尝试多种字段路径）。
func veoFileURIAtIndex(body []byte, index int) string {
	for _, tmpl := range veoFileURIPaths {
		path := fmt.Sprintf(tmpl, index)
		if uri := strings.TrimSpace(gjson.GetBytes(body, path).String()); uri != "" {
			return uri
		}
	}
	return ""
}

// ProxyVeoFile 代理下载 Veo 生成的视频文件（注入 api_key），返回原始响应流。
// fileURI 必须是服务端经 ResolveVeoFileURI 取回并校验过的 URI，调用方负责关闭 resp.Body。
func (s *GeminiMessagesCompatService) ProxyVeoFile(
	ctx context.Context, account *Account, fileURI string,
) (*http.Response, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	fileURI = strings.TrimSpace(fileURI)
	if fileURI == "" {
		return nil, errors.New("empty file uri")
	}
	if _, err := s.validateUpstreamBaseURL(fileURI); err != nil {
		return nil, fmt.Errorf("invalid veo file uri: %w", err)
	}

	req, err := s.buildVeoAPIKeyRequest(ctx, account, http.MethodGet, fileURI, nil)
	if err != nil {
		return nil, err
	}
	// 文件下载需要原始流，调用方负责关闭 resp.Body。
	return s.httpUpstream.Do(req, veoProxyURL(account), account.ID, account.Concurrency)
}
