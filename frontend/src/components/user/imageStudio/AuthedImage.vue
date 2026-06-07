<template>
  <div
    class="group relative aspect-square overflow-hidden rounded-xl bg-gray-100 ring-1 ring-black/5 dark:bg-dark-700 dark:ring-white/10"
  >
    <!-- Loading -->
    <div
      v-if="loading"
      class="absolute inset-0 flex items-center justify-center"
    >
      <div
        class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
      ></div>
    </div>

    <!-- Error -->
    <div
      v-else-if="error"
      class="absolute inset-0 flex flex-col items-center justify-center gap-1 px-2 text-center"
    >
      <Icon name="exclamationTriangle" size="md" class="text-gray-400 dark:text-dark-500" />
      <span class="text-xs text-gray-400 dark:text-dark-500">{{
        t('imageStudio.imageLoadFailed')
      }}</span>
    </div>

    <!-- Image -->
    <img
      v-else-if="src"
      :src="src"
      :alt="alt"
      loading="lazy"
      class="h-full w-full cursor-zoom-in object-cover transition-transform duration-200 group-hover:scale-[1.03]"
      @click="$emit('open', src)"
    />
  </div>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthedImage } from '@/composables/useAuthedImage'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  url: string
  alt?: string
}>()

defineEmits<{
  (e: 'open', src: string): void
}>()

const { t } = useI18n()

// Reactively follow `url` so the image re-fetches if the prop changes.
const { src, loading, error } = useAuthedImage(toRef(props, 'url'))
</script>
