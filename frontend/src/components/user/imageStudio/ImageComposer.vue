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
      class="composer-shell relative rounded-2xl border border-gray-100 bg-white shadow-sm transition-colors focus-within:border-primary-400 dark:border-dark-700/50 dark:bg-dark-800/70 dark:focus-within:border-primary-500"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="onDrop"
    >
      <!-- Drag overlay -->
      <div
        v-if="dragActive"
        class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-2xl border-2 border-dashed border-primary-400 bg-primary-50/80 dark:border-primary-500 dark:bg-primary-900/30"
      >
        <span class="flex items-center gap-2 text-sm font-medium text-primary-700 dark:text-primary-300">
          <Icon name="upload" size="sm" />
          {{ t('imageStudio.referenceImage') }}
        </span>
      </div>

      <!-- Reference image thumbnails + error -->
      <div v-if="referencePreviews.length > 0 || referenceError" class="px-5 pt-4">
        <div v-if="referencePreviews.length > 0" class="flex flex-wrap gap-2">
          <div
            v-for="(item, idx) in referencePreviews"
            :key="`${idx}-${item.url}`"
            class="relative overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
          >
            <img
              :src="item.url"
              :alt="t('imageStudio.referenceImage')"
              class="h-16 w-16 object-cover"
            />
            <button
              type="button"
              class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/55 text-white transition-colors hover:bg-black/75"
              :title="t('imageStudio.removeReference')"
              :aria-label="t('imageStudio.removeReference')"
              @click="removeReference(idx)"
            >
              <Icon name="x" size="xs" :stroke-width="2.5" />
            </button>
          </div>
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

      <!-- Visible workbench controls -->
      <div class="workbench-panel">
        <div class="workbench-row">
          <div class="control-field control-field-group">
            <span class="control-label">{{ t('imageStudio.group') }}</span>
            <Select
              v-model="groupId"
              class="workbench-select"
              :options="groupOptions"
              :disabled="disabled || imageGroups.length === 0"
              :placeholder="t('imageStudio.selectGroup')"
              :aria-label="t('imageStudio.group')"
            />
          </div>

          <div class="control-field control-field-mode">
            <span class="control-label">{{ t('imageStudio.mode') }}</span>
            <div class="segmented mode-segmented" role="group" :aria-label="t('imageStudio.mode')">
              <button
                v-for="option in modeOptions"
                :key="option.value"
                type="button"
                class="segmented-btn"
                :class="{ 'segmented-btn-active': mode === option.value }"
                :disabled="disabled"
                :aria-pressed="mode === option.value"
                @click="mode = option.value"
              >
                {{ option.label }}
              </button>
            </div>
            <div class="mode-summary">
              <span class="mode-summary-copy">{{ modeHint }}</span>
              <span
                class="mode-summary-requirement"
                :class="{ 'mode-summary-ok': hasRequiredReferenceSelection() }"
              >
                {{ referenceRequirement }}
              </span>
            </div>
          </div>

          <div class="control-field control-field-model">
            <span class="control-label">{{ t('imageStudio.model') }}</span>
            <Select
              v-model="model"
              class="workbench-select"
              :options="modelOptions"
              :disabled="disabled"
              :aria-label="t('imageStudio.model')"
            />
          </div>

          <div class="control-field control-field-count">
            <span class="control-label">{{ t('imageStudio.count') }}</span>
            <Select
              v-model="n"
              class="workbench-select"
              :options="countOptions"
              :disabled="disabled"
              :aria-label="t('imageStudio.count')"
            />
          </div>
        </div>

        <div class="workbench-row workbench-row-secondary">
          <div class="control-field control-field-quality">
            <span class="control-label">{{ t('imageStudio.quality') }}</span>
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

          <div class="control-field control-field-size">
            <span class="control-label">{{ t('imageStudio.aspectRatio') }}</span>
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

          <div class="control-field control-field-custom">
            <div class="flex items-center justify-between gap-3">
              <span class="control-label">{{ t('imageStudio.customSize') }}</span>
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
              <span class="text-gray-400 dark:text-dark-500">x</span>
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
        </div>

        <button
          type="button"
          class="reference-dropzone"
          :class="{ 'reference-dropzone-empty': referencePreviews.length === 0 }"
          :disabled="disabled"
          @click="triggerFilePicker"
        >
          <Icon name="image" size="sm" class="text-primary-500" />
          <span class="reference-dropzone-text">
            {{ referencePreviews.length > 0 ? t('imageStudio.referenceImage') : t('imageStudio.upload') }}
          </span>
          <span class="reference-dropzone-count">
            {{ referencePreviews.length }}/{{ MAX_REFERENCE_IMAGES }}
          </span>
        </button>
      </div>

      <!-- Action bar -->
      <div class="composer-controls action-bar">
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
          multiple
          class="hidden"
          @change="onFileChange"
        />

        <span class="balance-pill" :title="t('common.balance')">
          <Icon name="dollar" size="xs" class="flex-shrink-0 text-green-500" />
          <span class="text-gray-400 dark:text-dark-500">{{ t('imageStudio.balanceShort') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">${{ balance.toFixed(2) }}</span>
        </span>

        <span v-if="costEstimate != null" class="text-xs text-gray-400 dark:text-dark-500">
          ~= {{ costEstimate.toFixed(2) }}
        </span>
        <button
          type="button"
          class="send-button"
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
  mode: 'generate' | 'edit' | 'compose'
  prompt: string
  model: string
  size: string
  quality: string
  n: number
  referenceImage?: File | null
  referenceImages?: File[]
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
const MAX_REFERENCE_IMAGES = 8

// Only groups that allow image generation are usable.
const imageGroups = computed(() =>
  props.groups.filter((g) => g.allow_image_generation && g.status === 'active')
)

const promptRef = ref<HTMLTextAreaElement | null>(null)
const prompt = ref('')
const groupId = ref<number | null>(null)
const model = ref<ModelId>('gpt-image-2')
const mode = ref<'generate' | 'edit' | 'compose'>('generate')
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
const modeOptions = computed(() => [
  { value: 'generate' as const, label: t('imageStudio.modeGenerate') },
  { value: 'edit' as const, label: t('imageStudio.modeEdit') },
  { value: 'compose' as const, label: t('imageStudio.modeCompose') },
])

const modeHint = computed(() => {
  if (mode.value === 'edit') return t('imageStudio.modeEditHint')
  if (mode.value === 'compose') return t('imageStudio.modeComposeHint')
  return t('imageStudio.modeGenerateHint')
})

const referenceRequirement = computed(() => {
  if (mode.value === 'edit') return t('imageStudio.referenceRequirementEdit')
  if (mode.value === 'compose') return t('imageStudio.referenceRequirementCompose')
  return t('imageStudio.referenceRequirementGenerate')
})

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

// When the model changes, reset size/quality to that model's defaults to avoid
// invalid combinations (both models currently share one matrix).
watch(model, (next) => {
  const d = defaultsForModel(next)
  quality.value = d.quality
  width.value = 1024
  height.value = 1024
  sizeAuto.value = false
})

watch(mode, (next) => {
  if (next === 'generate' && referenceImages.value.length > 0) {
    resetReference()
  }
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
    imageGroups.value.length > 0 &&
    hasRequiredReferenceSelection()
)

// Best-effort client-side cost estimate. Currently always null (server is the
// source of truth); the markup reappears automatically if price tables land.
const costEstimate = computed(() =>
  estimateCost(model.value, submitSize.value, quality.value, n.value)
)

// ==================== Reference image (image-to-image) ====================

const fileInputRef = ref<HTMLInputElement | null>(null)
const referenceImages = ref<File[]>([])
const referencePreviews = ref<Array<{ file: File; url: string }>>([])
const referenceError = ref<string>('')
const dragActive = ref(false)

function hasRequiredReferenceSelection(): boolean {
  if (mode.value === 'edit') return referenceImages.value.length === 1
  if (mode.value === 'compose') return referenceImages.value.length >= 2
  return true
}

function addReferences(files: File[]) {
  referenceError.value = ''
  const nextFiles: File[] = []
  for (const file of files) {
    if (!file.type.startsWith('image/')) {
      referenceError.value = t('imageStudio.imageTypeError')
      continue
    }
    if (file.size > MAX_REFERENCE_BYTES) {
      referenceError.value = t('imageStudio.imageTooLarge')
      continue
    }
    nextFiles.push(file)
  }
  if (nextFiles.length === 0) {
    return
  }
  const capacity = Math.max(0, MAX_REFERENCE_IMAGES - referenceImages.value.length)
  const accepted = nextFiles.slice(0, capacity)
  if (accepted.length < nextFiles.length) {
    referenceError.value = t('imageStudio.tooManyReferences', { count: MAX_REFERENCE_IMAGES })
  }
  referenceImages.value = [...referenceImages.value, ...accepted]
  referencePreviews.value = [
    ...referencePreviews.value,
    ...accepted.map((file) => ({ file, url: URL.createObjectURL(file) })),
  ]
  if (referenceImages.value.length === 1 && mode.value === 'generate') {
    mode.value = 'edit'
  } else if (referenceImages.value.length > 1) {
    mode.value = 'compose'
  }
}

function revokeReferenceUrls() {
  for (const item of referencePreviews.value) {
    URL.revokeObjectURL(item.url)
  }
  referencePreviews.value = []
}

function removeReference(index: number) {
  const item = referencePreviews.value[index]
  if (item) {
    URL.revokeObjectURL(item.url)
  }
  referencePreviews.value = referencePreviews.value.filter((_, idx) => idx !== index)
  referenceImages.value = referenceImages.value.filter((_, idx) => idx !== index)
  if (referenceImages.value.length === 0 && mode.value !== 'generate') {
    mode.value = 'generate'
  } else if (referenceImages.value.length === 1 && mode.value === 'compose') {
    mode.value = 'edit'
  }
}

function triggerFilePicker() {
  if (disabled.value) return
  fileInputRef.value?.click()
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  if (files.length > 0) addReferences(files)
  // Reset so selecting the same file again re-triggers change.
  input.value = ''
}

function onPaste(e: ClipboardEvent) {
  const files = e.clipboardData?.files ? Array.from(e.clipboardData.files) : []
  const images = files.filter((file) => file.type.startsWith('image/'))
  if (images.length > 0) {
    e.preventDefault()
    addReferences(images)
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
  const images = Array.from(files).filter((f) => f.type.startsWith('image/'))
  if (images.length > 0) addReferences(images)
}

// Clears the reference file + revokes its object URL. Exposed for the parent so
// it can drop the source image after a successful generate.
function resetReference() {
  revokeReferenceUrls()
  referenceImages.value = []
  referenceError.value = ''
  if (mode.value !== 'generate') {
    mode.value = 'generate'
  }
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
    mode: mode.value,
    prompt: prompt.value.trim(),
    model: model.value,
    size: submitSize.value,
    quality: quality.value,
    n: n.value,
    referenceImage: referenceImages.value[0] ?? null,
    referenceImages: [...referenceImages.value],
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
  revokeReferenceUrls()
})

defineExpose({ resetPrompt, fillPrompt, resetReference })
</script>

<style scoped>
.composer-prompt {
  @apply w-full resize-none border-0 bg-transparent px-5 pb-3 pt-4 text-base leading-relaxed;
  @apply text-gray-900 dark:text-white;
  @apply placeholder:text-gray-400 dark:placeholder:text-dark-500;
  @apply focus:outline-none focus:ring-0;
  min-height: 60px;
}

.workbench-panel {
  @apply mx-3 mb-3 rounded-xl border border-gray-100 bg-gray-50/90 p-4;
  @apply dark:border-dark-700/70 dark:bg-dark-900/60;
}

.workbench-row {
  @apply grid gap-3;
  grid-template-columns: minmax(10rem, 1fr) minmax(15rem, 1.25fr) minmax(15rem, 1.2fr) minmax(6rem, 0.55fr);
}

.workbench-row-secondary {
  @apply mt-4;
  grid-template-columns: minmax(15rem, 1fr) minmax(20rem, 1.6fr) minmax(13rem, 0.9fr);
}

@media (max-width: 1024px) {
  .workbench-row,
  .workbench-row-secondary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .workbench-row,
  .workbench-row-secondary {
    grid-template-columns: minmax(0, 1fr);
  }
}

.control-field {
  @apply min-w-0;
}

.control-label {
  @apply mb-1.5 block text-[11px] font-semibold uppercase tracking-normal text-gray-500 dark:text-dark-300;
}

.workbench-select :deep(.select-trigger) {
  @apply h-9 rounded-lg border-gray-200 bg-white px-3 py-2 text-sm text-gray-800;
  @apply hover:border-gray-300;
  @apply focus:border-primary-500 focus:ring-2 focus:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200;
}

.workbench-select :deep(.select-value) {
  @apply truncate;
}

.workbench-select :deep(.select-trigger-disabled) {
  @apply opacity-60;
}

.reference-dropzone {
  @apply mt-4 flex w-full items-center gap-2 rounded-lg border border-dashed border-primary-300 bg-primary-50/70 px-3 py-2.5 text-left text-sm text-primary-700 transition-colors;
  @apply hover:bg-primary-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:border-primary-700/60 dark:bg-primary-900/20 dark:text-primary-300 dark:hover:bg-primary-900/30;
}

.reference-dropzone-empty {
  @apply text-gray-500 dark:text-dark-300;
}

.reference-dropzone:disabled {
  @apply cursor-not-allowed opacity-60;
}

.reference-dropzone-text {
  @apply min-w-0 flex-1 truncate;
}

.reference-dropzone-count {
  @apply rounded-full bg-white/80 px-2 py-0.5 text-xs font-semibold text-primary-700 dark:bg-dark-800 dark:text-primary-300;
}

.action-bar {
  @apply flex flex-wrap items-center gap-2 px-3 pb-3;
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

.mode-segmented {
  @apply rounded-lg p-1;
}

.mode-segmented .segmented-btn {
  @apply rounded-md px-2.5 py-1.5 text-xs;
}

.mode-summary {
  @apply mt-2 flex flex-wrap items-center gap-1.5 text-[11px] leading-snug text-gray-500 dark:text-dark-300;
}

.mode-summary-copy {
  @apply min-w-0 flex-1;
}

.mode-summary-requirement {
  @apply rounded-md bg-amber-50 px-1.5 py-0.5 font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300;
}

.mode-summary-ok {
  @apply bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300;
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
