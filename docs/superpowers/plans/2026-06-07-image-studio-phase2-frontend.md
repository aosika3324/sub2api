# Image Studio — Phase 2 Frontend (对话式工作台) 实现计划

> 执行:superpowers:subagent-driven-development。Vue 3 + TS + Pinia + Vite,前端在 `/home/miku/workspack/sub2api/frontend`,分支 `feature/image-studio`。后端 Phase 1 已就绪(`/api/v1/user/image-studio/*`,**同步** generate)。Spec §4:`docs/superpowers/specs/2026-06-06-image-studio-design.md`。

**Goal:** 登录用户在站内"作图工作台"页(对话式:多线程会话 + turn 时间线 + composer + 画廊)生成 OpenAI 图片,看到费用/余额,浏览历史。

**关键约束/坑:**
- generate 是**同步**接口(await POST 直接拿结果,无需轮询/队列)。
- 返回的 `images` 是**需 JWT Bearer 的 asset URL** → 必须用 `apiClient.get(url,{responseType:'blob'})` 拉取 → `URL.createObjectURL` 显示,组件卸载时 `revokeObjectURL`。**不能**直接 `<img :src=assetURL>`。
- 分组选择器用现成 `getAvailableGroups()`(`api/groups.ts`,返回 `Group[]`);余额读 `authStore.user.balance`,generate 返回新 balance 后更新它。
- 后端按分组 `allow_image_generation` 在请求时校验;前端 nav 项对认证用户显示(`hideInSimpleMode`),页面对"无可用作图分组/403"友好提示。

**后端端点(全 JWT,前缀 `/api/v1`):** `POST /user/image-studio/generate` {conversation_id?,group_id,prompt,model,size,quality,n}→{generation_id,conversation_id,images[url],cost,balance};`GET/POST /user/image-studio/conversations`;`PATCH/DELETE /user/image-studio/conversations/:id`;`GET /user/image-studio/conversations/:id/generations`;`GET /user/image-studio/generations`;`DELETE /user/image-studio/generations/:id`;`GET /user/image-studio/assets/:genID/:idx`(blob)。

---

## Task F1: 数据层 —— types + api 模块 + store + 鉴权图片 blob 助手
**Files:** Create `src/api/imageStudio.ts`, `src/stores/imageStudio.ts`, `src/composables/useAuthedImage.ts`; Modify `src/types/index.ts`(加 ImageStudio* 类型);Test `src/stores/__tests__/imageStudio.spec.ts`
- 步骤(TDD):
  1. 类型:`ImageStudioConversation{id,title,created_at,updated_at}`、`ImageStudioGeneration{id,conversation_id,group_id,prompt,model,size,quality,n,image_count,status,cost,created_at,images?:string[]}`、`GenerateImageRequest`、`GenerateImageResponse{generation_id,conversation_id,images:string[],cost,balance}`。
  2. `api/imageStudio.ts`:用 `apiClient` 封装上面所有端点(分页用现有 `PaginatedResponse`)。assets 拉取:`fetchAsset(url):Promise<Blob>` = `apiClient.get(url,{responseType:'blob'}).then(r=>r.data)`。
  3. `composables/useAuthedImage.ts`:给定 asset URL,内部 `fetchAsset`→`createObjectURL`,返回 ref(objectURL)+ onUnmounted 时 revoke。(供画廊用。)
  4. `stores/imageStudio.ts`(Pinia setup store):state(conversations、activeConversationId、generations[当前会话/全局画廊]、loading、generating、error);actions(loadConversations、createConversation、renameConversation、deleteConversation、selectConversation、loadGenerations(conversationId?)、generate(input)[await,成功后把新 generation 插入时间线、更新 authStore.user.balance=resp.balance]、deleteGeneration)。
  5. store 单测(mock api):generate 成功插入 + 更新余额;失败置 error 不插入;loadConversations 填充。
- 验证:`node_modules/.bin/vue-tsc --noEmit`(0 err)、`node_modules/.bin/vitest run src/stores/__tests__/imageStudio.spec.ts`、eslint 改动文件。提交(显式 add,**非** -A;deploy/ 不动)`feat(image-studio-fe): data layer (api + store + authed-image)`。

## Task F2: 工作台视图 ImageStudioView.vue(对话式)
**Files:** Create `src/views/user/ImageStudioView.vue`(可拆子组件到 `src/components/user/imageStudio/`:ConversationList.vue、TurnTimeline.vue、ImageComposer.vue、GenerationCard.vue);Test 关键子组件 vitest
- 布局:左=会话线程列表(新建/选中/重命名/删除);中=选中会话的 turn 时间线(每 turn=提示词+参数 → 结果图网格,用 useAuthedImage 显示,生成中显示 spinner;失败显示错误+重试);底=composer(分组选择器[getAvailableGroups]、提示词 textarea、模型下拉[gpt-image-1/dall-e-3]、尺寸、质量、数量 n、生成按钮);顶/角=实时余额 + 本次预估/实际费用。
- 行为:选会话→loadGenerations;generate→store.generate(乐观插入一个 generating turn→await→填图或错误);无可用作图分组时禁用 composer + 友好提示;403 错误提示"该分组未开启作图"。
- 风格:沿用现有 user 视图的设计语言(参考 `views/user/PaymentView.vue`/`KeysView.vue` 的卡片/间距/暗色);避免通用 AI 感,做出干净专业的工作台观感。
- 验证:vue-tsc 0 err、eslint、关键子组件 vitest(composer 提交 payload、GenerationCard 渲染态)。提交 `feat(image-studio-fe): conversation studio view`。

## Task F3: 接线 —— 侧边栏 + 路由 + i18n
**Files:** Modify `src/components/layout/AppSidebar.vue`(buildSelfNavItems 加 `{path:'/image-studio',label:t('nav.imageStudio'),icon:<某图标>,hideInSimpleMode:true}`,放在 /usage 附近);`src/router/index.ts`(`{path:'/image-studio',name:'ImageStudio',component:()=>import('@/views/user/ImageStudioView.vue'),meta:{requiresAuth:true,title,titleKey:'nav.imageStudio'}}`);`src/i18n/locales/{en,zh}.ts`(nav.imageStudio + 页面文案)。
- 验证:vue-tsc、eslint、`vitest run src/router`(若有守卫测试)。提交 `feat(image-studio-fe): sidebar nav + route + i18n`。

## Task F4: 验证 + 手测
- `node_modules/.bin/vue-tsc --noEmit`(0)、eslint 改动文件、相关 vitest 全绿。
- 手测(后端 feature/image-studio :8080 + `pnpm dev` :3000;某分组开 allow_image_generation + 挂 OpenAI 图片账号):侧边栏见"作图工作台"→选分组+提示词→生成→出图、余额下降、历史/会话可见、删除可用、刷新后画廊图片经 blob 正常显示。

## 自检
- 鉴权图片用 blob(F1 useAuthedImage)✓ 同步 generate 无轮询 ✓ 分组选择复用 getAvailableGroups ✓ 余额更新 ✓ nav+route+i18n ✓ 后端端点全覆盖 ✓
