<template>
  <div
    ref="containerRef"
    class="group relative overflow-hidden rounded-xl bg-gray-100 ring-1 ring-black/5 dark:bg-dark-700/60 dark:ring-white/10"
    :style="containerStyle"
  >
    <!-- Loading -->
    <div
      v-if="!visible || loading"
      class="absolute inset-0 flex flex-col items-center justify-center gap-2"
    >
      <div
        class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
      ></div>
      <span class="px-2 text-center text-xs text-gray-400 dark:text-dark-400">
        {{ t('imageStudio.imageLoading') }}
      </span>
    </div>

    <!-- Error -->
    <div
      v-else-if="error && !displaySrc"
      class="absolute inset-0 flex flex-col items-center justify-center gap-1 px-2 text-center"
    >
      <Icon name="exclamationTriangle" size="md" class="text-gray-400 dark:text-dark-500" />
      <span class="text-xs text-gray-400 dark:text-dark-500">{{
        t('imageStudio.imageLoadFailed')
      }}</span>
    </div>

    <!-- Image -->
    <img
      v-else-if="displaySrc"
      :src="displaySrc"
      :alt="alt"
      loading="lazy"
      class="h-full w-full cursor-zoom-in transition-transform duration-200 group-hover:scale-[1.02]"
      :class="aspectRatio ? 'object-contain' : 'object-cover'"
      @error="markImageError"
      @click="$emit('open', displaySrc)"
    />

    <div
      v-if="error && displaySrc"
      class="pointer-events-none absolute bottom-2 left-2 rounded-md bg-black/60 px-2 py-1 text-[11px] font-medium text-white"
    >
      {{ t('imageStudio.cachedUrlFallback') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { toRef, computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthedImage } from '@/composables/useAuthedImage'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  url: string
  alt?: string
  /**
   * Aspect ratio (width / height) for the frame so portrait/landscape images
   * render at their true shape. When omitted the frame falls back to a square.
   */
  aspectRatio?: number
}>()

defineEmits<{
  (e: 'open', src: string): void
}>()

const { t } = useI18n()

// Reactively follow `url` so the image re-fetches if the prop changes.
const containerRef = ref<HTMLElement | null>(null)
const visible = ref(typeof window === 'undefined' || !('IntersectionObserver' in window))
const { src, fallbackSrc, loading, error } = useAuthedImage(toRef(props, 'url'), visible)

let observer: IntersectionObserver | null = null

onMounted(() => {
  if (!('IntersectionObserver' in window)) {
    visible.value = true
    return
  }
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        visible.value = true
        observer?.disconnect()
        observer = null
      }
    },
    { rootMargin: '500px 0px' }
  )
  if (containerRef.value) {
    observer.observe(containerRef.value)
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})

function markImageError() {
  if (src.value && fallbackSrc.value && src.value !== fallbackSrc.value) {
    src.value = fallbackSrc.value
    error.value = null
    return
  }
  src.value = undefined
  fallbackSrc.value = undefined
  error.value = new Error('Image failed to render')
}

const containerStyle = computed(() => ({
  aspectRatio:
    props.aspectRatio && props.aspectRatio > 0 ? String(props.aspectRatio) : '1 / 1',
}))

const displaySrc = computed(() => src.value || fallbackSrc.value)
</script>
