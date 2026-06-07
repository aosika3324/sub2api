<template>
  <div class="composer card overflow-hidden">
    <!-- No usable group hint -->
    <div
      v-if="!loadingGroups && imageGroups.length === 0"
      class="flex items-center gap-2 border-b border-amber-100 bg-amber-50 px-4 py-2.5 text-sm text-amber-700 dark:border-amber-900/30 dark:bg-amber-900/15 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="sm" class="flex-shrink-0" />
      <span>{{ t('imageStudio.noImageGroupHint') }}</span>
    </div>

    <!-- Prompt hero -->
    <div class="relative">
      <textarea
        ref="promptRef"
        v-model="prompt"
        rows="1"
        :disabled="disabled"
        class="composer-prompt"
        :placeholder="t('imageStudio.promptPlaceholder')"
        @input="autoGrow"
        @keydown.ctrl.enter.prevent="submit"
        @keydown.meta.enter.prevent="submit"
      ></textarea>
    </div>

    <!-- Secondary control row -->
    <div
      class="flex flex-wrap items-center gap-x-4 gap-y-3 border-t border-gray-100 px-4 py-3 dark:border-dark-700/60"
    >
      <!-- Group (drives billing) -->
      <label class="composer-field min-w-[150px] flex-1 basis-44">
        <span class="composer-label">{{ t('imageStudio.group') }}</span>
        <Select
          v-model="groupId"
          :options="groupOptions"
          :disabled="disabled || imageGroups.length === 0"
          :placeholder="t('imageStudio.selectGroup')"
        />
      </label>

      <!-- Model -->
      <label class="composer-field w-[150px]">
        <span class="composer-label">{{ t('imageStudio.model') }}</span>
        <Select v-model="model" :options="modelOptions" :disabled="disabled" />
      </label>

      <!-- Size -->
      <label class="composer-field w-[148px]">
        <span class="composer-label">{{ t('imageStudio.size') }}</span>
        <Select v-model="size" :options="sizeOptions" :disabled="disabled" />
      </label>

      <!-- Quality -->
      <label class="composer-field w-[132px]">
        <span class="composer-label">{{ t('imageStudio.quality') }}</span>
        <Select v-model="quality" :options="qualityOptions" :disabled="disabled" />
      </label>

      <!-- Count -->
      <label class="composer-field w-[92px]">
        <span class="composer-label">{{ t('imageStudio.count') }}</span>
        <Select v-model="n" :options="countOptions" :disabled="disabled" />
      </label>
    </div>

    <!-- Action bar: balance + generate -->
    <div
      class="flex flex-wrap items-center gap-3 border-t border-gray-100 bg-gray-50/60 px-4 py-3 dark:border-dark-700/60 dark:bg-dark-900/30"
    >
      <!-- Balance -->
      <div class="flex items-center gap-1.5 text-sm">
        <Icon name="dollar" size="sm" class="text-green-500" />
        <span class="text-gray-400 dark:text-dark-500">{{ t('common.balance') }}</span>
        <span class="font-semibold text-gray-900 dark:text-white">${{ balance.toFixed(2) }}</span>
      </div>

      <!-- Submit hint -->
      <span class="hidden text-xs text-gray-400 dark:text-dark-500 sm:inline">{{
        t('imageStudio.submitHint')
      }}</span>

      <!-- Generate -->
      <button
        type="button"
        class="btn btn-primary ml-auto h-11 min-w-[150px] justify-center px-5 text-[15px]"
        :disabled="!canGenerate"
        @click="submit"
      >
        <template v-if="generating">
          <span
            class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"
          ></span>
          {{ t('imageStudio.generatingShort') }}
        </template>
        <template v-else>
          <Icon name="sparkles" size="sm" class="mr-1.5" />
          {{ generateLabel }}
        </template>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  MODEL_OPTIONS,
  optionsForModel,
  defaultsForModel,
  estimateCost,
  type ModelId,
} from './pricing'

export interface ComposerSubmitPayload {
  group_id: number
  prompt: string
  model: string
  size: string
  quality: string
  n: number
}

const props = defineProps<{
  groups: Group[]
  loadingGroups?: boolean
  generating?: boolean
  balance?: number
}>()

const emit = defineEmits<{
  (e: 'generate', payload: ComposerSubmitPayload): void
}>()

const { t } = useI18n()

// Only groups that allow image generation are usable.
const imageGroups = computed(() =>
  props.groups.filter((g) => g.allow_image_generation && g.status === 'active')
)

const promptRef = ref<HTMLTextAreaElement | null>(null)
const prompt = ref('')
const groupId = ref<number | null>(null)
const model = ref<ModelId>('gpt-image-1')
const size = ref('1024x1024')
const quality = ref('auto')
const n = ref(1)

const balance = computed(() => props.balance ?? 0)

const groupOptions = computed(() =>
  imageGroups.value.map((g) => ({ value: g.id, label: g.name }))
)

// ---- Model-aware options ----
const modelOptions = MODEL_OPTIONS

const sizeOptions = computed(() =>
  optionsForModel(model.value).sizes.map((s) => ({
    value: s.value,
    label: s.label,
  }))
)

const qualityOptions = computed(() =>
  optionsForModel(model.value).qualities.map((q) => ({
    value: q.value,
    label: q.labelKey ? t(q.labelKey) : q.value,
  }))
)

const countOptions = [1, 2, 3, 4].map((v) => ({ value: v, label: String(v) }))

// When the model changes, reset size/quality to valid defaults for that model
// to prevent invalid combinations.
watch(model, (next) => {
  const d = defaultsForModel(next)
  size.value = d.size
  quality.value = d.quality
})

// Default the group selection to the first usable group.
watch(
  imageGroups,
  (list) => {
    if (groupId.value === null && list.length > 0) {
      groupId.value = list[0].id
    } else if (groupId.value !== null && !list.some((g) => g.id === groupId.value)) {
      // Previously selected group is no longer usable — reset.
      groupId.value = list.length > 0 ? list[0].id : null
    }
  },
  { immediate: true }
)

const disabled = computed(() => props.generating === true)

const canGenerate = computed(
  () =>
    !disabled.value &&
    prompt.value.trim().length > 0 &&
    groupId.value !== null &&
    imageGroups.value.length > 0
)

// Best-effort client-side cost estimate. Only shows a price when we are
// confident; otherwise the button just reads "Generate".
const costEstimate = computed(() => estimateCost(model.value, size.value, quality.value, n.value))

const generateLabel = computed(() => {
  if (costEstimate.value != null) {
    return t('imageStudio.generateWithCost', {
      cost: `≈$${costEstimate.value.toFixed(2)}`,
    })
  }
  return t('imageStudio.generate')
})

function autoGrow() {
  const el = promptRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 320)}px`
}

function submit() {
  if (!canGenerate.value || groupId.value === null) return
  emit('generate', {
    group_id: groupId.value,
    prompt: prompt.value.trim(),
    model: model.value,
    size: size.value,
    quality: quality.value,
    n: n.value,
  })
}

// Allow the parent to clear the prompt after a successful generate.
function resetPrompt() {
  prompt.value = ''
  nextTick(autoGrow)
}

// Allow the parent (onboarding chips) to fill + focus the prompt.
function fillPrompt(value: string) {
  prompt.value = value
  nextTick(() => {
    autoGrow()
    promptRef.value?.focus()
    const len = value.length
    promptRef.value?.setSelectionRange(len, len)
  })
}

onMounted(autoGrow)

defineExpose({ resetPrompt, fillPrompt })
</script>

<style scoped>
.composer-prompt {
  @apply w-full resize-none border-0 bg-transparent px-4 py-4 text-base leading-relaxed;
  @apply text-gray-900 dark:text-white;
  @apply placeholder:text-gray-400 dark:placeholder:text-dark-500;
  @apply focus:outline-none focus:ring-0;
  min-height: 88px;
}

.composer-field {
  @apply block;
}

.composer-label {
  @apply mb-1 block text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500;
}
</style>
