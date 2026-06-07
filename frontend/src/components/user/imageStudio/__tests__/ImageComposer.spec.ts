import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
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
  })

  it('disables the Generate button when the prompt is empty', () => {
    const wrapper = mountComposer([makeGroup()])
    const btn = wrapper.find('button')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('emits generate with the selected group/model/size/quality/n payload', async () => {
    const groups = [makeGroup({ id: 7, name: 'Img A' })]
    const wrapper = mountComposer(groups)
    await flushPromises()

    // Fill prompt
    await wrapper.find('textarea').setValue('a cat riding a bike')

    // Selects are rendered in order: group, model, size, quality, count
    const selects = wrapper.findAll('select')
    expect(selects.length).toBe(5)
    // gpt-image-2 (default) — pick a non-default resolution tier + high quality
    await selects[2].setValue('2K') // size
    await selects[3].setValue('high') // quality
    await selects[4].setValue('3') // n

    const btn = wrapper.find('button')
    expect(btn.attributes('disabled')).toBeUndefined()
    await btn.trigger('click')

    const emitted = wrapper.emitted('generate')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toEqual({
      group_id: 7,
      prompt: 'a cat riding a bike',
      model: 'gpt-image-2',
      size: '2K',
      quality: 'high',
      n: 3,
    })
  })

  it('resets size & quality to the model defaults when the model changes', async () => {
    const groups = [makeGroup({ id: 7, name: 'Img A' })]
    const wrapper = mountComposer(groups)
    await flushPromises()

    await wrapper.find('textarea').setValue('a fox in a meadow')

    const selects = wrapper.findAll('select')
    // The size select offers the 1K/2K/4K resolution tiers.
    const sizeValues = selects[2].findAll('option').map((o) => o.attributes('value'))
    expect(sizeValues).toEqual(['1K', '2K', '4K'])

    // Move size/quality away from the defaults.
    await selects[2].setValue('4K')
    await selects[3].setValue('high')
    await flushPromises()

    // Switching the model (both share one matrix) should snap size/quality back
    // to that model's defaults via the watcher.
    await selects[1].setValue('gpt-image-1.5')
    await flushPromises()

    await wrapper.find('button').trigger('click')
    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({
      model: 'gpt-image-1.5',
      size: '1K',
      quality: 'auto',
    })
  })

  it('does not show a client-side cost estimate (server is source of truth)', async () => {
    const groups = [makeGroup({ id: 7, name: 'Img A' })]
    const wrapper = mountComposer(groups)
    await flushPromises()

    const selects = wrapper.findAll('select')
    // No price tables → no ≈$ label for any combo.
    await selects[3].setValue('high')
    await flushPromises()
    expect(wrapper.text()).not.toContain('≈$')

    await selects[3].setValue('auto')
    await flushPromises()
    expect(wrapper.text()).not.toContain('≈$')
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
    await wrapper.find('button').trigger('click')

    const emitted = wrapper.emitted('generate')
    expect(emitted![0][0]).toMatchObject({ group_id: 9 })
  })

  it('shows the no-image-group hint and disables Generate when no usable group', async () => {
    const groups = [makeGroup({ allow_image_generation: false })]
    const wrapper = mountComposer(groups, { loadingGroups: false })
    await flushPromises()

    expect(wrapper.text()).toContain('imageStudio.noImageGroupHint')
    await wrapper.find('textarea').setValue('something')
    expect(wrapper.find('button').attributes('disabled')).toBeDefined()
  })

  it('does not emit generate while generating', async () => {
    const wrapper = mountComposer([makeGroup()], { generating: true })
    await flushPromises()
    await wrapper.find('textarea').setValue('busy')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('generate')).toBeFalsy()
  })
})
