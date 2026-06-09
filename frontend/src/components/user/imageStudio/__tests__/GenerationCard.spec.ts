import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, defineComponent } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

// Stub useAuthedImage so AuthedImage renders a plain <img> without network I/O.
vi.mock('@/composables/useAuthedImage', () => ({
  useAuthedImage: (url: unknown) => {
    const resolve = (u: unknown) =>
      typeof u === 'object' && u !== null && 'value' in (u as Record<string, unknown>)
        ? (u as { value: string }).value
        : (u as string)
    return {
      src: ref(`blob:${resolve(url)}`),
      loading: ref(false),
      error: ref(null),
    }
  },
}))

import GenerationCard from '../GenerationCard.vue'
import type { ImageStudioGeneration } from '@/types'

const IconStub = defineComponent({ name: 'Icon', template: '<span />' })
const AuthedImageStub = defineComponent({
  name: 'AuthedImage',
  props: {
    url: { type: String, required: true },
    alt: { type: String, default: '' },
  },
  emits: ['open'],
  template: '<img :src="\'blob:\' + url" :alt="alt" @click="$emit(\'open\', \'blob:\' + url)" />',
})

function makeGeneration(overrides: Partial<ImageStudioGeneration> = {}): ImageStudioGeneration {
  return {
    id: 1,
    conversation_id: 1,
    group_id: 5,
    prompt: 'a serene lake at dawn',
    model: 'gpt-image-2',
    size: '1K',
    quality: 'high',
    n: 2,
    image_count: 2,
    status: 'succeeded',
    cost: 0.08,
    created_at: '2026-06-07T00:00:00Z',
    images: ['/assets/1/0', '/assets/1/1'],
    ...overrides,
  }
}

function mountCard(generation: ImageStudioGeneration) {
  return mount(GenerationCard, {
    props: { generation },
    global: { stubs: { Icon: IconStub, AuthedImage: AuthedImageStub } },
  })
}

describe('GenerationCard', () => {
  it('renders a spinner and generating text for pending status', () => {
    const wrapper = mountCard(makeGeneration({ status: 'pending', images: [] }))
    expect(wrapper.find('.animate-spin').exists()).toBe(true)
    expect(wrapper.text()).toContain('imageStudio.generating')
    expect(wrapper.text()).toContain('imageStudio.continueWaitingHint')
    expect(wrapper.text()).toContain('imageStudio.refreshStatus')
    expect(wrapper.findAll('img').length).toBe(0)
  })

  it('emits refresh from a pending generation', async () => {
    const wrapper = mountCard(makeGeneration({ status: 'pending', images: [] }))
    const refreshBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('imageStudio.refreshStatus'))
    expect(refreshBtn).toBeTruthy()

    await refreshBtn!.trigger('click')
    const emitted = wrapper.emitted('refresh')
    expect(emitted).toBeTruthy()
    expect((emitted![0][0] as ImageStudioGeneration).id).toBe(1)
  })

  it('renders an image grid for succeeded status', () => {
    const wrapper = mountCard(makeGeneration())
    const imgs = wrapper.findAll('img')
    expect(imgs.length).toBe(2)
    // useAuthedImage stub turns each URL into blob:<url>
    expect(imgs[0].attributes('src')).toBe('blob:/assets/1/0')
    expect(imgs[1].attributes('src')).toBe('blob:/assets/1/1')
    // Cost chip is shown
    expect(wrapper.text()).toContain('$0.0800')
    expect(wrapper.text()).toContain('imageStudio.quickEdit')
  })

  it('emits edit with the selected image url from the quick edit button', async () => {
    const wrapper = mountCard(makeGeneration())
    const editBtn = wrapper
      .findAll('button')
      .find((button) => button.text().includes('imageStudio.quickEdit'))
    expect(editBtn).toBeTruthy()

    await editBtn!.trigger('click')

    const emitted = wrapper.emitted('edit')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({
      generation: expect.objectContaining({ id: 1 }),
      url: '/assets/1/0',
    })
  })

  it('renders an error message and a Retry button for failed status', async () => {
    const wrapper = mountCard(makeGeneration({ status: 'failed', images: [] }))
    expect(wrapper.text()).toContain('imageStudio.generationFailed')

    const retryBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('imageStudio.retry'))
    expect(retryBtn).toBeTruthy()

    await retryBtn!.trigger('click')
    const emitted = wrapper.emitted('retry')
    expect(emitted).toBeTruthy()
    expect((emitted![0][0] as ImageStudioGeneration).id).toBe(1)
  })

  it('renders prompt and param chips', () => {
    const wrapper = mountCard(makeGeneration())
    expect(wrapper.text()).toContain('a serene lake at dawn')
    expect(wrapper.text()).toContain('gpt-image-2')
    expect(wrapper.text()).toContain('imageStudio.modeGenerate')
    expect(wrapper.text()).toContain('1K')
  })

  it('emits open with the image src when an image is clicked', async () => {
    const wrapper = mountCard(makeGeneration())
    await wrapper.find('img').trigger('click')
    const emitted = wrapper.emitted('open')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toBe('blob:/assets/1/0')
  })

  it('renders source images + an image-to-image chip when input_images are present', () => {
    const wrapper = mountCard(
      makeGeneration({
        input_images: ['/input-assets/1/0'],
        images: ['/assets/1/0'],
      })
    )
    // i2i chip
    expect(wrapper.text()).toContain('imageStudio.imageToImage')
    // mode chip is inferred from the single source image when older rows do not
    // carry an explicit mode field.
    expect(wrapper.text()).toContain('imageStudio.modeEdit')
    // source row label
    expect(wrapper.text()).toContain('imageStudio.sourceImage')
    // The source thumbnail (via AuthedImage stub) + the output image both render.
    const srcs = wrapper.findAll('img').map((i) => i.attributes('src'))
    expect(srcs).toContain('blob:/input-assets/1/0')
    expect(srcs).toContain('blob:/assets/1/0')
  })

  it('renders no source row or i2i chip when input_images are absent', () => {
    const wrapper = mountCard(makeGeneration())
    expect(wrapper.text()).not.toContain('imageStudio.imageToImage')
    expect(wrapper.text()).not.toContain('imageStudio.sourceImage')
    const srcs = wrapper.findAll('img').map((i) => i.attributes('src'))
    expect(srcs).not.toContain('blob:/input-assets/1/0')
  })

  it('renders compose mode for multi-reference generations', () => {
    const wrapper = mountCard(
      makeGeneration({
        mode: 'compose',
        input_images: ['/input-assets/1/0', '/input-assets/1/1'],
        images: ['/assets/1/0'],
      })
    )
    expect(wrapper.text()).toContain('imageStudio.modeCompose')
  })
})
