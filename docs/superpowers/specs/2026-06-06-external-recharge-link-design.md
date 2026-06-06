# 外部充值链接(External Recharge Link)— 设计文档

- 日期:2026-06-06
- 分支:`feature/external-recharge-link`(从 `dev` 切出)
- 状态:待用户复核

## 1. 概述

为用户增加"充值"入口:点击后在**新标签页**打开一个**后台可配置的外部充值地址**。入口出现在三处,且全部共用同一套跳转逻辑:

1. 顶栏余额数字(变为可点击)
2. 顶栏独立"充值"按钮(余额旁)
3. 用户左侧导航栏**现有的"充值/订阅"项**(增强其行为,不新增菜单项)

外部地址在管理后台设置;**配置了外链就优先走外链,没配置则回退到站点内置支付页 `/payment`**;两者都不可用时入口隐藏。

> **现状说明**:`/purchase` 路由指向**内置 `PaymentView.vue`**(站内支付页,受 `payment_enabled` 控制),侧边栏现有项 `nav.buySubscription` 标签即"充值/订阅"。本功能**复用并增强该项**而非新增,避免两个含义重叠的充值入口。后端 `purchase_subscription_*` 设置在当前前端基本未被消费,仅作为"新增设置项"的**代码模板**参考。

实现策略:**镜像代码库已有的 `purchase_subscription_enabled` / `purchase_subscription_url` 模式**,绝大多数改动为"加法"(新增设置键、在已有列表里插入条目、新增一个前端 composable);唯一对既有行为的改动是**增强侧边栏"充值/订阅"项**(改 `featureFlag` + 接入 `openRecharge`),范围很小。整体最大限度降低与上游同步的冲突面。

## 2. 目标 / 非目标

**目标**
- 后台可配置的外部充值 URL + 启用开关。
- 三个前端入口共用单一跳转逻辑;新标签打开外链。
- 外链优先、内置 `/payment` 兜底的优先级规则集中在一处实现。

**非目标(YAGNI)**
- URL **不**携带用户信息模板变量(`{userId}`/`{email}`/`{balance}`)。纯静态地址。如将来需要,可在 `useRecharge` 内集中扩展,不影响其它部分。
- 不做站内 iframe 嵌入(那是 `purchase_subscription` 的方式;本功能明确用新标签跳转,因此**无需改 CSP frame-src**)。
- 不改动内置支付系统本身。

## 3. 后端设计(设置项)

镜像 `purchase_subscription_*`,涉及以下既有文件,均为在现有结构/列表中"插入":

| 文件 | 改动 |
|---|---|
| `internal/service/domain_constants.go` | 新增 `SettingKeyExternalRechargeEnabled = "external_recharge_enabled"`、`SettingKeyExternalRechargeURL = "external_recharge_url"` |
| `internal/service/settings_view.go` | 在 admin `Settings`(~138)与 `PublicSettings`(~261)两个结构体各加 `ExternalRechargeEnabled bool` / `ExternalRechargeURL string` |
| `internal/service/setting_service.go` | ① `GetPublicSettings` keys 列表(~721)加两键;② 解析进 `PublicSettings`(~846);③ 解析进 admin settings(~2889);④ `PublicSettingsForInjection` JSON 结构(~1160)+ 映射(~1226)加字段(JSON tag:`external_recharge_enabled` / `external_recharge_url`);⑤ `UpdateSettings` 持久化(~1819);⑥ 默认值(~2693)`"false"` / `""` |

**校验**:`UpdateSettings` 中对 `external_recharge_url` 先 `strings.TrimSpace`;若启用且非空,校验必须以 `http://` 或 `https://` 开头,否则返回设置更新错误(沿用现有错误返回风格)。空字符串允许(表示未配置)。

**不改 CSP**:因为是新标签跳转而非 iframe,无需 `setting_service.go:~1481` 的 `addOrigin`。

## 4. 前端设计

### 4.1 共享逻辑 `composables/useRecharge.ts`(新文件)

唯一的"优先级 + 动作"实现处:

```ts
// 伪代码契约
useRecharge() => {
  isExternal:      ComputedRef<boolean>   // external_recharge_enabled === true && url 去空格后非空
  rechargeVisible: ComputedRef<boolean>   // isExternal || payment_enabled
  openRecharge():  void                   // isExternal ? window.open(url,'_blank','noopener') : router.push('/payment')
}
```

- 输入:`appStore.cachedPublicSettings`(`external_recharge_enabled`、`external_recharge_url`、`payment_enabled`)。
- 三个入口全部调用 `openRecharge()`,用 `rechargeVisible` 控制显隐。

### 4.2 类型与默认值

| 文件 | 改动 |
|---|---|
| `src/types/index.ts` | `PublicSettings` 接口加 `external_recharge_enabled?: boolean`、`external_recharge_url?: string` |
| `src/stores/app.ts` | 默认 settings 对象(~338)加 `external_recharge_enabled: false`、`external_recharge_url: ''` |

### 4.3 入口

1. **顶栏余额可点击** — `AppHeader.vue` 桌面端(~66)与移动端下拉(~114)的余额包成可点元素:`@click="openRecharge"`、`v-if`/包裹按 `rechargeVisible`、加鼠标手型 + `title` 提示。余额数字本身展示不变。
2. **顶栏充值按钮** — `AppHeader.vue` 余额旁新增小按钮,`v-if="rechargeVisible"`、`@click="openRecharge"`。
3. **左侧导航项(增强现有"充值/订阅"项,不新增)** — `AppSidebar.vue`:
   - `buildSelfNavItems()`(~673)现有项 `{ path: '/purchase', label: t('nav.buySubscription'), … featureFlag: flagPayment }`:
     - 将 `featureFlag` 由 `flagPayment` 改为 `() => rechargeVisible.value`(配了外链但关闭内置支付时也应显示)。
     - 给该项打标记(如 `rechargeAware: true`),由统一点击处理走 `openRecharge`:外链已配 → 新标签打开外链;否则按原行为进入内置 `/payment`。
   - 模板:对带标记的项,外链情形渲染为可点击元素并 `@click.prevent="openRecharge"`;非外链情形保持原 `<router-link>` 跳 `/payment` 的高亮语义。`NavItem` 类型加可选 `rechargeAware?: boolean`。"`<a>` vs `<router-link>` 条件渲染"细节在计划阶段定。
   - 标签沿用现有 `nav.buySubscription`("充值/订阅"),侧边栏不新增文案。

### 4.4 管理后台表单

在管理设置页中"支付/订阅"相关区块,镜像 `purchase_subscription` 那组表单(开关 + URL 输入)。**精确表单组件在写实现计划时定位**(`SettingsView.vue` 达 9783 行且表单已拆分,`purchase_subscription` 实际不在该文件;计划阶段用 grep 定位其真实编辑位置后在同处插入)。

### 4.5 i18n

`src/i18n/locales/en.ts` 与 `zh.ts` 各新增:
- 顶栏充值按钮文案(如 "充值" / "Recharge")与余额可点的 `title` 提示
- 管理页字段标签(启用外部充值 / 外部充值地址 + 帮助说明)
- 侧边栏沿用现有 `nav.buySubscription`,**不**新增

## 5. 边界与错误处理

- URL 为空或非 `http(s)`:启用校验拦截写入;前端 `externalConfigured=false` → 回退内置 `/payment`。
- `payment_enabled=false` 且未配外链:`rechargeVisible=false`,三入口全部隐藏。
- `simple 模式`:侧边栏充值项 `hideInSimpleMode:true`(与购买订阅一致);顶栏入口仍按 `rechargeVisible`。
- `backend 模式`:沿用现有路由守卫,不特殊处理(外链是新标签,不经路由)。
- 新标签统一带 `noopener`(安全,防 `window.opener` 反向控制)。

## 6. 测试

**后端**(仿 `setting_service_public_test.go`)
- 新设置默认值为 `false`/`""`。
- `GetPublicSettings` 正确暴露两字段。
- `UpdateSettings` 持久化;URL 非法(启用且非 http(s))被拒;空 URL 允许。

**前端**(vitest)
- `useRecharge` 单测:三分支 —— 外链已配(开新标签)、仅内置(push `/payment`)、都没有(`rechargeVisible=false`)。
- `AppHeader` 渲染:配/不配外链时充值按钮与余额可点性的显隐、点击触发 `openRecharge`。
- `AppSidebar`:外链已配时"充值/订阅"项点击触发 `openRecharge`(外链分支)、未配时仍指向 `/payment`;`rechargeVisible` 控制显隐(含"内置支付关闭但外链开启"的情形)。

## 7. 改动文件清单

**后端(3)**:`domain_constants.go`、`settings_view.go`、`setting_service.go`(+ 对应测试)
**前端(7)**:`composables/useRecharge.ts`(新)、`AppHeader.vue`(余额可点 + 充值按钮)、`AppSidebar.vue`(**增强现有"充值/订阅"项**)、`types/index.ts`、`stores/app.ts`、管理设置表单组件(在管理设置页"支付/站点"分区新增开关+URL,计划阶段定位具体组件)、`i18n/locales/{en,zh}.ts`(+ 对应测试 spec)

除"增强侧边栏现有项"一处小改动外,其余皆为新增文件或在既有列表/结构里插入条目 —— 符合 fork "加法优先、少碰上游既有逻辑" 的同步友好原则。

## 8. 实现顺序(供后续计划参考)

1. 后端设置键 + 结构 + 默认 + 暴露 + 校验 + 后端测试。
2. 前端类型 + store 默认 + `useRecharge` composable + 单测。
3. 三个入口接线(Header×2 + Sidebar)+ i18n + 渲染测试。
4. 管理后台表单。
5. `make test-unit` + 前端关键测试 全绿。
