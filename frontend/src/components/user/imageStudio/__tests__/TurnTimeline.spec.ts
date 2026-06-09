import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

// TurnTimeline imports GenerationCard, which pulls in useAuthedImage → api client
// → the real i18n bootstrap. Stub the composable to break that chain (the
// GenerationCard render itself is replaced by a stub below).
vi.mock('@/composables/useAuthedImage', () => ({
  useAuthedImage: () => ({
    src: { value: '' },
    loading: { value: false },
    error: { value: null },
  }),
}))

import TurnTimeline from '../TurnTimeline.vue'
import type { ImageStudioGeneration } from '@/types'

const IconStub = defineComponent({ name: 'Icon', template: '<span />' })

// Stub GenerationCard so we can read each rendered turn's id without pulling in
// AuthedImage / network concerns.
const GenerationCardStub = defineComponent({
  name: 'GenerationCardStub',
  props: { generation: { type: Object, required: true } },
  emits: ['refresh', 'reference'],
  template: `
    <div class="gen-card" :data-id="generation.id">
      <button class="refresh" @click="$emit('refresh', generation)" />
      <button class="reference" @click="$emit('reference', { generation, url: generation.images[0] })" />
    </div>
  `,
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
    n: 1,
    image_count: 1,
    status: 'completed',
    cost: 0.08,
    created_at: '2026-06-07T00:00:00Z',
    images: ['/assets/1/0'],
    ...overrides,
  }
}

function mountTimeline(props: Record<string, unknown>) {
  return mount(TurnTimeline, {
    props,
    global: {
      stubs: { Icon: IconStub, GenerationCard: GenerationCardStub },
    },
  })
}

describe('TurnTimeline', () => {
  it('renders the onboarding hero with example chips when empty', () => {
    const wrapper = mountTimeline({ generations: [], loading: false, generating: false })
    expect(wrapper.text()).toContain('imageStudio.onboardingTitle')
    expect(wrapper.find('.onboarding-hero').exists()).toBe(true)
    // Four example chips
    expect(wrapper.findAll('.example-chip').length).toBe(4)
    expect(wrapper.findAll('.workbench-empty-item').length).toBe(3)
    expect(wrapper.text()).toContain('imageStudio.capabilityGenerate')
    expect(wrapper.text()).toContain('imageStudio.capabilityEdit')
    expect(wrapper.text()).toContain('imageStudio.capabilityHistory')
    expect(wrapper.findAllComponents(GenerationCardStub).length).toBe(0)
  })

  it('emits useExample with the chip prompt when a chip is clicked', async () => {
    const wrapper = mountTimeline({ generations: [], loading: false, generating: false })
    await wrapper.find('.example-chip').trigger('click')
    const emitted = wrapper.emitted('useExample')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toBe('imageStudio.examplePrompt1')
  })

  it('renders the loading skeleton on initial fetch with no data', () => {
    const wrapper = mountTimeline({ generations: [], loading: true, generating: false })
    expect(wrapper.find('.animate-pulse').exists()).toBe(true)
    expect(wrapper.find('.onboarding-hero').exists()).toBe(false)
  })

  it('renders generations oldest→newest (newest at the bottom)', () => {
    // Store keeps newest-first: id 3 (newest) ... id 1 (oldest).
    const generations = [
      makeGeneration({ id: 3, created_at: '2026-06-07T03:00:00Z' }),
      makeGeneration({ id: 2, created_at: '2026-06-07T02:00:00Z' }),
      makeGeneration({ id: 1, created_at: '2026-06-07T01:00:00Z' }),
    ]
    const wrapper = mountTimeline({ generations, loading: false, generating: false })

    const ids = wrapper.findAll('.gen-card').map((n) => n.attributes('data-id'))
    // Rendered top→bottom should be oldest→newest.
    expect(ids).toEqual(['1', '2', '3'])
  })

  it('renders the generating placeholder at the very bottom, after the last turn', () => {
    const generations = [
      makeGeneration({ id: 2, created_at: '2026-06-07T02:00:00Z' }),
      makeGeneration({ id: 1, created_at: '2026-06-07T01:00:00Z' }),
    ]
    const wrapper = mountTimeline({
      generations,
      loading: false,
      generating: true,
      pendingPrompt: 'a new prompt in flight',
    })

    // The compact live placeholder is present and shows the pending prompt.
    expect(wrapper.find('.live-generating-card').exists()).toBe(true)
    expect(wrapper.text()).toContain('a new prompt in flight')
    expect(wrapper.text()).toContain('imageStudio.generating')

    // Placeholder is the last child within the gallery (after newest turn id 2).
    const gallery = wrapper.find('.timeline-list')
    const children = Array.from(gallery.element.children)
    const lastChild = children[children.length - 1] as HTMLElement
    expect(lastChild.classList.contains('live-generating-card')).toBe(true)
    // The card right before it is the newest turn (id 2).
    const prevChild = children[children.length - 2] as HTMLElement
    expect(prevChild.getAttribute('data-id')).toBe('2')
  })

  it('forwards pending generation refresh events', async () => {
    const generation = makeGeneration({ id: 9, status: 'pending' })
    const wrapper = mountTimeline({
      generations: [generation],
      loading: false,
      generating: false,
    })

    await wrapper.find('.gen-card .refresh').trigger('click')
    const emitted = wrapper.emitted('refresh')
    expect(emitted).toBeTruthy()
    expect((emitted![0][0] as ImageStudioGeneration).id).toBe(9)
  })

  it('forwards add-reference events', async () => {
    const generation = makeGeneration({ id: 10, images: ['/assets/10/0'] })
    const wrapper = mountTimeline({
      generations: [generation],
      loading: false,
      generating: false,
    })

    await wrapper.find('.gen-card .reference').trigger('click')
    const emitted = wrapper.emitted('reference')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({ url: '/assets/10/0' })
  })
})
