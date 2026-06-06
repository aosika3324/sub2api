# 站内作图工作台(Image Studio / 作图工作台)— 设计文档

- 日期:2026-06-06
- 分支:`feature/image-studio`(从 `dev` 切出)
- 参考:`/home/miku/workspack/chatgpt2api`(对话式作图工作台的 UX 参考)
- 状态:待用户复核

## 1. 概述

在 sub2api 内实现一个**登录用户可用的对话式作图工作台**:用户在站内页面输入提示词生成图片,**消耗走站内余额/配额(站内资源)**,生成结果**持久化**并提供**历史画廊**。区别于现有 `/v1/images/*`(面向外部 API Key 调用),本功能面向**浏览器 JWT 登录态**,把生成成本计入用户余额。

**关键杠杆**:sub2api 已有生产级的图片生成管线、计费与并发控制,本功能**复用**它们,只新增"JWT 入口 + 编排 + 存储 + 会话/画廊持久化 + 前端工作台"。

## 2. 目标 / 非目标

**目标(终态)**
- 登录用户在站内工作台用 OpenAI 模型(gpt-image / DALL·E)文生图。
- 成本计入站内余额(复用现有图片计费 + 原子扣费)。
- 生成结果**服务端持久化** + **历史画廊**。
- **多对话线程**(conversation)的工作台 UX(仿 chatgpt2api)。

**交付方式**:MVP 薄切片优先,分阶段逼近终态(见 §10)。

**非目标 / 暂缓(Phase 2+)**
- 图生图/参考图编辑(`/edits`)、批量异步队列、成本预估接口 → Phase 2。
- Gemini / Antigravity 等其它 provider → Phase 3(service 代码在,但未接路由)。
- 对象存储(S3/R2):**当前无可用对象存储**,故 MVP 用本地磁盘;存储层做成**可插拔接口**,日后配好 S3/R2 一键切换。

## 3. 关键决策(头脑风暴结论)

| 维度 | 决定 |
|---|---|
| 鉴权/入口 | 新建 **JWT 鉴权**的 `/api/v1/user/image-studio/*` 接口,内部复用现有 image service(不绕过计费/并发/失败转移) |
| 计费口径 | 复用**每图 × 尺寸档(1K/2K/4K)× 分组倍率**(`CalculateImageCost`)+ `UsageService.Create`(事务扣余额) |
| 访问控制 | 复用分组 `allow_image_generation` 开关 |
| 存储 | **本地磁盘**(MVP)→ 可插拔 `ImageStore` 接口 → S3/R2(配置后切换) |
| 持久化 | **服务端**:`image_conversation` + `image_generation` 两张新表;计费仍照常写 `usage_log` |
| 会话 | **多线程**对话(Phase 1 即含) |
| 同步/异步 | MVP 同步 + SSE;异步队列 Phase 2 |
| Provider | v1 仅 OpenAI |

## 4. 架构(终态)

```
浏览器(JWT)
  │ POST /api/v1/user/image-studio/generate {conversation_id?, prompt, model, size, quality, n}
  ▼ ImageStudioService(新,编排层)
     1. 权限:用户所属分组 allow_image_generation
     2. 预检:BillingCacheService.CheckBillingEligibility(余额/配额)
     3. 并发:imageConcurrencyLimiter.Acquire(用户/全局)  ── defer Release
     4. 会话:无 conversation_id 则新建 image_conversation
     5. 生成:复用 openai_images service(账号调度→上游→失败转移),拿到图片字节/尺寸
     6. 存储:ImageStore.Put(user,gen,idx,bytes) → 内部可访问引用(本地 key 或 S3 key)
     7. 扣费:CalculateImageCost → UsageService.Create(事务:usage_log 插入 + 扣 users.balance)
     8. 落库:image_generation 行(prompt/model/size/quality/n/storage_keys/dims/status=succeeded/cost)
  ▼ 返回 {generation_id, images:[{url}], cost, balance}
```
任一步失败:释放并发槽、标记 generation 失败、**不扣费**(扣费在生成+存储成功后、返回前)。

**后端组件**
- 路由:`internal/server/routes/user.go` 的 JWT `authenticated` 组下新增 `image-studio` 子组。
- `ImageStudioService`(新文件 `internal/service/image_studio_service.go`):编排上述链;依赖现有 image service 函数、`imageConcurrencyLimiter`、`CalculateImageCost`、`UsageService`、`ImageStore`、新仓储。
- `ImageStore` 接口(新 `internal/service/image_store.go` + 实现):
  - `Put(ctx, userID, genID, idx, contentType, data) (ref string, err error)`、`OpenURL/Presign(ref)`、`Delete(ref)`。
  - **本地实现**`localImageStore`:写到 `DATA_DIR/images/user_{id}/{genID}/{idx}.png`;经一个 JWT 鉴权的 `GET /user/image-studio/assets/:genID/:idx` 路由回传(校验该 generation 属于当前用户)。
  - **S3 实现**`s3ImageStore`:抽出 `repository/backup_s3_store.go` 的 aws-sdk client,独立 bucket/凭据配置;`user_{id}/...` 前缀 + 预签名 URL。按配置选择,接口对上层透明。
- 仓储:`internal/repository/image_generation_repo.go`(CRUD + 按用户/会话分页)。

**前端组件**(对话式工作台)
- `views/user/ImageStudioView.vue`:左=会话线程列表(新建/重命名/删除);中=选中会话的 turn 时间线(每 turn:提示词+参数 → 结果图网格,带状态/耗时/重试);底=composer(提示词、模型、比例/尺寸、质量、数量 n);顶=实时余额 + 预估费用。
- `api/imageStudio.ts`、`stores/imageStudio.ts`、`AppSidebar.vue` nav 项(受 `allow_image_generation` + featureFlag 控)、`router/index.ts`(`requiresAuth`)、`i18n/locales/{en,zh}.ts`。

## 5. 数据模型(ent schema,新增)

`image_conversation`:`id, user_id(FK), title, created_at, updated_at`(+ 软删除 mixin,与项目惯例一致)。

`image_generation`:`id, user_id(FK), conversation_id(FK), prompt, model, size, quality, n, image_count, status(pending/succeeded/failed), cost(decimal), storage_keys([]string/JSON), width, height, error, created_at`。

> 计费真实记录仍在 `usage_log`(已有 image 字段);`image_generation` 面向工作台展示/画廊/重试,与计费解耦。新增迁移用**高位前缀** `9xx_*.sql`(避免与上游顺序号冲突);schema 改后 `make generate`。

## 6. API surface(`/api/v1/user/image-studio`,全部 JWT)
- `POST /generate` — 生成(取/建 conversation)。
- `GET /conversations` / `POST /conversations` / `PATCH /conversations/:id`(重命名)/ `DELETE /conversations/:id`。
- `GET /conversations/:id/generations` — 会话内 turn 列表;`GET /generations`(全局画廊,分页)。
- `GET /assets/:genID/:idx` — 本地存储时回传图片(校验归属)。
- `DELETE /generations/:id` — 删除(同时删存储对象)。
- (Phase 2)`POST /generate` 支持参考图(multipart)、`GET /cost-estimate`。

## 7. 计费与并发(全部复用)
- 预检:`BillingCacheService.CheckBillingEligibility`(Redis 优先,DB 兜底)。
- 成本:`CalculateImageCost(model, size, count, groupImagePriceConfig, rateMultiplier)`;批量 n 即图数相乘。
- 扣费:`UsageService.Create`(单事务:usage_log + 扣余额 + 失效缓存)——**唯一扣费入口,不重造**。
- 并发:`imageConcurrencyLimiter`(用户/全局槽 + 背压),`defer Release`。

## 8. 存储细节
- MVP 本地:`DATA_DIR/images/...`;`DATA_DIR` 解析见 setup.GetDataDir。需保证目录可写(注意当前 `/app/data` 不可写会降级,本地开发落到工作目录;生产应显式设 `DATA_DIR`)。
- 服务:`GET /user/image-studio/assets/:genID/:idx` 经 JWT + 归属校验后 `io.Copy` 文件;设置合适 `Content-Type`/`Cache-Control`。
- S3/R2(Phase 2):新增管理端 `image_storage_*` 配置(endpoint/bucket/key/secret/region/public-base/presign-ttl);`s3ImageStore` 预签名直链。切换不影响上层与前端(都用返回的 url)。
- 保留/清理:Phase 2 增加按用户配额或天数的清理任务。

## 9. 错误处理
- **执行顺序**:预检余额 → 占并发槽 → 生成 → 存储 → 扣费(`UsageService.Create`)→ 落库(status=succeeded)→ 返回;`defer` 释放并发槽。
- **生成失败**:落 `image_generation.status=failed` + error,**不扣费**,释放槽,返回错误。
- **存储失败**:同样**不扣费**,记 failed,返回错误。
- **扣费失败**(罕见系统错误,余额已在预检通过):记 `[CRITICAL]` 对账日志,该 generation 标 `succeeded`(图已产出),仍返回图片。**不构成“白嫖”**——预检已保证余额充足,扣费失败属 DB/系统异常而非用户可操纵路径,用对账兜底而非阻断用户。
- **同步超时**:4K/大批量可能超时 → MVP 限制 n 与尺寸上限 + SSE 流式;Phase 2 异步队列。

## 10. 分阶段交付
- **Phase 1(MVP 端到端,含多线程 + 本地存储)**:`POST /generate`(OpenAI 文生图、单图+小批量)+ `image_conversation`/`image_generation` 表 + 本地 `ImageStore` + assets 路由 + 计费 + 工作台 UI(会话列表 + turn 时间线 + composer + 实时余额/预估)+ 会话/画廊读取。同步 + SSE。
- **Phase 2**:图生图/参考图上传、异步队列(仿 chatgpt2api queue runner)、cost-estimate 接口、**S3/R2 存储**、保留期清理。
- **Phase 3**:Gemini/Antigravity provider、收藏/标签、批量导出。

## 11. 测试
- 后端:`ImageStudioService`(预检拒绝/并发释放/扣费原子性/失败不扣费/会话创建)单测(stub image service + repo);`ImageStore` 本地实现读写 + 归属校验单测;计费数值用现有 `CalculateImageCost` 测试覆盖。
- 前端:`stores/imageStudio` 与 composer 提交/状态流转的 vitest;assets 鉴权由后端测试覆盖。

## 12. 改动清单(Phase 1)
**后端(新增为主)**:`internal/service/image_studio_service.go`、`image_store.go`(+local 实现)、`internal/repository/image_generation_repo.go`、`ent/schema/image_conversation.go` + `image_generation.go`(+ `make generate`)、`migrations/9xx_image_studio.sql`、`internal/handler/image_studio_handler.go`、`internal/server/routes/user.go`(挂子路由)、wire 注册。
**前端(新增为主)**:`views/user/ImageStudioView.vue`、`api/imageStudio.ts`、`stores/imageStudio.ts`、`AppSidebar.vue`(加 nav 项)、`router/index.ts`、`i18n/locales/{en,zh}.ts`。
> 绝大多数为新增文件;对既有文件仅在 routes/sidebar/router/i18n/wire 注册点插入——符合 fork“加法优先”。

## 13. 风险
1. **JWT↔现有 image 管线接缝**:必须复用而非绕过计费/并发/失败转移(唯一结构性新活)。
2. **同步 4K/批量超时**:MVP 最可能的故障点;以尺寸/n 上限 + SSE 缓解,Phase 2 异步化。
3. **本地存储**:仅单机可用、占磁盘;生产应尽快切 S3/R2(已预留接口)。`DATA_DIR` 必须可写且显式配置。
4. **OpenAI OAuth→Responses 图片桥**(`openai_images_responses.go`)较脆;Phase 1 先验证 API Key 路径,OAuth 路径作为次选。

## 14. 待实现计划阶段细化
- 扣费失败的精确策略(见 §9 权衡);composer 参数与 OpenAI 模型/尺寸枚举的精确映射;assets 路由的缓存/防盗链;前端会话状态与异步轮询的具体形态(Phase 2)。
