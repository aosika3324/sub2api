<template>
  <!--
    Stable, always-mounted history surface.

    This wrapper keeps a constant dark background and is NEVER unmounted when the
    conversation changes — only its inner content swaps. That prevents the brief
    white/light flash that used to happen when the whole gallery subtree was torn
    down and the onboarding hero (with its heavy blur layers) was remounted.
  -->
  <div class="flex h-full min-h-0 flex-col">
    <!-- Loading skeleton (initial fetch, nothing to show yet) -->
    <div
      v-if="loading && generations.length === 0 && !generating"
      class="space-y-4 p-4"
    >
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

    <!-- Onboarding hero (first-use / empty conversation) -->
    <div
      v-else-if="generations.length === 0 && !generating"
      class="onboarding-hero relative flex flex-1 items-center justify-center overflow-hidden px-6 py-12"
    >
      <div class="relative z-10 mx-auto max-w-3xl text-center">
        <!-- Serif display hero -->
        <h2
          class="font-serif text-4xl font-normal tracking-tight text-gray-900 md:text-5xl dark:text-white"
        >
          {{ t('imageStudio.onboardingTitle') }}
        </h2>
        <p
          class="mx-auto mt-5 max-w-md text-base leading-relaxed text-gray-500 dark:text-gray-400"
        >
          {{ t('imageStudio.onboardingSubtitle') }}
        </p>

        <div class="workbench-empty-grid">
          <div v-for="item in capabilityItems" :key="item.key" class="workbench-empty-item">
            <span class="workbench-empty-icon">
              <Icon :name="item.icon" size="sm" />
            </span>
            <span class="workbench-empty-title">{{ item.title }}</span>
            <span class="workbench-empty-copy">{{ item.copy }}</span>
          </div>
        </div>

        <!-- Example prompt chips (faint ghost pills) -->
        <div class="mt-7 flex flex-wrap justify-center gap-2">
          <button
            v-for="(example, idx) in examplePrompts"
            :key="idx"
            type="button"
            class="example-chip"
            @click="$emit('useExample', example)"
          >
            <span class="truncate">{{ example }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Results gallery (oldest → newest, newest nearest the composer) -->
    <div v-else class="p-4">
      <div v-if="hasMore" class="mb-4 flex justify-center">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loadingMore"
          @click="$emit('loadMore')"
        >
          <span
            v-if="loadingMore"
            class="mr-1.5 h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"
          ></span>
          <Icon v-else name="refresh" size="sm" class="mr-1.5" />
          {{ t(loadingMore ? 'imageStudio.loadingEarlier' : 'imageStudio.loadEarlier') }}
        </button>
      </div>

      <TransitionGroup tag="div" name="reveal" class="space-y-4">
        <GenerationCard
          v-for="gen in orderedGenerations"
          :key="gen.id"
          :generation="gen"
          @retry="$emit('retry', $event)"
          @refresh="$emit('refresh', $event)"
          @delete="$emit('delete', $event)"
          @open="$emit('open', $event)"
        />

        <!-- Live generating placeholder (shimmer) at the very bottom -->
        <div
          v-if="generating"
          key="__generating__"
          class="card overflow-hidden p-0"
        >
          <div class="border-b border-gray-100 p-4 dark:border-dark-700/60">
            <p
              v-if="pendingPrompt"
              class="whitespace-pre-wrap break-words text-sm leading-relaxed text-gray-900 dark:text-white"
            >
              {{ pendingPrompt }}
            </p>
            <div class="mt-3 flex items-center gap-2">
              <span
                class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
              ></span>
              <span class="text-xs font-medium text-primary-600 dark:text-primary-400">
                {{ t('imageStudio.generating') }}
              </span>
            </div>
          </div>
          <div class="p-4">
            <div class="shimmer aspect-square w-full rounded-xl"></div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioGeneration } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import GenerationCard from './GenerationCard.vue'

const props = defineProps<{
  generations: ImageStudioGeneration[]
  loading?: boolean
  generating?: boolean
  pendingPrompt?: string
  hasMore?: boolean
  loadingMore?: boolean
}>()

defineEmits<{
  (e: 'retry', generation: ImageStudioGeneration): void
  (e: 'refresh', generation: ImageStudioGeneration): void
  (e: 'delete', generation: ImageStudioGeneration): void
  (e: 'open', src: string): void
  (e: 'useExample', prompt: string): void
  (e: 'loadMore'): void
}>()

const { t } = useI18n()

// The store keeps generations newest-first; the chat layout wants oldest at the
// top and newest at the bottom (nearest the composer), so render a reversed view.
const orderedGenerations = computed<ImageStudioGeneration[]>(() =>
  [...props.generations].reverse()
)

const examplePrompts = computed<string[]>(() => [
  t('imageStudio.examplePrompt1'),
  t('imageStudio.examplePrompt2'),
  t('imageStudio.examplePrompt3'),
  t('imageStudio.examplePrompt4'),
])

const capabilityItems = computed(() => [
  {
    key: 'generate',
    icon: 'sparkles' as const,
    title: t('imageStudio.capabilityGenerate'),
    copy: t('imageStudio.capabilityGenerateCopy'),
  },
  {
    key: 'edit',
    icon: 'upload' as const,
    title: t('imageStudio.capabilityEdit'),
    copy: t('imageStudio.capabilityEditCopy'),
  },
  {
    key: 'history',
    icon: 'grid' as const,
    title: t('imageStudio.capabilityHistory'),
    copy: t('imageStudio.capabilityHistoryCopy'),
  },
])
</script>

<style scoped>
.example-chip {
  @apply inline-flex max-w-full items-center rounded-full border border-gray-200 bg-white/70 px-3.5 py-2 text-sm text-gray-700 transition-all;
  @apply hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 hover:shadow-sm;
  @apply dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-300;
  @apply dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

.workbench-empty-grid {
  @apply mx-auto mt-8 grid max-w-2xl grid-cols-1 gap-3 sm:grid-cols-3;
}

.workbench-empty-item {
  @apply rounded-xl border border-gray-200 bg-white/75 p-3 text-left shadow-sm;
  @apply dark:border-dark-700 dark:bg-dark-800/70;
}

.workbench-empty-icon {
  @apply inline-flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 text-primary-600;
  @apply dark:bg-primary-900/30 dark:text-primary-300;
}

.workbench-empty-title {
  @apply mt-3 block text-sm font-semibold text-gray-900 dark:text-white;
}

.workbench-empty-copy {
  @apply mt-1 block text-xs leading-relaxed text-gray-500 dark:text-dark-300;
}

/*
  Reveal animation for newly added results.

  Only the ENTER of a new item is animated. We deliberately do NOT animate the
  leave with `position: absolute`, because collapsing an item out of flow during
  a conversation switch (which clears the whole list at once) caused a layout
  jump / flash. Letting removed items disappear immediately keeps the swap clean.
*/
.reveal-enter-active {
  transition: opacity 0.45s ease, transform 0.45s cubic-bezier(0.16, 1, 0.3, 1);
}
.reveal-enter-from {
  opacity: 0;
  transform: translateY(12px) scale(0.985);
}
.reveal-move {
  transition: transform 0.45s cubic-bezier(0.16, 1, 0.3, 1);
}

/* Shimmer placeholder while a generation is in flight. */
.shimmer {
  background: linear-gradient(
    100deg,
    rgba(148, 163, 184, 0.12) 30%,
    rgba(148, 163, 184, 0.22) 50%,
    rgba(148, 163, 184, 0.12) 70%
  );
  background-size: 200% 100%;
  animation: shimmer 1.4s ease-in-out infinite;
}
@keyframes shimmer {
  0% {
    background-position: 180% 0;
  }
  100% {
    background-position: -180% 0;
  }
}
</style>
