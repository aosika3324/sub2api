<template>
  <div class="composer">
    <!-- No usable group hint (subtle muted inline notice) -->
    <div
      v-if="!loadingGroups && imageGroups.length === 0"
      class="mb-2 flex items-center gap-2 px-1 text-xs text-amber-600 dark:text-amber-400/80"
    >
      <Icon name="exclamationTriangle" size="xs" class="flex-shrink-0" />
      <span>{{ t('imageStudio.noImageGroupHint') }}</span>
    </div>

    <!-- Unified pill: textarea on top, compact controls + send below -->
    <div
      class="composer-shell rounded-[28px] border border-gray-200 bg-white shadow-sm transition-colors focus-within:border-gray-300 dark:border-white/10 dark:bg-white/[0.03] dark:shadow-black/20 dark:focus-within:border-white/20"
    >
      <!-- Prompt -->
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

      <!-- Control bar -->
      <div class="composer-controls flex flex-wrap items-center gap-2 px-3 pb-3">
        <!-- Group (drives billing) -->
        <Select
          v-model="groupId"
          class="pill-select"
          :options="groupOptions"
          :disabled="disabled || imageGroups.length === 0"
          :placeholder="t('imageStudio.selectGroup')"
          :title="t('imageStudio.group')"
          :aria-label="t('imageStudio.group')"
        />

        <!-- Model -->
        <Select
          v-model="model"
          class="pill-select"
          :options="modelOptions"
          :disabled="disabled"
          :title="t('imageStudio.model')"
          :aria-label="t('imageStudio.model')"
        />

        <!-- Size -->
        <Select
          v-model="size"
          class="pill-select"
          :options="sizeOptions"
          :disabled="disabled"
          :title="t('imageStudio.size')"
          :aria-label="t('imageStudio.size')"
        />

        <!-- Quality -->
        <Select
          v-model="quality"
          class="pill-select"
          :options="qualityOptions"
          :disabled="disabled"
          :title="t('imageStudio.quality')"
          :aria-label="t('imageStudio.quality')"
        />

        <!-- Count -->
        <Select
          v-model="n"
          class="pill-select pill-select-narrow"
          :options="countOptions"
          :disabled="disabled"
          :title="t('imageStudio.count')"
          :aria-label="t('imageStudio.count')"
        />

        <!-- Balance pill -->
        <span
          class="balance-pill"
          :title="t('common.balance')"
        >
          <Icon name="dollar" size="xs" class="flex-shrink-0 opacity-60" />
          <span class="opacity-60">{{ t('imageStudio.balanceShort') }}</span>
          <span class="font-medium">${{ balance.toFixed(2) }}</span>
        </span>

        <!-- Cost estimate + send -->
        <span
          v-if="costEstimate != null"
          class="ml-auto text-xs text-gray-400 dark:text-white/40"
        >
          ≈${{ costEstimate.toFixed(2) }}
        </span>
        <button
          type="button"
          class="send-button"
          :class="{ 'ml-auto': costEstimate == null }"
          :disabled="!canGenerate"
          :aria-label="t('imageStudio.sendAria')"
          :title="t('imageStudio.sendAria')"
          @click="submit"
        >
          <span
            v-if="generating"
            class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"
          ></span>
          <Icon v-else name="arrowUp" size="sm" :stroke-width="2" />
        </button>
      </div>
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

// Best-effort client-side cost estimate. Only surfaced as a faint label next to
// the send button when we are confident; otherwise it is hidden.
const costEstimate = computed(() => estimateCost(model.value, size.value, quality.value, n.value))

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
  @apply w-full resize-none border-0 bg-transparent px-5 pb-2 pt-4 text-base leading-relaxed;
  @apply text-gray-900 dark:text-white;
  @apply placeholder:text-gray-400 dark:placeholder:text-white/35;
  @apply focus:outline-none focus:ring-0;
  min-height: 60px;
}

/*
  Restyle the shared Select trigger LOCALLY into a small neutral pill, without
  touching the global Select.vue. Scoped :deep reaches the trigger button.
*/
.pill-select {
  @apply w-auto;
}

.pill-select :deep(.select-trigger) {
  @apply gap-1 rounded-full border-gray-200 bg-gray-900/[0.03] px-3 py-1.5 text-xs;
  @apply text-gray-600;
  @apply hover:border-gray-300;
  @apply focus:border-gray-300 focus:ring-2 focus:ring-gray-400/20;
  @apply dark:border-white/10 dark:bg-white/[0.05] dark:text-white/75;
  @apply dark:hover:border-white/20 dark:focus:border-white/20 dark:focus:ring-white/20;
  width: auto;
}

.pill-select :deep(.select-trigger-open) {
  @apply border-gray-300 ring-2 ring-gray-400/20;
  @apply dark:border-white/20 dark:ring-white/20;
}

.pill-select :deep(.select-trigger-disabled) {
  @apply bg-gray-900/[0.02] dark:bg-white/[0.02];
}

.pill-select :deep(.select-value) {
  @apply truncate;
  max-width: 9rem;
}

.pill-select-narrow :deep(.select-value) {
  max-width: 2.5rem;
}

.pill-select :deep(.select-icon) {
  @apply text-gray-400 dark:text-white/40;
}

.balance-pill {
  @apply inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-gray-900/[0.03] px-3 py-1.5 text-xs text-gray-600;
  @apply dark:border-white/10 dark:bg-white/[0.05] dark:text-white/75;
}

.send-button {
  @apply flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full transition-colors;
  @apply bg-gray-900 text-white hover:bg-gray-800;
  @apply dark:bg-white dark:text-black dark:hover:bg-white/90;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-gray-400/40 dark:focus-visible:ring-white/25;
}

.send-button:disabled {
  @apply cursor-not-allowed opacity-40;
  @apply hover:bg-gray-900 dark:hover:bg-white;
}
</style>
