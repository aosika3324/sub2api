import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

import ImageComposer from '../ImageComposer.vue'
import type { Group } from '@/types'

// A minimal Select stub: renders a <select> bound to modelValue so the test can
// drive each control and assert the payload the composer emits.
const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean, Object], default: null },
    options: { type: Array, default: () => [] },
    disabled: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h(
        'select',
        {
          disabled: props.disabled,
          value: props.modelValue as string,
          onChange: (e: Event) => {
            const raw = (e.target as HTMLSelectElement).value
            const match = (props.options as Array<{ value: unknown; label: string }>).find(
              (o) => String(o.value) === raw
            )
            emit('update:modelValue', match ? match.value : raw)
          },
        },
        (props.options as Array<{ value: unknown; label: string }>).map((o) =>
          h('option', { value: String(o.value) }, o.label)
        )
      )
  },
})

const IconStub = defineComponent({ name: 'Icon', template: '<span />' })

function makeGroup(overrides: Partial<Group> = {}): Group {
  return {
    id: 1,
    name: 'Image Group',
    description: null,
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'none',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    allow_image_generation: true,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2026-06-07T00:00:00Z',
    updated_at: '2026-06-07T00:00:00Z',
    ...overrides,
  } as Group
}

function mountComposer(groups: Group[], props: Record<string, unknown> = {}) {
  return mount(ImageComposer, {
    props: { groups, ...props },
    global: {
      stubs: {
        Select: SelectStub,
        Icon: IconStub,
      },
    },
  })
}

describe('ImageComposer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // jsdom lacks object-URL helpers; stub them for the reference-image flow.
    URL.createObjectURL = vi.fn(() => 'blob:reference') as unknown as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as unknown as typeof URL.revokeObjectURL
  })

  it('disables the Generate button when the prompt is empty', () => {
    const wrapper = mountComposer([makeGroup()])
    const btn = wrapper.find('.send-button')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('emits generate with the default size (1024x1024) / quality / n payload', async () => {
    const groups = [makeGroup({ id: 7, name: 'Img A' })]
    const wrapper = mountComposer(groups)
    await flushPromises()

    await wrapper.find('textarea').setValue('a cat riding a bike')

    const btn = wrapper.find('.send-button')
    expect(btn.attributes('disabled')).toBeUndefined()
    await btn.trigger('click')

    const emitted = wrapper.emitted('generate')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toEqual({
      group_id: 7,
      mode: 'generate',
      prompt: 'a cat riding a bike',
      model: 'gpt-image-2',
      size: '1024x1024',
      quality: 'auto',
      n: 1,
      referenceImage: null,
      referenceImages: [],
    })
  })

  it('renders visible workbench controls', async () => {
    const wrapper = mountComposer([makeGroup()])
    await flushPromises()

    expect(wrapper.find('.workbench-panel').exists()).toBe(true)
    expect(wrapper.find('.studio-panel-header').exists()).toBe(true)
    expect(wrapper.find('.capability-strip').exists()).toBe(true)
    expect(wrapper.find('.reference-workbench').exists()).toBe(true)
    expect(wrapper.find('.prompt-panel').exists()).toBe(true)
    expect(wrapper.find('.model-chip-grid').exists()).toBe(true)
    expect(wrapper.find('.count-chip-grid').exists()).toBe(true)
    expect(wrapper.text()).toContain('imageStudio.workbenchTitle')
    expect(wrapper.text()).toContain('imageStudio.workbenchSubtitle')
    expect(wrapper.text()).toContain('imageStudio.workbenchModeTitle')
    expect(wrapper.text()).toContain('imageStudio.referenceWorkbenchTitle')
    expect(wrapper.text()).toContain('imageStudio.promptTitle')
    expect(wrapper.text()).toContain('imageStudio.modeGenerate')
    expect(wrapper.text()).toContain('imageStudio.modeEdit')
    expect(wrapper.text()).toContain('imageStudio.modeCompose')
    expect(wrapper.text()).toContain('imageStudio.modeGenerateHint')
    expect(wrapper.text()).toContain('imageStudio.referenceRequirementGenerate')
  })

  it('exposes only the requested image model choices', async () => {
    const wrapper = mountComposer([makeGroup()])
    await flushPromises()

    const options = wrapper
      .findAll('.model-chip')
      .map((option) => option.text())

    expect(options).toEqual([
      'gpt-image-2',
      'codex-gpt-image-2',
      'auto',
      'gpt-5',
      'gpt-5-1',
      'gpt-5-2',
      'gpt-5-3',
      'gpt-5-3-mini',
      'gpt-5-mini',
    ])
  })

  it('clicking an aspect preset updates the submitted size', async () => {
    const groups = [makeGroup({ id: 7 })]
    const wrapper = mountComposer(groups)
    await flushPromises()
    await wrapper.find('textarea').setValue('landscape please')

    // Pick the 16:9 preset (1024x576).
    const presetBtn = wrapper
      .findAll('.aspect-btn')
      .find((b) => b.text() === '16:9')
    expect(presetBtn).toBeTruthy()
    await presetBtn!.trigger('click')

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ size: '1024x576' })
  })

  it('selecting the auto preset submits size "auto"', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    await wrapper.find('textarea').setValue('auto size')
    const autoBtn = wrapper
      .findAll('.aspect-btn')
      .find((b) => b.text() === 'imageStudio.aspectAuto')
    expect(autoBtn).toBeTruthy()
    await autoBtn!.trigger('click')

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ size: 'auto' })
  })

  it('quality segmented control changes payload.quality', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    await wrapper.find('textarea').setValue('hi-q')
    const highBtn = wrapper
      .findAll('.segmented-btn')
      .find((b) => b.text() === 'imageStudio.qualityHigh')
    expect(highBtn).toBeTruthy()
    await highBtn!.trigger('click')

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ quality: 'high' })
  })

  it('count change is reflected in the payload', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    await wrapper.find('textarea').setValue('three please')
    const countButton = wrapper
      .findAll('.count-chip')
      .find((button) => button.text() === '3')
    expect(countButton).toBeTruthy()
    await countButton!.trigger('click')

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ n: 3 })
  })

  it('selecting reference files shows thumbnails and sets payload.referenceImages', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    await wrapper.find('textarea').setValue('image to image')

    const file = new File(['x'], 'src.png', { type: 'image/png' })
    const file2 = new File(['y'], 'src-2.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    // Drive the change handler with a faux file list.
    Object.defineProperty(input.element, 'files', { value: [file, file2], configurable: true })
    await input.trigger('change')

    // Thumbnail preview appears.
    const thumbs = wrapper.findAll('img[src="blob:reference"]')
    expect(thumbs).toHaveLength(2)

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ referenceImage: file, referenceImages: [file, file2] })
  })

  it('requires reference images for edit and compose modes', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    await wrapper.find('textarea').setValue('edit this')

    const editBtn = wrapper
      .findAll('.mode-card')
      .find((b) => b.text().includes('imageStudio.modeEdit'))
    expect(editBtn).toBeTruthy()
    await editBtn!.trigger('click')
    expect(wrapper.find('.send-button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('imageStudio.modeEditHint')
    expect(wrapper.text()).toContain('imageStudio.referenceRequirementEdit')

    const file = new File(['x'], 'src.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    expect(wrapper.find('.send-button').attributes('disabled')).toBeUndefined()

    const composeBtn = wrapper
      .findAll('.mode-card')
      .find((b) => b.text().includes('imageStudio.modeCompose'))
    expect(composeBtn).toBeTruthy()
    await composeBtn!.trigger('click')
    expect(wrapper.find('.send-button').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('imageStudio.modeComposeHint')
    expect(wrapper.text()).toContain('imageStudio.referenceRequirementCompose')
  })

  it('rejects a non-image reference file with an inline error', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()

    const file = new File(['x'], 'note.txt', { type: 'text/plain' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')

    expect(wrapper.text()).toContain('imageStudio.imageTypeError')
    expect(wrapper.find('img[src="blob:reference"]').exists()).toBe(false)
  })

  it('resetReference clears the file + thumbnail', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()

    const file = new File(['x'], 'src.png', { type: 'image/png' })
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    await input.trigger('change')
    expect(wrapper.find('img[src="blob:reference"]').exists()).toBe(true)

    wrapper.vm.resetReference()
    await flushPromises()
    expect(wrapper.find('img[src="blob:reference"]').exists()).toBe(false)
  })

  it('resets size & quality to model defaults when the model changes', async () => {
    const groups = [makeGroup({ id: 7, name: 'Img A' })]
    const wrapper = mountComposer(groups)
    await flushPromises()
    await wrapper.find('textarea').setValue('a fox in a meadow')

    // Move size away from default via an aspect preset + quality to high.
    const preset = wrapper.findAll('.aspect-btn').find((b) => b.text() === '4:3')
    await preset!.trigger('click')
    const high = wrapper
      .findAll('.segmented-btn')
      .find((b) => b.text() === 'imageStudio.qualityHigh')
    await high!.trigger('click')
    await flushPromises()

    // Switch the model — the watcher snaps size/quality back to defaults.
    const modelButton = wrapper
      .findAll('.model-chip')
      .find((button) => button.text() === 'gpt-5')
    expect(modelButton).toBeTruthy()
    await modelButton!.trigger('click')
    await flushPromises()

    await wrapper.find('.send-button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({
      model: 'gpt-5',
      size: '1024x1024',
      quality: 'auto',
    })
  })

  it('does not show a client-side cost estimate (server is source of truth)', async () => {
    const wrapper = mountComposer([makeGroup({ id: 7 })])
    await flushPromises()
    expect(wrapper.text()).not.toContain('~=')
  })

  it('fillPrompt populates the prompt textarea (example chips)', async () => {
    const wrapper = mountComposer([makeGroup()])
    await flushPromises()

    wrapper.vm.fillPrompt('a serene japanese garden')
    await flushPromises()

    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe(
      'a serene japanese garden'
    )
  })

  it('defaults the group to the first image-enabled group', async () => {
    const groups = [
      makeGroup({ id: 2, name: 'No Image', allow_image_generation: false }),
      makeGroup({ id: 9, name: 'Yes Image', allow_image_generation: true }),
    ]
    const wrapper = mountComposer(groups)
    await flushPromises()

    await wrapper.find('textarea').setValue('hello')
    await wrapper.find('.send-button').trigger('click')

    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ group_id: 9 })
  })

  it('shows the no-image-group hint and disables Generate when no usable group', async () => {
    const groups = [makeGroup({ allow_image_generation: false })]
    const wrapper = mountComposer(groups, { loadingGroups: false })
    await flushPromises()

    expect(wrapper.text()).toContain('imageStudio.noImageGroupHint')
    await wrapper.find('textarea').setValue('something')
    expect(wrapper.find('.send-button').attributes('disabled')).toBeDefined()
  })

  it('does not emit generate while generating', async () => {
    const wrapper = mountComposer([makeGroup()], { generating: true })
    await flushPromises()
    await wrapper.find('textarea').setValue('busy')
    await wrapper.find('.send-button').trigger('click')
    expect(wrapper.emitted('generate')).toBeFalsy()
  })
})
