import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// getLocale is called by the client request interceptor; stub it like client.spec.ts.
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

// Regression test for the asset-URL double-prefix bug (C1): the backend builds
// asset URLs already prefixed with "/api/v1", and apiClient's baseURL is also
// "/api/v1", so passing the absolute path straight to apiClient produced
// "/api/v1/api/v1/..." → every studio image 404'd. The previous specs missed
// this because they mocked fetchAssetBlob / useAuthedImage; this test drives the
// real axios adapter and asserts the final request carries exactly one prefix.
describe('fetchAssetBlob URL prefixing (C1)', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let adapter: any
  let fetchAssetBlob: (url: string) => Promise<Blob>

  beforeEach(async () => {
    localStorage.clear()
    vi.resetModules()
    const client = await import('@/api/client')
    const api = await import('@/api/imageStudio')
    fetchAssetBlob = api.fetchAssetBlob
    adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: new Blob(['img']),
      headers: {},
      config: {},
      statusText: 'OK',
    })
    client.apiClient.defaults.adapter = adapter
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function fullURL(): string {
    const config = adapter.mock.calls[0][0]
    return `${config.baseURL ?? ''}${config.url}`
  }

  it('strips the duplicate /api/v1 from backend-built absolute asset URLs', async () => {
    await fetchAssetBlob('/api/v1/user/image-studio/assets/42/0')

    expect(adapter).toHaveBeenCalledTimes(1)
    const full = fullURL()
    expect(full).toBe('/api/v1/user/image-studio/assets/42/0')
    expect(full).not.toContain('/api/v1/api/v1')
  })

  it('leaves already-relative asset URLs intact', async () => {
    await fetchAssetBlob('/user/image-studio/assets/9/1')

    expect(fullURL()).toBe('/api/v1/user/image-studio/assets/9/1')
  })
})
