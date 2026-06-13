# Image Studio — Phase 1 Backend 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: 用 superpowers:subagent-driven-development(推荐)或 superpowers:executing-plans 按任务逐条实现。步骤用 `- [ ]` 复选框跟踪。每个任务的代码以"现有签名 + 要写的测试 + 落点"为准;重构类任务给出精确的"搬迁源行 + 新签名 + 回归测试",新代码给出完整代码。

**Goal:** 让登录用户经 JWT 接口生成 OpenAI 图片、计入站内余额、结果持久化(本地)并可按会话查询——后端可用 curl 端到端验证(前端为 Plan B)。

**Architecture:** 不绕过现有 image 管线:① 把 gin handler 里的"选账号+forward+failover+记用量"循环抽成共享 service 方法 `GenerateImages`;② 站内作图接口(JWT)经"为(用户,所选分组)惰性创建的隐藏合成 API Key"以现有身份模型驱动该方法;③ 新增 `image_conversation`/`image_generation` 表 + 本地 `ImageStore` 持久化。计费/用量复用 `OpenAIGatewayService.RecordUsage`(内部含 image 字段 + `CalculateImageCost` + 事务扣费)。

**Tech Stack:** Go(ent ORM + Gin + Wire DI)、PostgreSQL、`-tags=unit` 测试(miniredis + go-sqlmock)。规范见 `DEV_GUIDE.md`:改 ent schema 后 `make generate`;迁移用高位前缀 `9xx_*.sql`;`make test-unit`。

**前置:** 已在分支 `feature/image-studio`(从 dev 切出)。spec:`docs/superpowers/specs/2026-06-06-image-studio-design.md`。

---

## 文件结构(本计划新建/修改)

**重构(既有文件,行为不变 + 回归守护):**
- `backend/internal/handler/image_concurrency_limiter.go` — 导出为可复用类型/接口。
- `backend/internal/service/openai_images.go` — `ForwardImages` 去 `gin.Context`/ops 硬耦合(接受可空 ops sink)。
- `backend/internal/handler/openai_images.go` — 把"选账号+forward+failover+RecordUsage"循环改为调用新 service 方法 `GenerateImages`(行为不变)。

**新增(后端):**
- `backend/internal/service/image_generate_orchestrator.go` — `GenerateImages(ctx, ImageGenInput) (*ImageGenResult, error)`(从 handler 抽出的共享编排)。
- `backend/internal/service/image_store.go` + `image_store_local.go` — `ImageStore` 接口 + 本地磁盘实现。
- `backend/internal/service/studio_api_key.go` — `EnsureStudioAPIKey(ctx, userID, groupID) (*APIKey, error)` 合成隐藏 key。
- `backend/internal/service/image_studio_service.go` — 站内编排:校验分组 → 合成 key → 会话 → `GenerateImages` → 存储 → 落库。
- `backend/internal/repository/image_studio_repo.go` — `image_conversation`/`image_generation` 仓储。
- `backend/ent/schema/image_conversation.go`、`image_generation.go` — 新表;`api_key.go` 加 `internal` 列。
- `backend/migrations/900_image_studio.sql` — 建表 + 加列(checksum 锁定,新文件)。
- `backend/internal/handler/image_studio_handler.go` — JWT handler。
- `backend/internal/server/routes/user.go` — 挂 `image-studio` 子路由。
- wire:`service/wire.go`、`repository/wire.go`、`handler/wire.go` 注册。

---

## Task 1:导出并发限流器(重构,行为不变)

**Files:** Modify `backend/internal/handler/image_concurrency_limiter.go`;Test `backend/internal/handler/image_concurrency_limiter_test.go`(已存在则补用例)

现状:`type imageConcurrencyLimiter struct{...}`(未导出),方法 `Acquire(ctx, enabled, limit, wait, timeout, maxWaiting) (func(), bool)`、`TryAcquire(enabled, limit) (func(), bool)`,作为 `OpenAIGatewayHandler` 私有字段。目标:**移到 service 包**(供编排器与 handler 共用),保持 API 不变。

- [ ] **Step 1: 写失败测试** —— 新建 `backend/internal/service/image_concurrency_limiter_test.go`(`//go:build unit`):构造 `&ImageConcurrencyLimiter{}`,`limit=1` 下第一次 `Acquire(ctx,true,1,false,0,0)` 返回 `(release, true)`,未释放时第二次返回 `(_, false)`,`release()` 后再次成功。
- [ ] **Step 2: 运行验证失败** —— `cd backend && go test -tags=unit ./internal/service/ -run ImageConcurrencyLimiter -v` → 编译失败(类型不存在)。
- [ ] **Step 3: 实现** —— 把 `image_concurrency_limiter.go` 整体移到 `backend/internal/service/image_concurrency_limiter.go`,`package service`,类型重命名为导出 `ImageConcurrencyLimiter`,方法签名不变。在 handler 包保留一个别名 `type imageConcurrencyLimiter = service.ImageConcurrencyLimiter`(或直接改 `OpenAIGatewayHandler` 字段类型为 `*service.ImageConcurrencyLimiter`)以零行为改动。
- [ ] **Step 4: 运行验证通过** —— `go test -tags=unit ./internal/service/ -run ImageConcurrencyLimiter -v` PASS;`go build ./...` 通过。
- [ ] **Step 5: 提交** —— `git add -A && git commit -m "refactor(image): export ImageConcurrencyLimiter to service pkg"`

## Task 2:`ForwardImages` 去 gin/ops 硬耦合(重构,行为不变)

**Files:** Modify `backend/internal/service/openai_images.go`(`ForwardImages` ~538;ops 写入点如 `SetOpsLatencyMs`)

现状:`ForwardImages(c *gin.Context, ...)` 直接往 `c` 写 ops 指标 → 非 handler 调用不可用。目标:让 ops 写入对 `c==nil` 容忍(或抽一个 `opsSink` 接口,handler 传真实、编排器传 nil/no-op),其余不变。

- [ ] **Step 1: 写测试** —— `image_forward_nilctx_test.go`:以 `c=nil` 调 `ForwardImages` 的 ops 写入辅助(把 ops 写入抽成内部函数 `writeImageOpsLatency(c, ms)`,对 nil 安全),断言 nil 时不 panic。
- [ ] **Step 2: 运行失败** —— `go test -tags=unit ./internal/service/ -run ForwardImagesNilCtx -v` → panic/编译失败。
- [ ] **Step 3: 实现** —— 把对 `c` 的 ops 写入集中到小函数并加 `if c == nil { return }` 守卫;`ForwardImages` 内所有 `c` 解引用加 nil 判断。不改正常(c!=nil)路径行为。
- [ ] **Step 4: 通过** —— 该测试 + 现有 image 测试 `go test -tags=unit ./internal/service/ ./internal/handler/ -run Image -v` 全绿(回归)。
- [ ] **Step 5: 提交** —— `refactor(image): make ForwardImages ops-writes nil-context tolerant`

## Task 3:抽共享编排器 `GenerateImages`(重构核心,行为不变)

**Files:** Create `backend/internal/service/image_generate_orchestrator.go`;Modify `backend/internal/handler/openai_images.go`(把 ~146-353 的循环替换为调用)

定义输入/输出(放在新文件):
```go
type ImageGenInput struct {
    User      *User
    APIKey    *APIKey      // 现有路径=请求 key;作图路径=合成 key
    Group     *Group       // 已解析分组(来自 APIKey.Group)
    Parsed    *ParsedOpenAIImageRequest // 现有解析结构(沿用)
    Inbound   string       // endpoint 标识
    UserAgent string
    IPAddress string
}
type ImageGenResult struct {
    Result      *OpenAIForwardResult // 含 ImageCount/ImageSize/响应体
    Account     *Account
    SwitchCount int
}
// GenerateImages 选账号→ForwardImages→failover→返回结果;不记用量(调用方决定记)。
func (s *OpenAIGatewayService) GenerateImages(ctx context.Context, in ImageGenInput) (*ImageGenResult, error)
```
- [ ] **Step 1: 写"回归等价"测试** —— `image_orchestrator_test.go`:用现有 image 测试的 stub(scheduler 返回账号、ForwardImages 返回带 ImageCount 的 result),断言 `GenerateImages` 成功路径返回 result/account,且失败(`UpstreamFailoverError`)时按 max-switch 上限换账号(沿用现有常量)。
- [ ] **Step 2: 运行失败** —— `go test -tags=unit ./internal/service/ -run GenerateImages -v` → 未定义。
- [ ] **Step 3: 实现** —— 把 `handler/openai_images.go:146-353` 的"`SelectAccountWithSchedulerForImages` → `ForwardImages` → 处理 `UpstreamFailoverError`/换账号/max-switch"逻辑**搬迁**到 `GenerateImages`,入参用 `ImageGenInput`(不再依赖 `gin.Context`,ops 用 Task 2 的 nil-tolerant);**RecordUsage 留在调用方**。
- [ ] **Step 4: 改 handler 调用** —— `handler/openai_images.go` 改为:解析→占槽(Task1 限流器)→ `s.gatewayService.GenerateImages(ctx, in)` → 成功后 `RecordUsage(...)`(沿用现有 input 构造,见 :323)。删除被搬走的内联循环。
- [ ] **Step 5: 回归** —— `make test-unit`(尤其 `./internal/handler/ -run Image`、`./internal/service/ -run Image`)全绿;`go build ./...`。手测:现有 `/v1/images/generations`(APIKey)行为/计费不变。
- [ ] **Step 6: 提交** —— `refactor(image): extract GenerateImages orchestrator shared by gateway handler`

## Task 4:ent schema —— 新表 + api_keys.internal 列

**Files:** Create `backend/ent/schema/image_conversation.go`、`image_generation.go`;Modify `backend/ent/schema/api_key.go`;然后 `make generate`

- [ ] **Step 1: 写 schema** —— 镜像现有 schema(软删除 mixin 见现有用法)。`image_conversation`:`user_id(int64)`、`title(string, default "")`、时间戳;`image_generation`:`user_id`、`conversation_id`、`group_id`、`prompt(text)`、`model`、`size`、`quality`、`n(int)`、`image_count(int)`、`status(string, default "pending")`、`cost(float64)`、`storage_keys(JSON []string)`、`width/height(int, optional)`、`error(string, optional)`、时间戳;加 user/conversation 索引。`api_key.go` 加 `field.Bool("internal").Default(false)`。
- [ ] **Step 2: 生成** —— `cd backend && make generate`(= `go generate ./ent` + `./cmd/server`),`git add ent/`。
- [ ] **Step 3: 写迁移** —— `backend/migrations/900_image_studio.sql`:`CREATE TABLE image_conversations(...)`、`image_generations(...)`(列与索引同 schema;遵循现有迁移风格 + 软删除部分唯一索引惯例);`ALTER TABLE api_keys ADD COLUMN internal BOOLEAN NOT NULL DEFAULT false;`
- [ ] **Step 4: 验证** —— `go build ./...`;启动后端确认迁移自动应用无错(`curl localhost:8080/health`)。
- [ ] **Step 5: 提交** —— `feat(image): add image_conversation/image_generation schema + api_keys.internal`

## Task 5:仓储 image_studio_repo

**Files:** Create `backend/internal/repository/image_studio_repo.go`;Test `image_studio_repo_test.go`(集成 `//go:build integration` 或用 ent enttest 内存)

接口(放 service 层定义,repo 实现,与项目仓储惯例一致):
```go
type ImageStudioRepository interface {
    CreateConversation(ctx, userID int64, title string) (*ent.ImageConversation, error)
    ListConversations(ctx, userID int64, page, size int) ([]*ent.ImageConversation, int, error)
    CreateGeneration(ctx, g *ent.ImageGeneration) (*ent.ImageGeneration, error)
    UpdateGenerationStatus(ctx, id int64, status string, storageKeys []string, cost float64, errMsg string) error
    GetGeneration(ctx, id int64) (*ent.ImageGeneration, error)
    ListGenerations(ctx, userID int64, conversationID *int64, page, size int) ([]*ent.ImageGeneration, int, error)
}
```
- [ ] Step 1-5(TDD):用 ent enttest(内存 sqlite,见现有 repo 集成测试范式)测 Create/Update/List 归属过滤;实现;`make test-integration`(或 unit enttest)通过;提交 `feat(image): image studio repository`。

## Task 6:合成隐藏 API Key 助手

**Files:** Create `backend/internal/service/studio_api_key.go`;Test `studio_api_key_test.go`

```go
// EnsureStudioAPIKey 取/惰性建 (userID, groupID) 的隐藏作图 key(internal=true)。
func (s *ImageStudioService) EnsureStudioAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error)
```
- [ ] Step 1: 测试 —— 首次调用创建 internal=true 的 key 且 group_id 正确;再次调用返回同一把(幂等);该 key 不出现在用户密钥列表查询(`api_key_repo` 列表需加 `internal=false` 过滤——一并改并测)。
- [ ] Step 2-5:实现(复用现有 api_key 创建/哈希逻辑,见 `api_key_service.go`;name 固定如 `"__image_studio__"`);改 `ListByUser` 过滤 internal;`make test-unit` 通过;提交 `feat(image): synthetic internal studio api key`。

## Task 7:ImageStore 接口 + 本地实现

**Files:** Create `backend/internal/service/image_store.go`、`image_store_local.go`;Test `image_store_local_test.go`

```go
type ImageStore interface {
    Put(ctx, userID, genID int64, idx int, contentType string, data []byte) (key string, err error)
    Open(ctx, key string) (io.ReadCloser, string, error) // reader, contentType
    Delete(ctx, key string) error
}
```
本地实现:根目录 `setup.GetDataDir()+"/images"`;key = `user_{userID}/{genID}/{idx}.png`;`Put` 建目录+写文件;`Open` 校验路径在根内(防穿越)后打开。
- [ ] Step 1: 测试 —— Put 后 Open 读回一致;路径穿越(`..`)被拒;Delete 生效。临时 DATA_DIR 用 `t.TempDir()` + 注入根目录(构造函数 `NewLocalImageStore(rootDir string)`)。
- [ ] Step 2-5:实现;`go test -tags=unit ./internal/service/ -run ImageStoreLocal` 通过;提交 `feat(image): local ImageStore`。

## Task 8:ImageStudioService(站内编排)

**Files:** Create `backend/internal/service/image_studio_service.go`;Test `image_studio_service_test.go`

```go
type ImageStudioGenerateInput struct {
    UserID, GroupID int64
    ConversationID  *int64
    Prompt, Model, Size, Quality string
    N int
}
type ImageStudioGenerateResult struct {
    GenerationID int64
    Images       []string // 可访问 url/key
    Cost         float64
    Balance      float64
}
func (s *ImageStudioService) Generate(ctx, userID int64, in ImageStudioGenerateInput) (*ImageStudioGenerateResult, error)
```
流程(按 spec §9 顺序):
1. 取 User;校验 `in.GroupID ∈ user.AllowedGroups`(用现有 allowed-groups 查询)且 `Group.AllowImageGeneration`(`GroupAllowsImageGeneration`)。
2. `CheckBillingEligibility`(预检余额/配额)。
3. 占并发槽(`ImageConcurrencyLimiter.Acquire`,enabled/limit 来自设置或常量),`defer release`。
4. 取/建会话;插入 `image_generation`(status=pending)。
5. `EnsureStudioAPIKey(user, group)` → 合成 key;构造 `ParsedOpenAIImageRequest`(model/size/quality/n/prompt)→ `GenerateImages(ctx, ImageGenInput{User,APIKey:合成,Group,Parsed,...})`。
6. 成功:对每张图 `ImageStore.Put` 收集 keys;`RecordUsage(&OpenAIRecordUsageInput{Result, APIKey:合成, User, Account, InboundEndpoint:"image_studio", ...})`(记用量+扣费,含 image 字段);`UpdateGenerationStatus(succeeded, keys, cost)`;返回。
7. 失败:`UpdateGenerationStatus(failed, nil, 0, err)`,不记用量,返回错误(并发槽 defer 释放)。
- [ ] Step 1: 测试(stub repo + stub gatewayService.GenerateImages/RecordUsage + 内存 ImageStore + stub user/group 读取):
  - 分组不在 AllowedGroups → 拒绝,不生成不扣费。
  - 成功 → 生成 succeeded、调用 RecordUsage 一次、ImageStore.Put n 次、返回 cost/images。
  - GenerateImages 失败 → 标 failed、**不调 RecordUsage**。
- [ ] Step 2-5:实现;`make test-unit` 通过;提交 `feat(image): ImageStudioService orchestration + billing`。

## Task 9:JWT Handler + 路由 + wire

**Files:** Create `backend/internal/handler/image_studio_handler.go`;Modify `routes/user.go`、`handler/wire.go`、`service/wire.go`、`repository/wire.go`;Test `image_studio_handler_test.go`

Handler(取当前用户用 `GetAuthSubjectFromContext`,见现有 user handler 范式):
- `POST /generate` {conversation_id?, group_id, prompt, model, size, quality, n} → `ImageStudioService.Generate` → JSON {generation_id, images, cost, balance}。
- `GET /conversations`、`POST /conversations`、`GET /conversations/:id/generations`、`GET /generations`、`DELETE /generations/:id`。
- `GET /assets/:genID/:idx`:取 generation 校验 `user_id==当前用户` → `ImageStore.Open(key)` → 设 Content-Type + `io.Copy`。
路由:`routes/user.go` 的 `authenticated.Group("/user")` 下 `imageStudio := user.Group("/image-studio")` 挂上述;handler 经 wire 注入 `Handlers` 结构。
- [ ] Step 1: 测试 —— handler 单测(stub service):`/generate` 参数校验 + 透传;`/assets` 归属校验(他人 genID → 403/404)。沿用现有 handler 测试范式(gin test context + stub)。
- [ ] Step 2-5:实现 + wire 注册(`ProvideImageStudioService`、`ProvideImageStudioRepository`、handler 构造加入 `Handlers`);`go build ./...`;`make test-unit` 通过;提交 `feat(image): image studio JWT handler + routes`。

## Task 10:端到端冒烟 + 收尾

- [ ] **Step 1:** `cd backend && make test-unit`(+ 相关 integration)全绿;`go build ./...`;`gofmt`/`golangci-lint run ./...` 无新增问题。
- [ ] **Step 2:** 手测(后端 :8080 + 一个有 `allow_image_generation` 分组且配了 OpenAI 账号的环境):登录拿 JWT → `POST /api/v1/user/image-studio/generate` → 200 + images + 余额下降;`GET /generations` 见记录;`GET /assets/...` 拿到图;余额/usage_log 正确(合成 key 的 api_key_id)。
- [ ] **Step 3:** 提交 + 推送分支(需你的凭据)。

---

## 自检(spec 覆盖)
- 用户→分组:Task 8 step1 校验 AllowedGroups + AllowImageGeneration ✓
- 合成 key 解 usage_log NOT NULL:Task 6 + Task 8 用合成 key 的 id 走 RecordUsage ✓
- 抽编排器(blocker 1/5/6):Task 1-3 ✓
- 本地存储 + assets 归属:Task 7 + Task 9 ✓
- 多线程会话:Task 4/5/9(conversations + per-conversation generations)✓
- 计费复用 RecordUsage(含 image 字段):Task 8 ✓(修正 spec §7 的"直接 UsageService.Create")
- 回归不破坏现有 /v1/images/*:Task 3 step5 + Task 2/1 回归 ✓

## 备注 / 风险
- **同步超时**:`/generate` MVP 限 `n≤4` 且尺寸上限,SSE 视需要;4K+批量留意网关超时(spec §13)。
- **并发器全局语义**:MVP 全局,按用户限流 Phase 2。
- **OAuth 图片桥**先验 APIKey 路径(spec §13)。
- **RecordUsage 修正**:计划以 `RecordUsage` 为准(非 spec §7 写的 `UsageService.Create` 直调);实现时若 `OpenAIRecordUsageInput` 需要 `ChannelUsageFields` 等,按现有 handler:323 的构造照搬。
