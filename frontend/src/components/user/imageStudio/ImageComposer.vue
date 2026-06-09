<template>
  <div class="composer">
    <div class="studio-panel-header">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="studio-kicker">{{ t('imageStudio.title') }}</p>
          <h2 class="studio-title">{{ t('imageStudio.workbenchTitle') }}</h2>
          <p class="studio-subtitle">{{ t('imageStudio.workbenchSubtitle') }}</p>
        </div>
        <span class="balance-pill" :title="t('common.balance')">
          <Icon name="dollar" size="xs" class="flex-shrink-0 text-green-500" />
          <span class="text-gray-400 dark:text-dark-500">{{ t('imageStudio.balanceShort') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">${{ balance.toFixed(2) }}</span>
        </span>
      </div>

      <div class="studio-mode-summary">
        <span>{{ activeModeLabel }}</span>
        <span>{{ model }}</span>
        <span>{{ submitSize }}</span>
        <span>{{ t('imageStudio.countShort', { count: n }) }}</span>
      </div>
    </div>

    <div
      v-if="!loadingGroups && imageGroups.length === 0"
      class="mx-3 mt-3 flex items-center gap-2 rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/30 dark:bg-amber-900/15 dark:text-amber-300"
    >
      <Icon name="exclamationTriangle" size="xs" class="flex-shrink-0" />
      <span>{{ t('imageStudio.noImageGroupHint') }}</span>
    </div>

    <div
      class="composer-shell"
      @dragover.prevent="onDragOver"
      @dragleave.prevent="onDragLeave"
      @drop.prevent="onDrop"
    >
      <div
        v-if="dragActive"
        class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-lg border-2 border-dashed border-primary-400 bg-primary-50/80 dark:border-primary-500 dark:bg-primary-900/30"
      >
        <span class="flex items-center gap-2 text-sm font-medium text-primary-700 dark:text-primary-300">
          <Icon name="upload" size="sm" />
          {{ t('imageStudio.referenceImage') }}
        </span>
      </div>

      <div class="composer-main">
        <section class="workbench-panel">
          <div class="compact-grid">
            <div class="control-field">
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

            <div class="control-field">
              <span class="control-label">{{ t('imageStudio.model') }}</span>
              <Select
                :model-value="model"
                class="workbench-select model-select"
                :options="modelSelectOptions"
                :disabled="disabled"
                :searchable="false"
                :aria-label="t('imageStudio.model')"
                @update:model-value="selectModelValue"
              />
            </div>
          </div>

          <div class="mode-card-grid" role="group" :aria-label="t('imageStudio.mode')">
            <button
              v-for="option in modeOptions"
              :key="option.value"
              type="button"
              class="mode-card"
              :class="{ 'mode-card-active': mode === option.value }"
              :disabled="disabled"
              :aria-pressed="mode === option.value"
              @click="mode = option.value"
            >
              <span class="mode-card-title">{{ option.label }}</span>
              <span class="mode-card-copy">{{ modeHintFor(option.value) }}</span>
            </button>
          </div>

          <button
            type="button"
            class="settings-toggle"
            :aria-expanded="settingsOpen"
            @click="settingsOpen = !settingsOpen"
          >
            <span class="inline-flex items-center gap-2">
              <Icon name="cog" size="sm" />
              <span>{{ t('imageStudio.imageSettings') }}</span>
            </span>
            <span class="settings-summary">{{ settingsSummary }}</span>
            <Icon
              name="chevronDown"
              size="sm"
              class="settings-chevron"
              :class="{ 'rotate-180': settingsOpen }"
            />
          </button>

          <Transition name="fold">
            <div v-if="settingsOpen" class="advanced-settings">
              <div class="control-field">
                <span class="control-label">{{ t('imageStudio.count') }}</span>
                <div class="count-chip-grid" role="radiogroup" :aria-label="t('imageStudio.count')">
                  <button
                    v-for="count in countOptions"
                    :key="count.value"
                    type="button"
                    class="count-chip"
                    :class="{ 'count-chip-active': n === count.value }"
                    :disabled="disabled"
                    :aria-checked="n === count.value"
                    role="radio"
                    @click="selectCount(count.value)"
                  >
                    {{ count.label }}
                  </button>
                </div>
              </div>

              <div class="control-field">
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

              <div class="control-field">
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

              <div class="control-field">
                <div class="flex items-center justify-between gap-3">
                  <span class="control-label mb-0">{{ t('imageStudio.customSize') }}</span>
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
                <div class="mt-2 flex items-center gap-2">
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
          </Transition>

          <div class="reference-workbench">
            <div class="reference-workbench-header">
              <div class="min-w-0">
                <span class="section-eyebrow">{{ t('imageStudio.stepReference') }}</span>
                <h3>{{ t('imageStudio.referenceWorkbenchTitle') }}</h3>
              </div>
              <span
                class="mode-status-pill"
                :class="{ 'mode-status-ok': hasRequiredReferenceSelection() }"
              >
                {{ referenceRequirement }}
              </span>
            </div>
            <button
              type="button"
              class="reference-dropzone"
              :class="{ 'reference-dropzone-empty': referencePreviews.length === 0 }"
              :disabled="disabled"
              @click="triggerFilePicker"
            >
              <Icon name="upload" size="sm" class="text-primary-500" />
              <span class="reference-dropzone-text">
                {{ referencePreviews.length > 0 ? t('imageStudio.referenceImage') : t('imageStudio.upload') }}
              </span>
              <span class="reference-counter">
                {{ referencePreviews.length }}/{{ MAX_REFERENCE_IMAGES }}
              </span>
            </button>
            <div v-if="referencePreviews.length > 0" class="reference-grid">
              <div
                v-for="(item, idx) in referencePreviews"
                :key="`${idx}-${item.url}`"
                class="reference-thumb"
              >
                <img
                  :src="item.url"
                  :alt="t('imageStudio.referenceImage')"
                />
                <button
                  type="button"
                  :title="t('imageStudio.removeReference')"
                  :aria-label="t('imageStudio.removeReference')"
                  @click="removeReference(idx)"
                >
                  <Icon name="x" size="xs" :stroke-width="2.5" />
                </button>
              </div>
            </div>
            <p v-if="referenceError" class="mt-2 text-xs text-red-500">{{ referenceError }}</p>
          </div>
        </section>

        <section class="prompt-panel">
          <div class="workbench-section-heading compact">
            <div>
              <span class="section-eyebrow">{{ t('imageStudio.stepPrompt') }}</span>
              <h3>{{ t('imageStudio.promptTitle') }}</h3>
            </div>
          </div>
          <textarea
            ref="promptRef"
            v-model="prompt"
            rows="3"
            :disabled="disabled"
            class="composer-prompt"
            :placeholder="t('imageStudio.promptPlaceholder')"
            @input="autoGrow"
            @paste="onPaste"
            @keydown.ctrl.enter.prevent="submit"
            @keydown.meta.enter.prevent="submit"
          ></textarea>
        </section>
      </div>

      <div class="action-bar">
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
          <template v-else>
            <Icon name="arrowUp" size="sm" :stroke-width="2" />
            <span>{{ t('imageStudio.sendAria') }}</span>
          </template>
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

type ComposerMode = 'generate' | 'edit' | 'compose'

export interface ComposerSubmitPayload {
  group_id: number
  mode: ComposerMode
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

const imageGroups = computed(() =>
  props.groups.filter((g) => g.allow_image_generation && g.status === 'active')
)

const promptRef = ref<HTMLTextAreaElement | null>(null)
const prompt = ref('')
const groupId = ref<number | null>(null)
const model = ref<ModelId>('gpt-image-2')
const mode = ref<ComposerMode>('generate')
const quality = ref('auto')
const n = ref(1)
const width = ref(1024)
const height = ref(1024)
const sizeAuto = ref(false)
const settingsOpen = ref(false)

const balance = computed(() => props.balance ?? 0)

const groupOptions = computed(() =>
  imageGroups.value.map((g) => ({ value: g.id, label: g.name }))
)

const modelSelectOptions = computed(() => [
  {
    value: '__image_core',
    label: t('imageStudio.modelGroupImage'),
    kind: 'group',
    disabled: true,
  },
  ...MODEL_OPTIONS.filter((option) =>
    option.value === 'gpt-image-2' || option.value === 'codex-gpt-image-2'
  ),
  {
    value: '__routing',
    label: t('imageStudio.modelGroupRouting'),
    kind: 'group',
    disabled: true,
  },
  ...MODEL_OPTIONS.filter((option) => option.value === 'auto'),
  {
    value: '__gpt5',
    label: t('imageStudio.modelGroupGpt5'),
    kind: 'group',
    disabled: true,
  },
  ...MODEL_OPTIONS.filter((option) => option.value.startsWith('gpt-5')),
])

const aspectPresets = ASPECT_PRESETS
const modeOptions = computed(() => [
  { value: 'generate' as const, label: t('imageStudio.modeGenerate') },
  { value: 'edit' as const, label: t('imageStudio.modeEdit') },
  { value: 'compose' as const, label: t('imageStudio.modeCompose') },
])

const activeModeLabel = computed(() => {
  if (mode.value === 'edit') return t('imageStudio.modeEdit')
  if (mode.value === 'compose') return t('imageStudio.modeCompose')
  return t('imageStudio.modeGenerate')
})

function modeHintFor(value: ComposerMode): string {
  if (value === 'edit') return t('imageStudio.modeEditHint')
  if (value === 'compose') return t('imageStudio.modeComposeHint')
  return t('imageStudio.modeGenerateHint')
}

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

function isModelId(value: unknown): value is ModelId {
  return MODEL_OPTIONS.some((option) => option.value === value)
}

function selectModelValue(value: string | number | boolean | null) {
  if (disabled.value || !isModelId(value)) return
  model.value = value
}

function selectCount(next: number) {
  if (disabled.value) return
  n.value = next
}

const submitSize = computed(() =>
  sizeAuto.value ? 'auto' : formatSize(width.value, height.value)
)

const activeQualityLabel = computed(
  () => qualityOptions.value.find((option) => option.value === quality.value)?.label ?? quality.value
)

const activeAspectLabel = computed(() => {
  if (sizeAuto.value) return t('imageStudio.aspectAuto')
  const preset = aspectPresets.find((item) => item.size === submitSize.value)
  return preset?.label ?? submitSize.value
})

const settingsSummary = computed(
  () =>
    `${t('imageStudio.countShort', { count: n.value })} · ${activeQualityLabel.value} · ${activeAspectLabel.value}`
)

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

watch(
  imageGroups,
  (list) => {
    if (groupId.value === null && list.length > 0) {
      groupId.value = list[0].id
    } else if (groupId.value !== null && !list.some((g) => g.id === groupId.value)) {
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

const costEstimate = computed(() =>
  estimateCost(model.value, submitSize.value, quality.value, n.value)
)

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

function addReferences(files: File[]): number {
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
    return 0
  }
  const capacity = Math.max(0, MAX_REFERENCE_IMAGES - referenceImages.value.length)
  const accepted = nextFiles.slice(0, capacity)
  if (accepted.length < nextFiles.length) {
    referenceError.value = t('imageStudio.tooManyReferences', { count: MAX_REFERENCE_IMAGES })
  }
  if (accepted.length === 0) {
    return 0
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
  return accepted.length
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

function resetReference() {
  revokeReferenceUrls()
  referenceImages.value = []
  referenceError.value = ''
  if (mode.value !== 'generate') {
    mode.value = 'generate'
  }
}

function loadReferenceFiles(files: File[], nextMode: ComposerMode = 'edit') {
  resetReference()
  const accepted = addReferences(files)
  if (accepted > 0) {
    mode.value = nextMode
    nextTick(() => promptRef.value?.focus())
  }
}

function focusPrompt() {
  nextTick(() => promptRef.value?.focus())
}

function autoGrow() {
  const el = promptRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 260)}px`
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

function resetPrompt() {
  prompt.value = ''
  nextTick(autoGrow)
}

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

defineExpose({ resetPrompt, fillPrompt, resetReference, loadReferenceFiles, focusPrompt })
</script>

<style scoped>
.composer {
  @apply flex h-full min-h-0 flex-col rounded-2xl border border-gray-100 bg-white shadow-sm dark:border-dark-700/50 dark:bg-dark-800/80;
}

.studio-panel-header {
  @apply flex-shrink-0 border-b border-gray-100 px-4 py-4 dark:border-dark-700/60;
}

.studio-kicker {
  @apply text-[11px] font-semibold uppercase tracking-normal text-primary-600 dark:text-primary-300;
}

.studio-title {
  @apply mt-1 truncate text-lg font-semibold text-gray-900 dark:text-white;
}

.studio-subtitle {
  @apply mt-1 text-xs leading-relaxed text-gray-500 dark:text-dark-300;
}

.studio-mode-summary {
  @apply mt-3 flex flex-wrap gap-1.5;
}

.studio-mode-summary span {
  @apply rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300;
}

.balance-pill {
  @apply inline-flex flex-shrink-0 items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1.5 text-xs text-gray-600;
  @apply dark:bg-dark-700 dark:text-gray-300;
}

.composer-shell {
  @apply relative flex min-h-0 flex-1 flex-col;
}

.composer-main {
  @apply min-h-0 flex-1 space-y-3 overflow-y-auto px-3 py-3;
}

.workbench-panel,
.prompt-panel {
  @apply rounded-xl border border-gray-100 bg-gray-50/90 p-3;
  @apply dark:border-dark-700/70 dark:bg-dark-900/60;
}

.compact-grid {
  @apply grid gap-3;
  grid-template-columns: minmax(0, 1fr);
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

.mode-card-grid {
  @apply mt-3 grid grid-cols-3 gap-1.5;
}

.mode-card {
  @apply min-w-0 rounded-lg border border-gray-200 bg-white px-2.5 py-2 text-left transition-colors;
  @apply hover:border-primary-300 hover:bg-primary-50;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700 dark:hover:bg-primary-900/20;
}

.mode-card-active {
  @apply border-primary-500 bg-primary-50 ring-1 ring-primary-500/30 dark:border-primary-500 dark:bg-primary-900/30;
}

.mode-card:disabled {
  @apply cursor-not-allowed opacity-50;
}

.mode-card-title {
  @apply block truncate text-sm font-semibold text-gray-900 dark:text-white;
}

.mode-card-copy {
  @apply mt-1 block line-clamp-2 text-[11px] leading-snug text-gray-500 dark:text-dark-300;
}

.settings-toggle {
  @apply mt-3 flex w-full items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-left text-xs font-semibold text-gray-700 transition-colors;
  @apply hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

.settings-summary {
  @apply min-w-0 flex-1 truncate text-right font-medium text-gray-400 dark:text-dark-400;
}

.settings-chevron {
  @apply flex-shrink-0 text-gray-400 transition-transform duration-200 dark:text-dark-400;
}

.advanced-settings {
  @apply mt-3 space-y-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800/70;
}

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

.segmented-btn:disabled,
.aspect-btn:disabled,
.count-chip:disabled {
  @apply cursor-not-allowed opacity-50;
}

.count-chip-grid {
  @apply grid grid-cols-5 gap-1.5;
}

.count-chip {
  @apply flex h-8 min-w-0 items-center justify-center rounded-lg border border-gray-200 bg-white text-xs font-semibold text-gray-600 transition-colors;
  @apply hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300;
  @apply dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

.count-chip-active {
  @apply border-primary-500 bg-primary-600 text-white shadow-sm;
  @apply hover:bg-primary-600 hover:text-white;
  @apply dark:border-primary-500 dark:bg-primary-600 dark:text-white;
}

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

.auto-toggle {
  @apply flex cursor-pointer select-none items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400;
}

.auto-checkbox {
  @apply h-3.5 w-3.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-900;
}

.size-input {
  @apply w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900;
  @apply focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30;
  @apply dark:border-dark-600 dark:bg-dark-900 dark:text-white;
  @apply disabled:cursor-not-allowed disabled:opacity-50;
}

.reference-workbench {
  @apply mt-3 rounded-lg border border-dashed border-gray-200 bg-white/75 p-3 dark:border-dark-600 dark:bg-dark-800/60;
}

.reference-workbench-header {
  @apply mb-2 flex items-start justify-between gap-3;
}

.reference-workbench-header h3,
.workbench-section-heading h3 {
  @apply text-sm font-semibold text-gray-900 dark:text-white;
}

.section-eyebrow {
  @apply block text-[10px] font-semibold uppercase tracking-normal text-primary-600 dark:text-primary-300;
}

.mode-status-pill {
  @apply flex-shrink-0 rounded-md bg-amber-50 px-2 py-1 text-[11px] font-semibold text-amber-700 dark:bg-amber-900/20 dark:text-amber-300;
}

.mode-status-ok {
  @apply bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300;
}

.reference-dropzone {
  @apply flex w-full items-center gap-2 rounded-lg border border-dashed border-primary-300 bg-primary-50/70 px-3 py-2.5 text-left text-sm text-primary-700 transition-colors;
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

.reference-counter {
  @apply flex-shrink-0 rounded-md bg-white px-2 py-1 text-[11px] font-semibold text-gray-600 dark:bg-dark-700 dark:text-gray-300;
}

.reference-grid {
  @apply mt-3 grid grid-cols-4 gap-2;
}

.reference-thumb {
  @apply relative overflow-hidden rounded-lg border border-gray-200 bg-gray-100 dark:border-dark-600 dark:bg-dark-700;
  aspect-ratio: 1 / 1;
}

.reference-thumb img {
  @apply h-full w-full object-cover;
}

.reference-thumb button {
  @apply absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white transition-colors hover:bg-black/80;
}

.workbench-section-heading {
  @apply mb-2 flex items-start justify-between gap-3;
}

.workbench-section-heading.compact {
  @apply mb-1;
}

.composer-prompt {
  @apply w-full resize-none border-0 bg-transparent px-0 pb-0 pt-2 text-sm leading-relaxed;
  @apply text-gray-900 dark:text-white;
  @apply placeholder:text-gray-400 dark:placeholder:text-dark-500;
  @apply focus:outline-none focus:ring-0;
  min-height: 112px;
}

.action-bar {
  @apply flex flex-shrink-0 flex-wrap items-center gap-2 border-t border-gray-100 px-3 py-3 dark:border-dark-700/60;
}

.upload-pill {
  @apply inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1.5 text-xs text-gray-700 transition-colors;
  @apply hover:bg-gray-200;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600;
}

.upload-pill:disabled {
  @apply cursor-not-allowed opacity-50;
}

.send-button {
  @apply ml-auto flex h-10 flex-shrink-0 items-center justify-center gap-2 rounded-full px-4 text-sm font-semibold transition-colors;
  @apply bg-primary-600 text-white hover:bg-primary-700;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40;
}

.send-button:disabled {
  @apply cursor-not-allowed opacity-40;
  @apply hover:bg-primary-600;
}

.fold-enter-active,
.fold-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.fold-enter-from,
.fold-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
