# 站内作图工作台(Image Studio / 作图工作台)— 设计文档

- 日期:2026-06-06
- 分支:`feature/image-studio`(从 `dev` 切出)
- 参考:`/home/miku/workspack/chatgpt2api`(对话式作图工作台的 UX 参考)
- 状态:**已按代码自检修订(v2)**,待用户复核 → writing-plans

> **v2 修订说明**:v1 假设"`ImageStudioService` 在 service 层薄薄复用生成逻辑"。代码自检(并行核验 24 项承诺:15 确认/3 部分/6 错)证明该假设**不成立**——生成编排被困在 gin handler、无"用户→分组"路径、`usage_logs.api_key_id` 为 NOT NULL、并发器 handler 私有。v2 改为"**合成内部 API Key + 抽共享编排器**"的可行路线,详见 §3/§4/§4.5。

## 1. 概述

在 sub2api 内实现**登录用户可用的对话式作图工作台**:站内页面输入提示词生成图片,**消耗走站内余额/配额**,结果**持久化** + **历史画廊**。面向浏览器 **JWT 登录态**(区别于现有 `/v1/images/*` 的外部 API Key 调用)。

## 2. 目标 / 非目标

**目标(终态)**:OpenAI(gpt-image/DALL·E)文生图;成本计入站内余额;结果服务端持久化 + 历史画廊;**多线程对话**工作台(仿 chatgpt2api)。
**交付**:MVP 薄切片优先、分阶段(§10)。
**暂缓(Phase 2+)**:图生图/参考图、异步队列、成本预估接口、S3/R2 存储、保留期清理;Gemini/Antigravity(Phase 3)。

## 3. 关键决策(头脑风暴 + 自检后)

| 维度 | 决定 |
|---|---|
| 鉴权/入口 | 新建 **JWT** 路由 `/api/v1/user/image-studio/*` |
| **用户→分组** | **UI 让用户在其 `AllowedGroups` 中选分组**;该分组驱动计费/账号选择 |
| **管线复用方式** | **合成内部 API Key**:为 (用户, 选中分组) 惰性创建一把隐藏的"作图专用" api_key;站内接口"以该 key 身份"驱动现有 image 管线 |
| **生成编排** | **抽出共享编排器** `GenerateImage(ctx, input)`(选择+forward+failover+用量),现有 gateway handler 与新作图接口都调它(消除重复) |
| 计费口径 | 复用 `CalculateImageCost(model, imageSize, imageCount, *ImagePriceConfig, rateMultiplier)` → `*CostBreakdown`;`UsageService.Create`(事务扣余额),用量记到合成 key 的 id 上 |
| usage_log 约束 | 合成 key 使 `usage_logs.api_key_id`(NOT NULL)天然满足,**无需改表** |
| 访问控制 | 选中分组的 `Group.AllowImageGeneration` 标志(已存在 `ent/schema/group.go`) |
| 存储 | **本地磁盘**(MVP)→ 可插拔 `ImageStore` 接口 → S3/R2(配置后切换) |
| 持久化/会话 | 新表 `image_conversation` + `image_generation`;**多线程**(Phase 1 即含) |
| 同步/异步 | MVP 同步 + SSE;异步队列 Phase 2 |
| Provider | v1 仅 OpenAI |

## 4. 架构(终态)

```
浏览器(JWT)  ── group 选择器只列 user.AllowedGroups
  │ POST /api/v1/user/image-studio/generate {conversation_id?, group_id, prompt, model, size, quality, n}
  ▼ ImageStudioHandler(JWT,取当前 user)
  │   1. 校验 group_id ∈ user.AllowedGroups 且 Group.AllowImageGeneration
  │   2. 取/惰性建 该 (user,group) 的“作图专用”合成 api_key
  │   3. 取/建 image_conversation
  ▼ GenerateImage(ctx, input{user, apiKey(合成), group, params})   ← 新共享编排器(从 handler 抽出)
  │   a. CheckBillingEligibility(余额/配额)
  │   b. 占并发槽(导出后的 image 限流器) ── defer 释放
  │   c. for: SelectAccountWithSchedulerForImages(groupID) → ForwardImages(account) →
  │          处理 UpstreamFailoverError/换账号(至 max-switch 上限)
  │   d. 成功:得到图片字节/尺寸
  ▼ ImageStore.Put → 引用;落 image_generation;
  ▼ 扣费:CalculateImageCost → UsageService.Create(api_key_id=合成 key;事务:usage_log+扣余额)
  ▼ 返回 {generation_id, images:[{url}], cost, balance}
```

**后端组件**
- 路由:`internal/server/routes/user.go` 的 JWT `authenticated` 组下新增 `image-studio` 子组;handler 经 `GetAuthSubjectFromContext` 取当前 user。
- **`GenerateImage` 共享编排器**(新,见 §4.5 抽取范围):封装 eligibility→并发→选择→forward→failover→用量。**现有 `handler/openai_images.go` 改为也调它**(行为不变),新作图 handler 同样调它。
- `ImageStudioService` / handler(新):做 group 校验、合成 key 解析、会话管理、存储、落库,然后调 `GenerateImage`。
- **合成 api_key 助手**(新):`EnsureStudioAPIKey(ctx, userID, groupID) -> *APIKey`,惰性创建一把标记为内部、且**不在用户密钥列表中展示**的 key(标记方式见 §5)。
- `ImageStore` 接口 + 本地实现(同 v1):`Put/OpenURL/Delete`;本地落 `DATA_DIR/images/user_{id}/{genID}/{idx}.png`,经 JWT + 归属校验的 `GET /user/image-studio/assets/:genID/:idx` 回传。S3/R2 实现 Phase 2。
- 仓储:`image_generation_repo.go`(按用户/会话分页)。

**前端**:`views/user/ImageStudioView.vue`(左=会话线程列表;中=turn 时间线;底=composer 含**分组选择器**[列 `GET /groups/available`]+模型+比例/尺寸+质量+n;顶=实时余额+预估费用)。配套 `api/imageStudio.ts`、`stores/imageStudio.ts`、`AppSidebar` nav(featureFlag 控)、router、i18n。

## 4.5 重构范围(Phase 1 前置,自检新增)

现状:`handler/openai_images.go:23-354` 在 gin handler 内做 APIKey 解析、计费检查、占槽、**账号选择+failover 循环**;`service/openai_images.go:ForwardImages` 只处理单个**已选**账号且**硬依赖 `gin.Context`**(还往 c 写 ops 指标);并发器 `imageConcurrencyLimiter` 为 handler 私有、**全局**计数。

Phase 1 必须先做(此为本功能主要工作量,非"薄包"):
1. **抽 `GenerateImage(ctx, input) (*Result, error)` 编排器**:把 `openai_images.go:146-353` 的选择+failover+max-switch+用量逻辑下沉到 service;让现有 gateway handler 与新作图 handler 都调它。
2. **解耦 `ForwardImages` 的 gin/ops 依赖**:接受 nil 容忍的 ops sink(或显式 ops 接口),使非 handler 调用可用。
3. **导出并发器**:`imageConcurrencyLimiter` 抽到共享包 / 定义接口经 `wire.go` 注入。**MVP 沿用全局语义**(够用);按用户限流留 Phase 2(可接现有 `ConcurrencyService`)。
4. **回归保证**:重构后现有 `/v1/images/*` 行为、计费、failover、ops 指标**不变**(以现有 image handler 测试 + 新增测试守住)。

## 5. 数据模型(ent schema,新增 + 1 处小增)

- `image_conversation`:`id, user_id(FK), title, created_at, updated_at`(+ 软删除 mixin)。
- `image_generation`:`id, user_id(FK), conversation_id(FK), group_id, prompt, model, size, quality, n, image_count, status(pending/succeeded/failed), cost, storage_keys(JSON), width, height, error, created_at`。
- **合成 key 标记**:给 `api_keys` 增加一个**加法**字段(如 `internal bool` 或 `purpose string`,默认空)标识"作图专用",并在用户密钥列表查询处过滤掉。加法列、低风险;若不想动表则退化为保留命名 + 查询过滤(较脆,不推荐)。
- 迁移:新表/列用**高位前缀** `9xx_*.sql`(与上游顺序号错开)+ `make generate`。计费真实记录仍在 `usage_log`(已有 `image_count/image_size` 等字段,注意它们在 `UsageLog` 结构上,不在 `CreateUsageLogRequest`,成本由调用方预算)。

## 6. API(`/api/v1/user/image-studio`,JWT)
- `POST /generate`(body 含 `group_id`)、`GET/POST /conversations`、`PATCH/DELETE /conversations/:id`、`GET /conversations/:id/generations`、`GET /generations`(画廊分页)、`GET /assets/:genID/:idx`(本地存储回传)、`DELETE /generations/:id`。
- 分组选择器复用现有 `GET /groups/available`。
- (Phase 2)参考图 multipart、`GET /cost-estimate`。

## 7. 计费与并发(复用,签名已校正)
- 预检:`CheckBillingEligibility`(非 gateway 亦可调,自检确认)。
- 成本:`CalculateImageCost(model string, imageSize string, imageCount int, groupConfig *ImagePriceConfig, rateMultiplier float64) *CostBreakdown`;批量 n 即图数相乘。
- 扣费:`UsageService.Create`(单事务:usage_log + 扣余额 + 失效缓存,自检确认可独立调用)——唯一扣费入口;`api_key_id` 用合成 key;`TotalCost/ActualCost` 由调用方预算。
- 并发:导出后的 image 限流器(MVP 全局)。

## 8. 存储(同 v1)
本地 `DATA_DIR/images/...`(注意 `/app/data` 不可写会降级,生产显式设可写 `DATA_DIR`);JWT + 归属校验的 assets 路由回传。S3/R2(Phase 2):新增 `image_storage_*` 配置 + `s3ImageStore`(抽 `backup_s3_store.go` 的 client,自检确认其当前仅绑备份配置),预签名直链;切换对上层/前端透明。保留期清理 Phase 2。

## 9. 错误处理
- **顺序**:预检余额 → 占并发槽 → 生成 → 存储 → 扣费(`UsageService.Create`)→ 落库(succeeded)→ 返回;`defer` 释放槽。
- **生成/存储失败**:记 `status=failed`,**不扣费**,释放槽,返回错误。
- **扣费失败**(罕见系统错误,余额已预检):记 `[CRITICAL]` 对账日志,generation 标 succeeded(图已产出),仍返回图;**不构成白嫖**(预检已保证余额,扣费失败属系统异常)。
- **同步超时**:4K/批量限上限 + SSE;Phase 2 异步队列。

## 10. 分阶段交付
- **Phase 1(含重构 + 多线程 + 本地存储)**:§4.5 重构(抽 `GenerateImage` + 解耦 ForwardImages + 导出限流器)→ 合成 key 助手 → JWT `/generate`(OpenAI 文生图、单图+小批量)→ 本地 `ImageStore` + assets 路由 → `image_conversation/image_generation` 表 → 计费 → 工作台 UI(会话列表 + turn 时间线 + composer 含分组选择器 + 实时余额/预估)+ 画廊读取。同步 + SSE。
- **Phase 2**:图生图/参考图、异步队列、cost-estimate、**S3/R2 存储**、保留期清理、按用户并发限流。
- **Phase 3**:Gemini/Antigravity、收藏/标签、批量导出。

## 11. 测试
- 后端:`GenerateImage` 编排器(选择/failover/eligibility 拒绝/失败不扣费)单测;**现有 `/v1/images/*` 回归**(重构不改行为);`ImageStudioService`(group 校验/合成 key 解析/会话/存储归属/扣费原子)单测;`ImageStore` 本地实现读写 + 归属校验。
- 前端:`stores/imageStudio` + composer 提交/状态流转 vitest。

## 12. 改动清单(Phase 1)
**后端**:`service/image_generate_orchestrator.go`(抽取,新)、改 `handler/openai_images.go`(改调编排器,行为不变)、改 `service/openai_images.go`(ForwardImages 去 gin/ops 耦合)、导出 `image_concurrency_limiter`、`service/image_studio_service.go` + `image_store.go`(+local)、`service/studio_api_key.go`(合成 key 助手)、`repository/image_generation_repo.go`、`ent/schema/{image_conversation,image_generation}.go` + `api_keys` 加 `internal` 列(+ `make generate`)、`migrations/9xx_image_studio.sql`、`handler/image_studio_handler.go`、`routes/user.go`、wire 注册。
**前端**:`views/user/ImageStudioView.vue`、`api/imageStudio.ts`、`stores/imageStudio.ts`、`AppSidebar.vue`、`router/index.ts`、`i18n/locales/{en,zh}.ts`。

## 13. 风险(自检后)
1. **重构是主要工作量与主要风险**:抽 `GenerateImage` 必须保持现有 `/v1/images/*` 计费/failover/ops 行为不变 —— 用现有+新增测试守回归。
2. **合成 key 生命周期**:惰性创建、绑分组、用户改 AllowedGroups/分组停用时的处理;务必从用户密钥列表与配额视图中排除。
3. **同步 4K/批量超时**:MVP 限 n/尺寸 + SSE;Phase 2 异步。
4. **本地存储仅过渡**:单机、占磁盘;`DATA_DIR` 须可写且显式配置;尽快切 S3/R2(已留接口)。
5. **OAuth 图片桥**(`openai_images_responses.go`)较脆:Phase 1 先验 API Key 路径,OAuth 次选。
6. 并发器 MVP 为全局语义(非按用户)——按用户限流 Phase 2。

## 14. 自检已确认可直接依赖的支柱(无需改)
`UsageService.Create` 事务且可独立调用;`CheckBillingEligibility` 可非 gateway 调用;`Group.AllowImageGeneration` 字段存在;S3 client 可抽出;`GetDataDir` 兜底;ent/迁移约定(9xx 前缀安全、软删除 mixin);JWT 用户路由 + 取 user;前端 view/api/store/sidebar/router/i18n 范式。
