/**
 * useAuthedImage 组合式函数单元测试
 * 重点验证 object URL 取消语义（竞速时撤销被取代的 URL，避免泄漏）
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { useAuthedImage } from '../useAuthedImage'

// ==================== Mock @/api/imageStudio ====================

const mockFetchAssetBlob = vi.fn()

vi.mock('@/api/imageStudio', () => ({
  fetchAssetBlob: (...args: any[]) => mockFetchAssetBlob(...args),
  toAssetBrowserURL: (url: string) =>
    url.startsWith('/user/image-studio/') ? `/api/v1${url}` : url,
}))

// ==================== Deferred helper ====================

interface Deferred<T> {
  promise: Promise<T>
  resolve: (v: T) => void
  reject: (e: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

// Mount the composable inside a real component so onUnmounted works.
function mountComposable(
  url: ReturnType<typeof ref<string | undefined | null>>,
  enabled?: ReturnType<typeof ref<boolean>>
) {
  let api!: ReturnType<typeof useAuthedImage>
  const wrapper = mount(
    defineComponent({
      setup() {
        api = enabled ? useAuthedImage(url, enabled) : useAuthedImage(url)
        return () => h('div')
      },
    })
  )
  return { wrapper, api: api! }
}

describe('useAuthedImage', () => {
  // jsdom does not implement the URL.createObjectURL / revokeObjectURL API,
  // so we assign mocks directly (vi.spyOn requires a pre-existing property).
  const createObjectURL = vi.fn()
  const revokeSpy = vi.fn()
  let urlCounter = 0
  const originalCreate = URL.createObjectURL
  const originalRevoke = URL.revokeObjectURL

  beforeEach(() => {
    vi.clearAllMocks()
    urlCounter = 0
    createObjectURL.mockImplementation(() => `blob:mock-${++urlCounter}`)
    revokeSpy.mockImplementation(() => {})
    URL.createObjectURL = createObjectURL as unknown as typeof URL.createObjectURL
    URL.revokeObjectURL = revokeSpy as unknown as typeof URL.revokeObjectURL
  })

  afterEach(() => {
    URL.createObjectURL = originalCreate
    URL.revokeObjectURL = originalRevoke
  })

  it('成功加载后 src 为创建的 object URL', async () => {
    const blob = new Blob(['x'])
    mockFetchAssetBlob.mockResolvedValue(blob)

    const url = ref<string | undefined | null>('/assets/1/0')
    const { api } = mountComposable(url)

    await nextTick()
    await Promise.resolve() // let the awaited fetch settle

    expect(api.src.value).toBe('blob:mock-1')
    expect(api.loading.value).toBe(false)
    expect(api.error.value).toBeNull()
  })

  it('does not fetch until enabled becomes true', async () => {
    const blob = new Blob(['x'])
    mockFetchAssetBlob.mockResolvedValue(blob)

    const url = ref<string | undefined | null>('/assets/1/0')
    const enabled = ref(false)
    const { api } = mountComposable(url, enabled)

    await nextTick()
    expect(mockFetchAssetBlob).not.toHaveBeenCalled()
    expect(api.src.value).toBeUndefined()

    enabled.value = true
    await nextTick()
    await Promise.resolve()

    expect(mockFetchAssetBlob).toHaveBeenCalledWith('/assets/1/0')
    expect(api.src.value).toBe('blob:mock-1')
  })

  it('URL 快速切换: 被取代的在途请求撤销自己创建的 object URL（不泄漏）', async () => {
    const d1 = deferred<Blob>()
    const d2 = deferred<Blob>()
    mockFetchAssetBlob
      .mockReturnValueOnce(d1.promise)
      .mockReturnValueOnce(d2.promise)

    const url = ref<string | undefined | null>('/assets/1/0')
    const { api } = mountComposable(url)
    await nextTick()

    // 切换到新 URL，触发第二次 load（取代第一次）
    url.value = '/assets/2/0'
    await nextTick()

    // 第二次先返回 -> 创建 blob:mock-1，成为 currentObjectUrl
    d2.resolve(new Blob(['second']))
    await Promise.resolve()
    await Promise.resolve()
    expect(api.src.value).toBe('blob:mock-1')

    // 现在第一次（已被取代）才返回 -> 必须撤销它自己创建的 URL，且不覆盖 src
    d1.resolve(new Blob(['first']))
    await Promise.resolve()
    await Promise.resolve()

    // 它创建的是 blob:mock-2，应被立即撤销
    expect(revokeSpy).toHaveBeenCalledWith('blob:mock-2')
    // src 仍是胜出请求的 URL，未被陈旧请求覆盖
    expect(api.src.value).toBe('blob:mock-1')
  })

  it('被取代的请求不写入 loading/error 状态', async () => {
    const d1 = deferred<Blob>()
    const d2 = deferred<Blob>()
    mockFetchAssetBlob
      .mockReturnValueOnce(d1.promise)
      .mockReturnValueOnce(d2.promise)

    const url = ref<string | undefined | null>('/assets/1/0')
    const { api } = mountComposable(url)
    await nextTick()

    url.value = '/assets/2/0'
    await nextTick()

    // 第二个（胜出）完成
    d2.resolve(new Blob(['ok']))
    await Promise.resolve()
    await Promise.resolve()
    expect(api.loading.value).toBe(false)

    // 第一个（陈旧）以错误失败，不得污染 error
    d1.reject(new Error('stale failure'))
    await Promise.resolve()
    await Promise.resolve()
    expect(api.error.value).toBeNull()
    expect(api.loading.value).toBe(false)
  })

  it('组件卸载时撤销当前 object URL', async () => {
    mockFetchAssetBlob.mockResolvedValue(new Blob(['x']))

    const url = ref<string | undefined | null>('/assets/1/0')
    const { wrapper, api } = mountComposable(url)
    await nextTick()
    await Promise.resolve()
    expect(api.src.value).toBe('blob:mock-1')

    wrapper.unmount()
    expect(revokeSpy).toHaveBeenCalledWith('blob:mock-1')
  })
})
