<template>
  <div class="flex h-full flex-col">
    <!-- Loading skeleton -->
    <div v-if="loading && generations.length === 0" class="space-y-4">
      <div
        v-for="i in 2"
        :key="i"
        class="card animate-pulse space-y-4 p-4"
      >
        <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
        <div class="flex gap-2">
          <div class="h-5 w-16 rounded bg-gray-200 dark:bg-dark-700"></div>
          <div class="h-5 w-12 rounded bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="aspect-square w-1/2 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="generations.length === 0"
      class="flex flex-1 items-center justify-center py-16"
    >
      <EmptyState
        :title="emptyTitle"
        :description="emptyDescription"
      >
        <template #icon>
          <Icon name="sparkles" class="h-10 w-10 text-gray-400 dark:text-dark-500" />
        </template>
      </EmptyState>
    </div>

    <!-- Turns -->
    <div v-else class="space-y-4">
      <GenerationCard
        v-for="gen in generations"
        :key="gen.id"
        :generation="gen"
        @retry="$emit('retry', $event)"
        @delete="$emit('delete', $event)"
        @open="$emit('open', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioGeneration } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GenerationCard from './GenerationCard.vue'

const props = defineProps<{
  generations: ImageStudioGeneration[]
  loading?: boolean
  hasActiveConversation?: boolean
}>()

defineEmits<{
  (e: 'retry', generation: ImageStudioGeneration): void
  (e: 'delete', generation: ImageStudioGeneration): void
  (e: 'open', src: string): void
}>()

const { t } = useI18n()

const emptyTitle = computed(() =>
  props.hasActiveConversation
    ? t('imageStudio.emptyTurnsTitle')
    : t('imageStudio.emptyGalleryTitle')
)
const emptyDescription = computed(() =>
  props.hasActiveConversation
    ? t('imageStudio.emptyTurnsDescription')
    : t('imageStudio.emptyGalleryDescription')
)
</script>
