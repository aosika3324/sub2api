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
describe('imageStudio API', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let adapter: any
  let fetchAssetBlob: (url: string) => Promise<Blob>
  let toAssetBrowserURL: (url: string) => string
  let generate: typeof import('@/api/imageStudio').generate
  let getGeneration: typeof import('@/api/imageStudio').getGeneration
  let clearHistory: typeof import('@/api/imageStudio').clearHistory

  beforeEach(async () => {
    localStorage.clear()
    vi.resetModules()
    const client = await import('@/api/client')
    const api = await import('@/api/imageStudio')
    fetchAssetBlob = api.fetchAssetBlob
    toAssetBrowserURL = api.toAssetBrowserURL
    generate = api.generate
    getGeneration = api.getGeneration
    clearHistory = api.clearHistory
    adapter = vi.fn().mockResolvedValue({
      status: 200,
      data: new Blob(['img'], { type: 'image/png' }),
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

  it('sniffs PNG blobs when the asset response has no image content-type', async () => {
    adapter.mockResolvedValueOnce({
      status: 200,
      data: new Blob([
        new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
      ]),
      headers: {},
      config: {},
      statusText: 'OK',
    })

    const blob = await fetchAssetBlob('/user/image-studio/assets/9/1')

    expect(blob.type).toBe('image/png')
  })

  it('normalizes relative asset paths into browser URLs for direct image fallback', () => {
    expect(toAssetBrowserURL('/user/image-studio/assets/9/1')).toBe(
      '/api/v1/user/image-studio/assets/9/1'
    )
    expect(toAssetBrowserURL('/api/v1/user/image-studio/assets/9/1')).toBe(
      '/api/v1/user/image-studio/assets/9/1'
    )
  })

  it('sends multiple reference images as multipart image fields with no timeout', async () => {
    adapter.mockResolvedValueOnce({
      status: 200,
      data: { code: 0, data: { generation_id: 1, conversation_id: 1, images: [], status: 'pending', cost: 0, balance: 0 } },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    const files = [
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.png', { type: 'image/png' }),
    ]

    await generate({
      group_id: 1,
      mode: 'compose',
      prompt: 'edit',
      model: 'gpt-image-2',
      size: '1024x1024',
      quality: 'auto',
      n: 1,
      referenceImages: files,
    })

    const config = adapter.mock.calls[0][0]
    expect(config.timeout).toBe(0)
    expect(config.data).toBeInstanceOf(FormData)
    expect(config.data.get('mode')).toBe('compose')
    expect(config.data.getAll('image')).toEqual(files)
  })

  it('infers compose mode for legacy multi-reference multipart requests', async () => {
    adapter.mockResolvedValueOnce({
      status: 200,
      data: { code: 0, data: { generation_id: 1, conversation_id: 1, images: [], status: 'pending', cost: 0, balance: 0 } },
      headers: {},
      config: {},
      statusText: 'OK',
    })
    const files = [
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.png', { type: 'image/png' }),
    ]

    await generate({
      group_id: 1,
      prompt: 'compose',
      model: 'gpt-image-2',
      size: '1024x1024',
      quality: 'auto',
      n: 1,
      referenceImages: files,
    })

    const config = adapter.mock.calls[0][0]
    expect(config.data.get('mode')).toBe('compose')
    expect(config.data.getAll('image')).toEqual(files)
  })

  it('exposes generation status and clear history endpoints', async () => {
    adapter.mockResolvedValue({
      status: 200,
      data: { code: 0, data: {} },
      headers: {},
      config: {},
      statusText: 'OK',
    })

    await getGeneration(42)
    expect(fullURL()).toBe('/api/v1/user/image-studio/generations/42')

    adapter.mockClear()
    await clearHistory()
    expect(fullURL()).toBe('/api/v1/user/image-studio/history')
  })
})
