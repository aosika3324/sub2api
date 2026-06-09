<template>
  <div class="generation-card card overflow-hidden p-0">
    <!-- Header: prompt + params chips -->
    <div class="border-b border-gray-100 p-4 dark:border-dark-700/60">
      <div class="flex items-start justify-between gap-3">
        <p
          class="flex-1 whitespace-pre-wrap break-words text-sm leading-relaxed text-gray-900 dark:text-white"
        >
          {{ generation.prompt }}
        </p>
        <button
          v-if="!hideDelete"
          type="button"
          class="-mr-1 -mt-1 flex-shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
          :title="t('common.delete')"
          @click="$emit('delete', generation)"
        >
          <Icon name="trash" size="sm" />
        </button>
      </div>

      <!-- Params chips -->
      <div class="mt-3 flex flex-wrap items-center gap-1.5">
        <span class="chip">
          <Icon name="sparkles" size="xs" class="mr-1 opacity-70" />
          {{ generation.model }}
        </span>
        <span class="chip chip-mode">{{ modeLabel }}</span>
        <span v-if="inputImages.length > 0" class="chip chip-i2i">{{
          t('imageStudio.imageToImage')
        }}</span>
        <span class="chip">{{ generation.size }}</span>
        <span v-if="generation.quality" class="chip">{{
          t('imageStudio.qualityChip', { quality: generation.quality })
        }}</span>
        <span class="chip">{{ t('imageStudio.countChip', { count: generation.n }) }}</span>
        <span
          v-if="isSucceeded && generation.cost != null"
          class="chip chip-cost"
          :title="t('imageStudio.cost')"
        >
          ${{ generation.cost.toFixed(4) }}
        </span>
        <span class="ml-auto text-xs text-gray-400 dark:text-dark-500">{{
          formattedTime
        }}</span>
      </div>
    </div>

    <!-- Body -->
    <div class="generation-body">
      <!-- Source / reference images (image-to-image) -->
      <div v-if="inputImages.length > 0" class="mb-3">
        <p class="mb-1.5 text-xs font-medium text-gray-400 dark:text-dark-500">
          {{ t('imageStudio.sourceImage') }}
        </p>
        <div class="flex flex-wrap gap-2">
          <AuthedImage
            v-for="(url, idx) in inputImages"
            :key="`src-${generation.id}-${idx}`"
            :url="url"
            :alt="t('imageStudio.sourceImage')"
            class="source-thumb"
            @open="$emit('open', $event)"
          />
        </div>
      </div>

      <!-- Pending / generating -->
      <div
        v-if="isPending"
        class="pending-state"
      >
        <div
          class="h-8 w-8 animate-spin rounded-full border-[3px] border-primary-500 border-t-transparent"
        ></div>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('imageStudio.generating') }}
        </p>
        <p class="text-xs font-medium text-primary-600 dark:text-primary-300">
          {{ t('imageStudio.waitingElapsed', { elapsed: elapsedLabel }) }}
        </p>
        <p class="max-w-sm text-center text-xs text-gray-400 dark:text-dark-500">
          {{ t('imageStudio.continueWaitingHint') }}
        </p>
        <button type="button" class="btn btn-secondary" @click="$emit('refresh', generation)">
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ t('imageStudio.refreshStatus') }}
        </button>
      </div>

      <!-- Failed -->
      <div
        v-else-if="isFailed"
        class="flex flex-col items-center justify-center gap-3 rounded-xl border border-red-200 bg-red-50 py-8 px-4 text-center dark:border-red-900/40 dark:bg-red-900/10"
      >
        <Icon name="x" size="lg" class="text-red-500" />
        <p class="text-sm text-red-600 dark:text-red-400">
          {{ t('imageStudio.generationFailed') }}
        </p>
        <p
          v-if="failureMessage"
          class="max-w-xl whitespace-pre-wrap break-words rounded-lg bg-white/70 px-3 py-2 text-left text-xs leading-relaxed text-red-700 dark:bg-red-950/20 dark:text-red-300"
        >
          {{ failureMessage }}
        </p>
        <button type="button" class="btn btn-secondary" @click="$emit('retry', generation)">
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ t('imageStudio.retry') }}
        </button>
      </div>

      <!-- Succeeded with images -->
      <div
        v-else-if="isSucceeded && images.length > 0"
        class="result-grid"
        :class="[gridColsClass, { 'result-grid-single': images.length === 1 }]"
      >
        <div
          v-for="(url, idx) in images"
          :key="`${generation.id}-${idx}`"
          class="result-item"
        >
          <AuthedImage
            :url="url"
            :alt="generation.prompt"
            :aspect-ratio="aspectRatio"
            class="result-image"
            @open="$emit('open', $event)"
          />
          <button
            type="button"
            class="quick-edit-button"
            :title="t('imageStudio.quickEdit')"
            :aria-label="t('imageStudio.quickEdit')"
            @click="$emit('edit', { generation, url })"
          >
            <Icon name="edit" size="xs" :stroke-width="2" />
            <span>{{ t('imageStudio.quickEdit') }}</span>
          </button>
          <button
            type="button"
            class="quick-reference-button"
            :title="t('imageStudio.addReference')"
            :aria-label="t('imageStudio.addReference')"
            @click="$emit('reference', { generation, url })"
          >
            <Icon name="plus" size="xs" :stroke-width="2" />
            <span>{{ t('imageStudio.addReference') }}</span>
          </button>
        </div>
      </div>

      <!-- Succeeded but no images returned -->
      <div
        v-else
        class="py-8 text-center text-sm text-gray-400 dark:text-dark-500"
      >
        {{ t('imageStudio.noImages') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioGeneration } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import AuthedImage from './AuthedImage.vue'
import { aspectRatioFromSize } from './pricing'

const props = defineProps<{
  generation: ImageStudioGeneration
  hideDelete?: boolean
}>()

defineEmits<{
  (e: 'retry', generation: ImageStudioGeneration): void
  (e: 'refresh', generation: ImageStudioGeneration): void
  (e: 'delete', generation: ImageStudioGeneration): void
  (e: 'open', src: string): void
  (e: 'edit', payload: { generation: ImageStudioGeneration; url: string }): void
  (e: 'reference', payload: { generation: ImageStudioGeneration; url: string }): void
}>()

const { t } = useI18n()

// Status helpers — treat unknown statuses as pending so a freshly-created
// turn never renders as "no images" before the backend reports completion.
const isPending = computed(
  () => props.generation.status === 'pending' || props.generation.status === 'generating'
)
const isFailed = computed(() => props.generation.status === 'failed')
const isSucceeded = computed(
  () => props.generation.status === 'succeeded' || props.generation.status === 'completed'
)

const images = computed(() => props.generation.images ?? [])
const failureMessage = computed(() => (props.generation.error ?? '').trim())

const now = ref(Date.now())
let elapsedTimer: number | null = null

function stopElapsedTimer() {
  if (elapsedTimer == null) return
  window.clearInterval(elapsedTimer)
  elapsedTimer = null
}

function syncElapsedTimer() {
  now.value = Date.now()
  if (!isPending.value) {
    stopElapsedTimer()
    return
  }
  if (elapsedTimer == null) {
    elapsedTimer = window.setInterval(() => {
      now.value = Date.now()
    }, 1000)
  }
}

watch(isPending, syncElapsedTimer, { immediate: true })
onBeforeUnmount(stopElapsedTimer)

// Source/reference images for image-to-image generations. When present we render
// a small "source" row above the output grid and flag the turn with an i2i chip.
const inputImages = computed(() => props.generation.input_images ?? [])
const generationMode = computed(() => {
  if (props.generation.mode) return props.generation.mode
  if (inputImages.value.length >= 2) return 'compose'
  if (inputImages.value.length === 1) return 'edit'
  return 'generate'
})

const modeLabel = computed(() => {
  if (generationMode.value === 'compose') return t('imageStudio.modeCompose')
  if (generationMode.value === 'edit') return t('imageStudio.modeEdit')
  return t('imageStudio.modeGenerate')
})

// Derive the true aspect ratio from the generation size so portrait/landscape
// images render uncropped. `undefined` lets AuthedImage fall back to a square.
const aspectRatio = computed(() => aspectRatioFromSize(props.generation.size) ?? undefined)

const gridColsClass = computed(() => {
  const count = images.value.length
  if (count <= 1) return 'grid-cols-1'
  if (count === 2) return 'grid-cols-2'
  return 'grid-cols-2 sm:grid-cols-3'
})

const formattedTime = computed(() => {
  if (!props.generation.created_at) return ''
  const d = new Date(props.generation.created_at)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
})

const elapsedLabel = computed(() => {
  const createdAt = props.generation.created_at
    ? new Date(props.generation.created_at).getTime()
    : NaN
  const elapsedMs = Number.isFinite(createdAt) ? Math.max(0, now.value - createdAt) : 0
  return formatElapsed(elapsedMs)
})

function formatElapsed(ms: number) {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const mm = String(minutes).padStart(2, '0')
  const ss = String(seconds).padStart(2, '0')
  if (hours > 0) {
    return `${hours}:${mm}:${ss}`
  }
  return `${mm}:${ss}`
}
</script>

<style scoped>
.chip {
  @apply inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300;
}
.chip-cost {
  @apply bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400;
}
.chip-i2i {
  @apply bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300;
}
.chip-mode {
  @apply bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300;
}

.generation-card {
  @apply w-full;
}

.generation-body {
  @apply p-4;
}

.pending-state {
  @apply flex min-h-[360px] flex-col items-center justify-center gap-3 py-10;
}

/* Small fixed-height source thumbnails for image-to-image inputs. */
.source-thumb {
  @apply h-16 w-16 flex-shrink-0;
}

.result-grid {
  @apply grid gap-3;
}

.result-grid-single {
  @apply w-full;
}

.result-item {
  @apply relative min-w-0;
}

.result-image {
  @apply w-full;
  min-height: 280px;
}

.quick-edit-button {
  @apply absolute left-3 top-3 z-10 inline-flex h-8 items-center gap-1.5 rounded-full bg-black/60 px-3 text-xs font-semibold text-white shadow-sm backdrop-blur transition-colors;
  @apply hover:bg-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/50;
}

.quick-reference-button {
  @apply absolute left-3 top-12 z-10 inline-flex h-8 items-center gap-1.5 rounded-full bg-black/55 px-3 text-xs font-semibold text-white shadow-sm backdrop-blur transition-colors;
  @apply hover:bg-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/50;
}

@media (max-width: 640px) {
  .generation-body {
    @apply p-3;
  }

  .result-image {
    min-height: 220px;
  }

  .quick-edit-button {
    @apply left-2 top-2 h-7 px-2.5;
  }

  .quick-reference-button {
    @apply left-2 top-10 h-7 px-2.5;
  }
}
</style>
