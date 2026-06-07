<template>
  <div class="card overflow-hidden p-0">
    <!-- Header: prompt + params chips -->
    <div class="border-b border-gray-100 p-4 dark:border-dark-700">
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
        <span class="chip">{{ generation.size }}</span>
        <span v-if="generation.quality" class="chip">{{
          t('imageStudio.qualityChip', { quality: generation.quality })
        }}</span>
        <span class="chip">{{ t('imageStudio.countChip', { n: generation.n }) }}</span>
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
    <div class="p-4">
      <!-- Pending / generating -->
      <div
        v-if="isPending"
        class="flex flex-col items-center justify-center gap-3 py-10"
      >
        <div
          class="h-8 w-8 animate-spin rounded-full border-[3px] border-primary-500 border-t-transparent"
        ></div>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('imageStudio.generating') }}
        </p>
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
        <button type="button" class="btn btn-secondary" @click="$emit('retry', generation)">
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ t('imageStudio.retry') }}
        </button>
      </div>

      <!-- Succeeded with images -->
      <div
        v-else-if="isSucceeded && images.length > 0"
        class="grid gap-3"
        :class="gridColsClass"
      >
        <AuthedImage
          v-for="(url, idx) in images"
          :key="`${generation.id}-${idx}`"
          :url="url"
          :alt="generation.prompt"
          @open="$emit('open', $event)"
        />
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
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioGeneration } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import AuthedImage from './AuthedImage.vue'

const props = defineProps<{
  generation: ImageStudioGeneration
  hideDelete?: boolean
}>()

defineEmits<{
  (e: 'retry', generation: ImageStudioGeneration): void
  (e: 'delete', generation: ImageStudioGeneration): void
  (e: 'open', src: string): void
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
</script>

<style scoped>
.chip {
  @apply inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300;
}
.chip-cost {
  @apply bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400;
}
</style>
