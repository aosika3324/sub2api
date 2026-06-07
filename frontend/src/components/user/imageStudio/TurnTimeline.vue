<template>
  <div class="flex h-full flex-col">
    <!-- Loading skeleton (initial fetch) -->
    <div v-if="loading && generations.length === 0 && !generating" class="space-y-4">
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

    <!-- Onboarding hero (first-use / empty) -->
    <div
      v-else-if="generations.length === 0 && !generating"
      class="onboarding-hero relative flex flex-1 items-center justify-center overflow-hidden rounded-2xl border border-gray-100 px-6 py-16 dark:border-dark-700/50"
    >
      <!-- Atmosphere -->
      <div class="pointer-events-none absolute inset-0 opacity-80" aria-hidden="true">
        <div
          class="absolute -left-16 -top-16 h-64 w-64 rounded-full bg-primary-400/20 blur-3xl dark:bg-primary-500/15"
        ></div>
        <div
          class="absolute -bottom-20 right-0 h-72 w-72 rounded-full bg-fuchsia-400/10 blur-3xl dark:bg-fuchsia-500/10"
        ></div>
      </div>

      <div class="relative z-10 mx-auto max-w-xl text-center">
        <div
          class="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 text-white shadow-lg shadow-primary-500/30"
        >
          <Icon name="sparkles" size="xl" />
        </div>
        <h2 class="text-xl font-bold text-gray-900 dark:text-white">
          {{ t('imageStudio.onboardingTitle') }}
        </h2>
        <p class="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-500 dark:text-gray-400">
          {{ t('imageStudio.onboardingSubtitle') }}
        </p>

        <!-- Example prompt chips -->
        <div class="mt-7">
          <p class="mb-3 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">
            {{ t('imageStudio.tryExample') }}
          </p>
          <div class="flex flex-wrap justify-center gap-2">
            <button
              v-for="(example, idx) in examplePrompts"
              :key="idx"
              type="button"
              class="example-chip"
              @click="$emit('useExample', example)"
            >
              <Icon name="lightbulb" size="xs" class="mr-1.5 flex-shrink-0 opacity-60" />
              <span class="truncate">{{ example }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Results gallery -->
    <TransitionGroup v-else tag="div" name="reveal" class="space-y-4">
      <!-- Live generating placeholder (shimmer) at the top -->
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

      <GenerationCard
        v-for="gen in generations"
        :key="gen.id"
        :generation="gen"
        @retry="$emit('retry', $event)"
        @delete="$emit('delete', $event)"
        @open="$emit('open', $event)"
      />
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioGeneration } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import GenerationCard from './GenerationCard.vue'

defineProps<{
  generations: ImageStudioGeneration[]
  loading?: boolean
  generating?: boolean
  pendingPrompt?: string
}>()

defineEmits<{
  (e: 'retry', generation: ImageStudioGeneration): void
  (e: 'delete', generation: ImageStudioGeneration): void
  (e: 'open', src: string): void
  (e: 'useExample', prompt: string): void
}>()

const { t } = useI18n()

const examplePrompts = computed<string[]>(() => [
  t('imageStudio.examplePrompt1'),
  t('imageStudio.examplePrompt2'),
  t('imageStudio.examplePrompt3'),
  t('imageStudio.examplePrompt4'),
])
</script>

<style scoped>
.example-chip {
  @apply inline-flex max-w-full items-center rounded-full border border-gray-200 bg-white/70 px-3.5 py-2 text-sm text-gray-700 transition-all;
  @apply hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 hover:shadow-sm;
  @apply dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-300;
  @apply dark:hover:border-primary-700 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

/* Reveal animation for newly added results. */
.reveal-enter-active {
  transition: opacity 0.45s ease, transform 0.45s cubic-bezier(0.16, 1, 0.3, 1);
}
.reveal-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
  position: absolute;
  width: 100%;
}
.reveal-enter-from {
  opacity: 0;
  transform: translateY(-12px) scale(0.985);
}
.reveal-leave-to {
  opacity: 0;
  transform: scale(0.98);
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
