<template>
  <div class="video-card" :class="`video-card--${gen.status}`">
    <!-- Media / status area -->
    <div class="video-card-media">
      <!-- Succeeded: lazy authed <video>, fetched only when the user clicks play -->
      <template v-if="gen.status === 'succeeded'">
        <div v-if="!activated" class="video-card-poster" @click="activate">
          <button type="button" class="video-card-play" :aria-label="t('videoStudio.play')">
            <Icon name="play" size="lg" />
          </button>
          <p class="video-card-poster-meta">
            {{ formatDuration(gen.duration_seconds) }}
          </p>
        </div>
        <template v-else>
          <div v-if="videoLoading" class="video-card-loading">
            <Icon name="sync" size="md" class="animate-spin" />
            <span>{{ t('videoStudio.loadingVideo') }}</span>
          </div>
          <div v-else-if="videoError" class="video-card-expired">
            <Icon name="exclamationTriangle" size="md" />
            <span>{{ t('videoStudio.videoExpired') }}</span>
          </div>
          <video
            v-else-if="videoSrc"
            :src="videoSrc"
            class="video-card-player"
            controls
            autoplay
            playsinline
          />
        </template>
      </template>

      <!-- Pending / processing: animated placeholder -->
      <div v-else-if="isInFlight" class="video-card-progress">
        <Icon name="sync" size="lg" class="animate-spin" />
        <p class="video-card-progress-label">{{ t('videoStudio.statusProcessing') }}</p>
        <p class="video-card-progress-hint">{{ t('videoStudio.processingHint') }}</p>
      </div>

      <!-- Failed -->
      <div v-else class="video-card-failed">
        <Icon name="exclamationTriangle" size="lg" />
        <p class="video-card-failed-label">{{ failureMessage }}</p>
      </div>
    </div>

    <!-- Footer: prompt + meta + actions -->
    <div class="video-card-body">
      <p class="video-card-prompt" :title="gen.prompt">{{ gen.prompt }}</p>
      <div class="video-card-meta">
        <span class="video-card-model">{{ gen.model }}</span>
        <span v-if="gen.status === 'succeeded' && gen.cost > 0" class="video-card-cost">
          {{ formatCost(gen.cost) }}
        </span>
      </div>
      <div class="video-card-actions">
        <button
          v-if="gen.status === 'succeeded' && firstVideoUrl"
          type="button"
          class="video-card-action"
          @click="$emit('download', gen)"
        >
          <Icon name="download" size="xs" />
          <span>{{ t('videoStudio.download') }}</span>
        </button>
        <button
          v-if="gen.status === 'failed'"
          type="button"
          class="video-card-action"
          @click="$emit('retry', gen)"
        >
          <Icon name="refresh" size="xs" />
          <span>{{ t('videoStudio.retry') }}</span>
        </button>
        <button
          type="button"
          class="video-card-action video-card-action--danger"
          @click="$emit('delete', gen)"
        >
          <Icon name="trash" size="xs" />
          <span>{{ t('common.delete') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAuthedVideo } from '@/composables/useAuthedVideo'
import type { VideoStudioGeneration } from '@/types'

const props = defineProps<{
  gen: VideoStudioGeneration
}>()

defineEmits<{
  (e: 'download', gen: VideoStudioGeneration): void
  (e: 'retry', gen: VideoStudioGeneration): void
  (e: 'delete', gen: VideoStudioGeneration): void
}>()

const { t } = useI18n()

// The video bytes are only fetched after the user clicks play (lazy/opt-in), so
// a long history list never eagerly downloads every clip.
const activated = ref(false)
const firstVideoUrl = computed(() => props.gen.videos?.[0] ?? '')
const activeUrl = computed(() => (activated.value ? firstVideoUrl.value : ''))
const { src: videoSrc, loading: videoLoading, error: videoError } = useAuthedVideo(
  activeUrl,
  computed(() => activated.value)
)

const isInFlight = computed(
  () => props.gen.status === 'pending' || props.gen.status === 'processing'
)

const failureMessage = computed(() => {
  const code = props.gen.error_code
  if (code) {
    const key = `videoStudio.error.${code}`
    const translated = t(key)
    if (translated !== key) return translated
  }
  return t('videoStudio.error.generic')
})

function activate() {
  if (firstVideoUrl.value) activated.value = true
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return ''
  return `${seconds.toFixed(0)}s`
}

function formatCost(cost: number): string {
  return `$${cost.toFixed(4)}`
}
</script>

<style scoped>
.video-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 0.875rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.08));
  background: var(--ui-surface, #fff);
}

.video-card-media {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 16 / 9;
  background: var(--ui-surface-sunken, #0b0f14);
  color: rgba(255, 255, 255, 0.85);
}

.video-card-player {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #000;
}

.video-card-poster {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  width: 100%;
  height: 100%;
  cursor: pointer;
}

.video-card-play {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3.5rem;
  height: 3.5rem;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.15);
  color: #fff;
  transition: background 0.15s ease;
}
.video-card-play:hover {
  background: rgba(255, 255, 255, 0.28);
}

.video-card-poster-meta {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.7);
}

.video-card-loading,
.video-card-expired,
.video-card-progress,
.video-card-failed {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 1rem;
  text-align: center;
  font-size: 0.8125rem;
}

.video-card-progress-label {
  font-weight: 600;
}
.video-card-progress-hint,
.video-card-poster-meta {
  font-size: 0.75rem;
  opacity: 0.7;
}

.video-card-failed {
  color: #fca5a5;
}

.video-card-body {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem;
}

.video-card-prompt {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 0.8125rem;
  line-height: 1.4;
  color: var(--ui-text, #1f2937);
}

.video-card-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.6875rem;
  color: var(--ui-text-muted, #6b7280);
}

.video-card-cost {
  font-variant-numeric: tabular-nums;
}

.video-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.video-card-action {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.5rem;
  border-radius: 0.5rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.08));
  font-size: 0.6875rem;
  color: var(--ui-text-muted, #6b7280);
  transition: background 0.15s ease;
}
.video-card-action:hover {
  background: var(--ui-surface-hover, rgba(0, 0, 0, 0.04));
}
.video-card-action--danger:hover {
  color: #ef4444;
}
</style>
