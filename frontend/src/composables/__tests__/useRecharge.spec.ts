/**
 * useRecharge 组合式函数单元测试 —— 验证"外链优先 / 内置兜底 / 都无则隐藏"三分支。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { PublicSettings } from '@/types'

const { pushSpy } = vi.hoisted(() => ({ pushSpy: vi.fn() }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushSpy }) }))

import { useRecharge } from '../useRecharge'
import { useAppStore } from '@/stores/app'

function setSettings(partial: Partial<PublicSettings>) {
  const appStore = useAppStore()
  appStore.cachedPublicSettings = {
    ...(appStore.cachedPublicSettings ?? {}),
    ...partial,
  } as PublicSettings
}

describe('useRecharge', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    pushSpy.mockReset()
    window.open = vi.fn() as unknown as typeof window.open
  })

  it('外链已配置:isExternal/visible=true,openRecharge 新标签打开外链(URL 去空格)', () => {
    setSettings({
      external_recharge_enabled: true,
      external_recharge_url: '  https://pay.example.com/topup  ',
      payment_enabled: false,
    })
    const { isExternal, rechargeVisible, openRecharge } = useRecharge()
    expect(isExternal.value).toBe(true)
    expect(rechargeVisible.value).toBe(true)
    openRecharge()
    expect(window.open).toHaveBeenCalledWith(
      'https://pay.example.com/topup',
      '_blank',
      'noopener,noreferrer',
    )
    expect(pushSpy).not.toHaveBeenCalled()
  })

  it('仅内置支付:isExternal=false,openRecharge 跳转 /purchase', () => {
    setSettings({
      external_recharge_enabled: false,
      external_recharge_url: '',
      payment_enabled: true,
    })
    const { isExternal, rechargeVisible, openRecharge } = useRecharge()
    expect(isExternal.value).toBe(false)
    expect(rechargeVisible.value).toBe(true)
    openRecharge()
    expect(pushSpy).toHaveBeenCalledWith('/purchase')
    expect(window.open).not.toHaveBeenCalled()
  })

  it('都未启用:rechargeVisible=false,openRecharge 不动作', () => {
    setSettings({
      external_recharge_enabled: false,
      external_recharge_url: '',
      payment_enabled: false,
    })
    const { rechargeVisible, openRecharge } = useRecharge()
    expect(rechargeVisible.value).toBe(false)
    openRecharge()
    expect(pushSpy).not.toHaveBeenCalled()
    expect(window.open).not.toHaveBeenCalled()
  })

  it('开关开但 URL 为空白:视为非外链,payment 开时回退 /purchase', () => {
    setSettings({
      external_recharge_enabled: true,
      external_recharge_url: '   ',
      payment_enabled: true,
    })
    const { isExternal, openRecharge } = useRecharge()
    expect(isExternal.value).toBe(false)
    openRecharge()
    expect(pushSpy).toHaveBeenCalledWith('/purchase')
  })
})
