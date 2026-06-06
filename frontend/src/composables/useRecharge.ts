import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

/** 站内内置充值页(PaymentView)的路由,无外链时回退到此。 */
export const INTERNAL_RECHARGE_PATH = '/purchase'

/**
 * 充值入口统一逻辑 —— 顶栏余额、顶栏充值按钮、侧边栏"充值/订阅"项共用。
 *
 * 优先级(由后台设置驱动):
 *   1. 配置了外部充值地址(external_recharge_enabled 且 url 非空)→ 新标签打开外链;
 *   2. 否则站内 payment 已启用 → 跳转内置 /purchase;
 *   3. 两者都不可用 → rechargeVisible=false,入口应隐藏。
 *
 * 这是该优先级规则的唯一实现处;新增入口只需调用 openRecharge / 用 rechargeVisible 控显隐。
 */
export function useRecharge() {
  const appStore = useAppStore()
  const router = useRouter()

  const externalUrl = computed(() =>
    (appStore.cachedPublicSettings?.external_recharge_url ?? '').trim(),
  )

  /** 外链已正确配置(开关开 + 地址非空)。 */
  const isExternal = computed(
    () =>
      appStore.cachedPublicSettings?.external_recharge_enabled === true &&
      externalUrl.value !== '',
  )

  const paymentEnabled = computed(
    () => appStore.cachedPublicSettings?.payment_enabled === true,
  )

  /** 是否展示充值入口:外链已配 或 内置支付已启用。 */
  const rechargeVisible = computed(() => isExternal.value || paymentEnabled.value)

  function openRecharge(): void {
    if (!rechargeVisible.value) return
    if (isExternal.value) {
      window.open(externalUrl.value, '_blank', 'noopener,noreferrer')
      return
    }
    void router.push(INTERNAL_RECHARGE_PATH)
  }

  return { isExternal, rechargeVisible, externalUrl, openRecharge }
}
