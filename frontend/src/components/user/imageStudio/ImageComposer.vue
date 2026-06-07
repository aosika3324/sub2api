<template>
  <div class="composer">
    <!-- No usable group hint (subtle muted inline notice) -->
    <div
      v-if="!loadingGroups && imageGroups.length === 0"
      class="mb-2 flex items-center gap-2 rounded-xl border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/30 dark:bg-amber-900/15 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="xs" class="flex-shrink-0" />
      <span>{{ t('imageStudio.noImageGroupHint') }}</span>
    </div>

    <!-- Unified pill: textarea on top, compact controls + send below.
         Doubles as a drag-and-drop target for reference images. -->
    <div
      class="composer-shell relative rounded-[28px] border border-gray-100 bg-white shadow-sm transition-colors focus-within:border-primary-400 dark:border-dark-700/50 dark:bg-dark-800/50 dark:focus-within:border-primary-500"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="onDrop"
    >
      <!-- Drag overlay -->
      <div
        v-if="dragActive"
        class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-[28px] border-2 border-dashed border-primary-400 bg-primary-50/80 dark:border-primary-500 dark:bg-primary-900/30"
      >
        <span class="flex items-center gap-2 text-sm font-medium text-primary-700 dark:text-primary-300">
          <Icon name="upload" size="sm" />
          {{ t('imageStudio.referenceImage') }}
        </span>
      </div>

      <!-- Reference image thumbnail + error -->
      <div v-if="referenceUrl || referenceError" class="px-5 pt-4">
        <div
          v-if="referenceUrl"
          class="relative inline-block overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600"
        >
          <img
            :src="referenceUrl"
            :alt="t('imageStudio.referenceImage')"
            class="h-16 w-16 object-cover"
          />
          <button
            type="button"
            class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/55 text-white transition-colors hover:bg-black/75"
            :title="t('imageStudio.removeReference')"
            :aria-label="t('imageStudio.removeReference')"
            @click="resetReference"
          >
            <Icon name="x" size="xs" :stroke-width="2.5" />
          </button>
        </div>
        <p v-if="referenceError" class="mt-1 text-xs text-red-500">{{ referenceError }}</p>
      </div>

      <!-- Prompt -->
      <textarea
        ref="promptRef"
        v-model="prompt"
        rows="1"
        :disabled="disabled"
        class="composer-prompt"
        :placeholder="t('imageStudio.promptPlaceholder')"
        @input="autoGrow"
        @paste="onPaste"
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

        <!-- Settings summary pill (toggles the popover) -->
        <div ref="settingsWrapRef" class="relative">
          <button
            type="button"
            class="summary-pill"
            :class="{ 'summary-pill-open': settingsOpen }"
            :disabled="disabled"
            :aria-expanded="settingsOpen"
            :aria-haspopup="true"
            :title="t('imageStudio.imageSettings')"
            @click="toggleSettings"
          >
            <Icon name="sparkles" size="xs" class="flex-shrink-0 opacity-70" />
            <span class="truncate">{{ summaryText }}</span>
            <Icon
              :name="settingsOpen ? 'chevronDown' : 'chevronUp'"
              size="xs"
              class="flex-shrink-0 text-gray-400 dark:text-dark-400"
            />
          </button>

          <!-- Settings popover -->
          <div
            v-if="settingsOpen"
            class="settings-popover"
            role="dialog"
            :aria-label="t('imageStudio.imageSettings')"
            @mousedown.stop
          >
            <p class="settings-title">{{ t('imageStudio.imageSettings') }}</p>

            <!-- Model -->
            <div class="settings-row">
              <label class="settings-label">{{ t('imageStudio.model') }}</label>
              <Select
                v-model="model"
                class="settings-select"
                :options="modelOptions"
                :disabled="disabled"
                :aria-label="t('imageStudio.model')"
              />
            </div>

            <!-- Quality (segmented) -->
            <div class="settings-row">
              <label class="settings-label">{{ t('imageStudio.quality') }}</label>
              <div class="segmented" role="group" :aria-label="t('imageStudio.quality')">
                <button
                  v-for="q in qualityOptions"
                  :key="q.value"
                  type="button"
                  class="segmented-btn"
                  :class="{ 'segmented-btn-active': quality === q.value }"
                  :disabled="disabled"
                  :aria-pressed="quality === q.value"
                  @click="quality = q.value"
                >
                  {{ q.label }}
                </button>
              </div>
            </div>

            <!-- Custom size (W × H) -->
            <div class="settings-row">
              <div class="flex items-center justify-between">
                <label class="settings-label">{{ t('imageStudio.customSize') }}</label>
                <label class="auto-toggle">
                  <input
                    v-model="sizeAuto"
                    type="checkbox"
                    class="auto-checkbox"
                    :disabled="disabled"
                  />
                  <span>{{ t('imageStudio.aspectAuto') }}</span>
                </label>
              </div>
              <div class="mt-1.5 flex items-center gap-2">
                <input
                  v-model.number="width"
                  type="number"
                  min="1"
                  class="size-input"
                  :disabled="disabled || sizeAuto"
                  :placeholder="t('imageStudio.width')"
                  :aria-label="t('imageStudio.width')"
                />
                <span class="text-gray-400 dark:text-dark-500">×</span>
                <input
                  v-model.number="height"
                  type="number"
                  min="1"
                  class="size-input"
                  :disabled="disabled || sizeAuto"
                  :placeholder="t('imageStudio.height')"
                  :aria-label="t('imageStudio.height')"
                />
              </div>
            </div>

            <!-- Aspect ratio presets -->
            <div class="settings-row">
              <label class="settings-label">{{ t('imageStudio.aspectRatio') }}</label>
              <div class="aspect-grid" role="group" :aria-label="t('imageStudio.aspectRatio')">
                <button
                  v-for="preset in aspectPresets"
                  :key="preset.size"
                  type="button"
                  class="aspect-btn"
                  :class="{ 'aspect-btn-active': isPresetActive(preset) }"
                  :disabled="disabled"
                  :aria-pressed="isPresetActive(preset)"
                  @click="applyPreset(preset)"
                >
                  {{ presetLabel(preset) }}
                </button>
              </div>
            </div>

            <!-- Count -->
            <div class="settings-row">
              <label class="settings-label">{{ t('imageStudio.count') }}</label>
              <Select
                v-model="n"
                class="settings-select settings-select-narrow"
                :options="countOptions"
                :disabled="disabled"
                :aria-label="t('imageStudio.count')"
              />
            </div>
          </div>
        </div>

        <!-- Upload reference image -->
        <button
          type="button"
          class="upload-pill"
          :disabled="disabled"
          :title="t('imageStudio.upload')"
          :aria-label="t('imageStudio.upload')"
          @click="triggerFilePicker"
        >
          <Icon name="upload" size="xs" class="flex-shrink-0" />
          <span>{{ t('imageStudio.upload') }}</span>
        </button>
        <input
          ref="fileInputRef"
          type="file"
          accept="image/*"
          class="hidden"
          @change="onFileChange"
        />

        <!-- Balance pill -->
        <span
          class="balance-pill"
          :title="t('common.balance')"
        >
          <Icon name="dollar" size="xs" class="flex-shrink-0 text-green-500" />
          <span class="text-gray-400 dark:text-dark-500">{{ t('imageStudio.balanceShort') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">${{ balance.toFixed(2) }}</span>
        </span>

        <!-- Cost estimate + send -->
        <span
          v-if="costEstimate != null"
          class="ml-auto text-xs text-gray-400 dark:text-dark-500"
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
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  MODEL_OPTIONS,
  optionsForModel,
  defaultsForModel,
  estimateCost,
  parseSize,
  formatSize,
  ASPECT_PRESETS,
  COUNT_OPTIONS,
  type ModelId,
  type AspectPreset,
} from './pricing'

export interface ComposerSubmitPayload {
  group_id: number
  prompt: string
  model: string
  size: string
  quality: string
  n: number
  referenceImage?: File | null
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

const MAX_REFERENCE_BYTES = 20 * 1024 * 1024

// Only groups that allow image generation are usable.
const imageGroups = computed(() =>
  props.groups.filter((g) => g.allow_image_generation && g.status === 'active')
)

const promptRef = ref<HTMLTextAreaElement | null>(null)
const prompt = ref('')
const groupId = ref<number | null>(null)
const model = ref<ModelId>('gpt-image-2')
const quality = ref('auto')
const n = ref(1)

// Size is driven by W×H inputs + an `auto` toggle (the old single `size` Select
// was replaced by the popover's custom-size + aspect grid).
const width = ref(1024)
const height = ref(1024)
const sizeAuto = ref(false)

const balance = computed(() => props.balance ?? 0)

const groupOptions = computed(() =>
  imageGroups.value.map((g) => ({ value: g.id, label: g.name }))
)

// ---- Settings options ----
const modelOptions = MODEL_OPTIONS
const aspectPresets = ASPECT_PRESETS

const qualityOptions = computed(() =>
  optionsForModel(model.value).qualities.map((q) => ({
    value: q.value,
    label: q.labelKey ? t(q.labelKey) : q.value,
  }))
)

const countOptions = COUNT_OPTIONS.map((v) => ({ value: v, label: String(v) }))

// Submit payload size: the `auto` sentinel or a concrete "WxH" pair.
const submitSize = computed(() =>
  sizeAuto.value ? 'auto' : formatSize(width.value, height.value)
)

// Human summary for the trigger pill: matching preset label, else WxH, else auto.
const sizeSummary = computed(() => {
  if (sizeAuto.value) return t('imageStudio.aspectAuto')
  const match = ASPECT_PRESETS.find(
    (p) => p.size !== 'auto' && p.size === formatSize(width.value, height.value)
  )
  if (match) return match.label
  return `${width.value}×${height.value}`
})

const qualityLabel = computed(() => {
  const found = qualityOptions.value.find((q) => q.value === quality.value)
  return found ? found.label : quality.value
})

const summaryText = computed(
  () =>
    `${qualityLabel.value} · ${sizeSummary.value} · ${t('imageStudio.countShort', {
      count: n.value,
    })}`
)

// ---- Aspect preset helpers ----
function presetLabel(preset: AspectPreset): string {
  return preset.size === 'auto' ? t('imageStudio.aspectAuto') : preset.label
}

function isPresetActive(preset: AspectPreset): boolean {
  if (preset.size === 'auto') return sizeAuto.value
  if (sizeAuto.value) return false
  return preset.size === formatSize(width.value, height.value)
}

function applyPreset(preset: AspectPreset) {
  if (preset.size === 'auto') {
    sizeAuto.value = true
    return
  }
  const parsed = parseSize(preset.size)
  if (!parsed) return
  sizeAuto.value = false
  width.value = parsed.w
  height.value = parsed.h
}

// ---- Settings popover open/close ----
const settingsOpen = ref(false)
const settingsWrapRef = ref<HTMLElement | null>(null)

function onSettingsOutside(e: MouseEvent) {
  const target = e.target as HTMLElement
  // The Select inside the popover teleports its dropdown to <body>, so a click on
  // one of its options lands outside settingsWrapRef. Don't treat that (or any
  // teleported Select surface) as an "outside" click that should close us.
  if (target.closest?.('.select-dropdown-portal')) return
  if (settingsWrapRef.value && !settingsWrapRef.value.contains(target)) {
    closeSettings()
  }
}

function onSettingsKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') closeSettings()
}

function openSettings() {
  if (settingsOpen.value) return
  settingsOpen.value = true
  document.addEventListener('mousedown', onSettingsOutside)
  document.addEventListener('keydown', onSettingsKeydown)
}

function closeSettings() {
  if (!settingsOpen.value) return
  settingsOpen.value = false
  document.removeEventListener('mousedown', onSettingsOutside)
  document.removeEventListener('keydown', onSettingsKeydown)
}

function toggleSettings() {
  if (disabled.value) return
  settingsOpen.value ? closeSettings() : openSettings()
}

// When the model changes, reset size/quality to that model's defaults to avoid
// invalid combinations (both models currently share one matrix).
watch(model, (next) => {
  const d = defaultsForModel(next)
  quality.value = d.quality
  width.value = 1024
  height.value = 1024
  sizeAuto.value = false
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

// Best-effort client-side cost estimate. Currently always null (server is the
// source of truth); the markup reappears automatically if price tables land.
const costEstimate = computed(() =>
  estimateCost(model.value, submitSize.value, quality.value, n.value)
)

// ==================== Reference image (image-to-image) ====================

const fileInputRef = ref<HTMLInputElement | null>(null)
const referenceImage = ref<File | null>(null)
const referenceUrl = ref<string>('')
const referenceError = ref<string>('')
const dragActive = ref(false)

function setReference(file: File) {
  referenceError.value = ''
  if (!file.type.startsWith('image/')) {
    referenceError.value = t('imageStudio.imageTypeError')
    return
  }
  if (file.size > MAX_REFERENCE_BYTES) {
    referenceError.value = t('imageStudio.imageTooLarge')
    return
  }
  revokeReferenceUrl()
  referenceImage.value = file
  referenceUrl.value = URL.createObjectURL(file)
}

function revokeReferenceUrl() {
  if (referenceUrl.value) {
    URL.revokeObjectURL(referenceUrl.value)
    referenceUrl.value = ''
  }
}

function triggerFilePicker() {
  if (disabled.value) return
  fileInputRef.value?.click()
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) setReference(file)
  // Reset so selecting the same file again re-triggers change.
  input.value = ''
}

function onPaste(e: ClipboardEvent) {
  const file = e.clipboardData?.files?.[0]
  if (file && file.type.startsWith('image/')) {
    e.preventDefault()
    setReference(file)
  }
}

function onDragOver() {
  if (disabled.value) return
  dragActive.value = true
}

function onDragLeave() {
  dragActive.value = false
}

function onDrop(e: DragEvent) {
  dragActive.value = false
  if (disabled.value) return
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return
  const file = Array.from(files).find((f) => f.type.startsWith('image/'))
  if (file) setReference(file)
}

// Clears the reference file + revokes its object URL. Exposed for the parent so
// it can drop the source image after a successful generate.
function resetReference() {
  revokeReferenceUrl()
  referenceImage.value = null
  referenceError.value = ''
}

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
    size: submitSize.value,
    quality: quality.value,
    n: n.value,
    referenceImage: referenceImage.value,
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

onBeforeUnmount(() => {
  revokeReferenceUrl()
  document.removeEventListener('mousedown', onSettingsOutside)
  document.removeEventListener('keydown', onSettingsKeydown)
})

defineExpose({ resetPrompt, fillPrompt, resetReference })
</script>

<style scoped>
.composer-prompt {
  @apply w-full resize-none border-0 bg-transparent px-5 pb-2 pt-4 text-base leading-relaxed;
  @apply text-gray-900 dark:text-white;
  @apply placeholder:text-gray-400 dark:placeholder:text-dark-500;
  @apply focus:outline-none focus:ring-0;
  min-height: 60px;
}

/*
  Restyle the shared Select trigger LOCALLY into a small compact pill (shape
  only), without touching the global Select.vue. Colors follow the sub2api
  palette (slate surfaces, teal focus). Scoped :deep reaches the trigger button.
*/
.pill-select {
  @apply w-auto;
}

.pill-select :deep(.select-trigger) {
  @apply gap-1 rounded-full border-transparent bg-gray-100 px-3 py-1.5 text-xs;
  @apply text-gray-700;
  @apply hover:border-transparent hover:bg-gray-200;
  @apply focus:border-primary-500 focus:ring-2 focus:ring-primary-500/30;
  @apply dark:bg-dark-700 dark:text-gray-200;
  @apply dark:hover:bg-dark-600 dark:focus:border-primary-500;
  width: auto;
}

.pill-select :deep(.select-trigger-open) {
  @apply border-primary-500 ring-2 ring-primary-500/30;
}

.pill-select :deep(.select-trigger-disabled) {
  @apply bg-gray-100 dark:bg-dark-900;
}

.pill-select :deep(.select-value) {
  @apply truncate;
  max-width: 9rem;
}

.pill-select :deep(.select-icon) {
  @apply text-gray-400 dark:text-dark-400;
}

/* Settings summary pill (popover trigger). */
.summary-pill {
  @apply inline-flex max-w-[16rem] items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1.5 text-xs text-gray-700 transition-colors;
  @apply hover:bg-gray-200;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600;
}

.summary-pill-open {
  @apply ring-2 ring-primary-500/30;
}

.summary-pill:disabled {
  @apply cursor-not-allowed opacity-50;
}

/* Upload pill. */
.upload-pill {
  @apply inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1.5 text-xs text-gray-700 transition-colors;
  @apply hover:bg-gray-200;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600;
}

.upload-pill:disabled {
  @apply cursor-not-allowed opacity-50;
}

/* ============= Settings popover ============= */
.settings-popover {
  @apply absolute bottom-full left-0 z-30 mb-2 w-72 rounded-2xl border border-gray-100 bg-white p-4 shadow-xl;
  @apply dark:border-dark-700 dark:bg-dark-800;
}

.settings-title {
  @apply mb-3 text-sm font-semibold text-gray-900 dark:text-white;
}

.settings-row {
  @apply mb-3 last:mb-0;
}

.settings-label {
  @apply mb-1 block text-xs font-medium text-gray-500 dark:text-dark-400;
}

.settings-select :deep(.select-trigger) {
  @apply rounded-lg px-3 py-2 text-sm;
  @apply border-gray-200 bg-white text-gray-800;
  @apply hover:border-gray-300;
  @apply focus:border-primary-500 focus:ring-2 focus:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-900 dark:text-gray-200;
}

.settings-select-narrow {
  @apply w-24;
}

/* Segmented quality control. */
.segmented {
  @apply flex gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-900;
}

.segmented-btn {
  @apply flex-1 rounded-md px-2 py-1.5 text-xs font-medium text-gray-600 transition-colors;
  @apply hover:text-gray-900;
  @apply dark:text-gray-300 dark:hover:text-white;
}

.segmented-btn-active {
  @apply bg-primary-600 text-white shadow-sm;
  @apply hover:text-white;
  @apply dark:bg-primary-600 dark:text-white;
}

.segmented-btn:disabled {
  @apply cursor-not-allowed opacity-50;
}

/* Auto toggle (custom-size disabled state). */
.auto-toggle {
  @apply flex cursor-pointer select-none items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400;
}

.auto-checkbox {
  @apply h-3.5 w-3.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-900;
}

/* Width / height number inputs. */
.size-input {
  @apply w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900;
  @apply focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-900 dark:text-white;
  @apply disabled:cursor-not-allowed disabled:opacity-50;
}

/* Aspect preset grid. */
.aspect-grid {
  @apply grid grid-cols-3 gap-1.5;
}

.aspect-btn {
  @apply rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-[11px] font-medium text-gray-600 transition-colors;
  @apply hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700;
  @apply dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300;
  @apply dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

.aspect-btn-active {
  @apply border-primary-500 bg-primary-50 text-primary-700;
  @apply dark:border-primary-500 dark:bg-primary-900/30 dark:text-primary-300;
}

.aspect-btn:disabled {
  @apply cursor-not-allowed opacity-50;
}

.balance-pill {
  @apply inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1.5 text-xs text-gray-600;
  @apply dark:bg-dark-700 dark:text-gray-300;
}

.send-button {
  @apply flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full transition-colors;
  @apply bg-primary-600 text-white hover:bg-primary-700;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40;
}

.send-button:disabled {
  @apply cursor-not-allowed opacity-40;
  @apply hover:bg-primary-600;
}
</style>
