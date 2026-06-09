import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/api/imageStudio', () => ({
  fetchAssetBlob: vi.fn(),
  toAssetBrowserURL: (url: string) =>
    url.startsWith('/user/image-studio/') ? `/api/v1${url}` : url,
}))

import AuthedImage from '../AuthedImage.vue'
import { fetchAssetBlob } from '@/api/imageStudio'

const IconStub = defineComponent({ name: 'Icon', template: '<span />' })
const fetchAssetBlobMock = vi.mocked(fetchAssetBlob)

describe('AuthedImage', () => {
  let observerCallback: IntersectionObserverCallback | null = null
  let observedElement: Element | null = null
  let disconnectMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    observerCallback = null
    observedElement = null
    disconnectMock = vi.fn()
    fetchAssetBlobMock.mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
    URL.createObjectURL = vi.fn(() => 'blob:asset') as unknown as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as unknown as typeof URL.revokeObjectURL

    class TestIntersectionObserver {
      constructor(callback: IntersectionObserverCallback) {
        observerCallback = callback
      }

      observe(element: Element) {
        observedElement = element
      }

      disconnect = disconnectMock
      unobserve = vi.fn()
      takeRecords = vi.fn(() => [])
      root = null
      rootMargin = '0px'
      thresholds = []
    }

    Object.defineProperty(window, 'IntersectionObserver', {
      configurable: true,
      value: TestIntersectionObserver,
    })
    Object.defineProperty(globalThis, 'IntersectionObserver', {
      configurable: true,
      value: TestIntersectionObserver,
    })
  })

  it('defers fetching the protected asset until the image enters the viewport', async () => {
    const wrapper = mount(AuthedImage, {
      props: {
        url: '/api/v1/user/image-studio/assets/9/0',
        alt: 'generated image',
      },
      global: { stubs: { Icon: IconStub } },
    })
    await flushPromises()

    expect(observedElement).not.toBeNull()
    expect(fetchAssetBlobMock).not.toHaveBeenCalled()
    expect(wrapper.find('img').exists()).toBe(false)

    observerCallback?.([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    await flushPromises()

    expect(fetchAssetBlobMock).toHaveBeenCalledWith('/api/v1/user/image-studio/assets/9/0')
    expect(disconnectMock).toHaveBeenCalled()

    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('blob:asset')
    expect(img.attributes('loading')).toBe('lazy')

    await img.trigger('click')
    expect(wrapper.emitted('open')?.[0]).toEqual(['blob:asset'])
  })

  it('revokes the created object URL when unmounted', async () => {
    const wrapper = mount(AuthedImage, {
      props: { url: '/api/v1/user/image-studio/assets/9/0' },
      global: { stubs: { Icon: IconStub } },
    })

    observerCallback?.([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    await flushPromises()
    wrapper.unmount()

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:asset')
  })

  it('falls back to the browser asset URL when blob fetching fails', async () => {
    fetchAssetBlobMock.mockRejectedValueOnce(new Error('bad blob'))

    const wrapper = mount(AuthedImage, {
      props: { url: '/user/image-studio/assets/9/0' },
      global: { stubs: { Icon: IconStub } },
    })

    observerCallback?.([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    await flushPromises()

    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('/api/v1/user/image-studio/assets/9/0')
  })
})
