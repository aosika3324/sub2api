<template>
  <AppLayout>
    <div class="video-studio-page">
      <h1 class="sr-only">{{ t('videoStudio.title') }}</h1>

      <div class="video-studio-layout">
        <!-- Left: composer -->
        <aside class="video-studio-composer">
          <div class="composer-hero">
            <Icon name="play" size="md" class="composer-hero-icon" />
            <div class="min-w-0">
              <p class="composer-hero-title">{{ t('videoStudio.workbenchTitle') }}</p>
              <p class="composer-hero-sub">{{ t('videoStudio.workbenchSubtitle') }}</p>
            </div>
          </div>

          <!-- Group selector (only Veo-enabled groups) -->
          <label class="composer-field">
            <span class="composer-label">{{ t('videoStudio.group') }}</span>
            <select v-model="selectedGroupId" class="composer-input" :disabled="loadingGroups">
              <option v-for="g in veoGroups" :key="g.id" :value="g.id">
                {{ g.name }}
              </option>
            </select>
          </label>

          <!-- Model -->
          <label class="composer-field">
            <span class="composer-label">{{ t('videoStudio.model') }}</span>
            <select v-model="selectedModel" class="composer-input">
              <option v-for="m in modelOptions" :key="m" :value="m">{{ m }}</option>
            </select>
          </label>

          <!-- Prompt -->
          <label class="composer-field composer-field--grow">
            <span class="composer-label">{{ t('videoStudio.prompt') }}</span>
            <textarea
              v-model="prompt"
              class="composer-input composer-textarea"
              :placeholder="t('videoStudio.promptPlaceholder')"
              rows="5"
              maxlength="5000"
            />
          </label>

          <!-- Estimated cost + balance -->
          <div class="composer-estimate">
            <div class="composer-estimate-row">
              <span>{{ t('videoStudio.estimatedCost') }}</span>
              <span class="composer-estimate-value">{{ estimatedCostLabel }}</span>
            </div>
            <div class="composer-estimate-row composer-estimate-row--muted">
              <span>{{ t('videoStudio.balance') }}</span>
              <span>{{ balanceLabel }}</span>
            </div>
          </div>

          <!-- Inline error -->
          <div v-if="inlineError" class="composer-error">
            <Icon name="exclamationTriangle" size="sm" class="flex-shrink-0" />
            <span class="flex-1">{{ inlineError }}</span>
            <button type="button" @click="inlineError = ''"><Icon name="x" size="xs" /></button>
          </div>

          <button
            type="button"
            class="composer-submit"
            :disabled="!canGenerate"
            @click="handleGenerate"
          >
            <Icon v-if="store.generating" name="sync" size="sm" class="animate-spin" />
            <Icon v-else name="sparkles" size="sm" />
            <span>{{ t('videoStudio.generate') }}</span>
          </button>

          <!-- Retention notice (videos are not stored long-term) -->
          <div class="composer-notice">
            <Icon name="infoCircle" size="xs" class="flex-shrink-0" />
            <span>{{ t('videoStudio.retentionNotice') }}</span>
          </div>
        </aside>

        <!-- Right: task-card stream -->
        <section class="video-studio-canvas">
          <div class="canvas-toolbar">
            <div class="min-w-0">
              <p class="canvas-kicker">{{ t('videoStudio.myVideos') }}</p>
            </div>
            <div class="canvas-toolbar-actions">
              <button
                type="button"
                class="canvas-btn"
                :title="t('videoStudio.refresh')"
                :aria-label="t('videoStudio.refresh')"
                @click="store.refreshPending().catch(() => {})"
              >
                <Icon name="refresh" size="sm" />
              </button>
              <button
                v-if="store.generations.length > 0"
                type="button"
                class="canvas-btn"
                :title="t('videoStudio.clearHistory')"
                :aria-label="t('videoStudio.clearHistory')"
                @click="clearHistoryOpen = true"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <!-- Empty state -->
          <div v-if="store.hasLoaded && store.generations.length === 0" class="canvas-empty">
            <Icon name="play" size="lg" class="canvas-empty-icon" />
            <p class="canvas-empty-title">{{ t('videoStudio.emptyTitle') }}</p>
            <p class="canvas-empty-sub">{{ t('videoStudio.emptySubtitle') }}</p>
          </div>

          <!-- Loading skeleton (first load only) -->
          <div v-else-if="!store.hasLoaded && store.loading" class="canvas-loading">
            <Icon name="sync" size="md" class="animate-spin" />
          </div>

          <!-- Card grid -->
          <div v-else class="canvas-grid">
            <VideoCard
              v-for="gen in store.generations"
              :key="gen.id"
              :gen="gen"
              @download="handleDownload"
              @retry="handleRetry"
              @delete="confirmDelete"
            />
          </div>
        </section>
      </div>
    </div>

    <!-- Delete confirm -->
    <ConfirmDialog
      :show="deleteTarget !== null"
      :title="t('videoStudio.deleteTitle')"
      :message="t('videoStudio.deleteMessage')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="deleteTarget = null"
    />

    <!-- Clear history confirm -->
    <ConfirmDialog
      :show="clearHistoryOpen"
      :title="t('videoStudio.clearHistoryTitle')"
      :message="t('videoStudio.clearHistoryMessage')"
      :confirm-text="t('videoStudio.clearHistory')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleClearHistory"
      @cancel="clearHistoryOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useVideoStudioStore } from '@/stores/videoStudio'
import videoStudioAPI from '@/api/videoStudio'
import { useAuthStore } from '@/stores/auth'
import { userGroupsAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import VideoCard from '@/components/user/videoStudio/VideoCard.vue'
import type { Group, VideoStudioGeneration } from '@/types'

const { t } = useI18n()
const store = useVideoStudioStore()
const authStore = useAuthStore()

const DEFAULT_VEO_MODEL = 'veo-3.0-generate-001'
const modelOptions = ['veo-3.0-generate-001', 'veo-3.1-generate-preview']

const groups = ref<Group[]>([])
const loadingGroups = ref(false)
const selectedGroupId = ref<number | null>(null)
const selectedModel = ref(DEFAULT_VEO_MODEL)
const prompt = ref('')
const inlineError = ref('')
const deleteTarget = ref<VideoStudioGeneration | null>(null)
const clearHistoryOpen = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

// Only groups with a configured Veo per-second price can generate video; this
// same gate drives sidebar visibility (the entry is hidden when none qualify).
const veoGroups = computed(() =>
  groups.value.filter(
    (g) => g.veo_video_price_per_second != null && g.veo_video_price_per_second > 0
  )
)

const selectedGroup = computed(
  () => veoGroups.value.find((g) => g.id === selectedGroupId.value) ?? null
)

const balance = computed(() => authStore.user?.balance ?? 0)
const balanceLabel = computed(() => `$${balance.value.toFixed(2)}`)

// A rough estimate: per-second price × the default 8s clip (the authoritative
// charge is computed on completion from the real duration).
const estimatedCost = computed(() => {
  const price = selectedGroup.value?.veo_video_price_per_second
  if (!price || price <= 0) return 0
  const multiplier = selectedGroup.value?.rate_multiplier ?? 1
  return price * 8 * multiplier
})
const estimatedCostLabel = computed(() =>
  estimatedCost.value > 0 ? `~$${estimatedCost.value.toFixed(4)}` : '—'
)

const canGenerate = computed(
  () =>
    !store.generating &&
    !!selectedGroupId.value &&
    prompt.value.trim().length > 0
)

async function loadGroups() {
  loadingGroups.value = true
  try {
    groups.value = await userGroupsAPI.getAvailable()
    if (!selectedGroupId.value && veoGroups.value.length > 0) {
      selectedGroupId.value = veoGroups.value[0].id
    }
  } catch {
    // Non-fatal; the user simply sees no selectable group.
  } finally {
    loadingGroups.value = false
  }
}

async function handleGenerate() {
  if (!canGenerate.value || !selectedGroupId.value) return
  inlineError.value = ''
  try {
    await store.generate({
      group_id: selectedGroupId.value,
      prompt: prompt.value.trim(),
      model: selectedModel.value,
    })
    prompt.value = ''
    ensurePolling()
  } catch (err) {
    inlineError.value = extractError(err)
  }
}

async function handleDownload(gen: VideoStudioGeneration) {
  const url = gen.videos?.[0]
  if (!url) return
  try {
    const blob = await videoStudioAPI.fetchVideoBlob(url)
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = `video-${gen.id}.mp4`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(objectUrl)
  } catch {
    inlineError.value = t('videoStudio.videoExpired')
  }
}

function handleRetry(gen: VideoStudioGeneration) {
  // Re-submit the same prompt/model; the failed row stays in history.
  prompt.value = gen.prompt
  const m = modelOptions.includes(gen.model) ? gen.model : DEFAULT_VEO_MODEL
  selectedModel.value = m
  handleGenerate()
}

function confirmDelete(gen: VideoStudioGeneration) {
  deleteTarget.value = gen
}

async function handleDelete() {
  const target = deleteTarget.value
  deleteTarget.value = null
  if (!target) return
  try {
    await store.deleteGeneration(target.id)
  } catch (err) {
    inlineError.value = extractError(err)
  }
}

async function handleClearHistory() {
  clearHistoryOpen.value = false
  try {
    await store.clearHistory()
  } catch (err) {
    inlineError.value = extractError(err)
  }
}

function ensurePolling() {
  if (pollTimer) return
  pollTimer = setInterval(() => {
    if (!store.hasPending()) {
      stopPolling()
      return
    }
    store.refreshPending().catch(() => {})
  }, 5000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function extractError(err: unknown): string {
  const e = err as { response?: { data?: { message?: string } }; message?: string }
  return e?.response?.data?.message || e?.message || t('videoStudio.error.generic')
}

watch(
  () => store.generations.map((g) => g.status).join(','),
  () => {
    if (store.hasPending()) ensurePolling()
    else stopPolling()
  }
)

onMounted(async () => {
  await Promise.all([loadGroups(), store.loadGenerations().catch(() => {})])
  if (store.hasPending()) ensurePolling()
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<style scoped>
.video-studio-page {
  height: 100%;
  padding: 1rem;
}

.video-studio-layout {
  display: grid;
  grid-template-columns: minmax(280px, 340px) 1fr;
  gap: 1rem;
  height: 100%;
}

@media (max-width: 900px) {
  .video-studio-layout {
    grid-template-columns: 1fr;
  }
}

.video-studio-composer {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.08));
  background: var(--ui-surface, #fff);
  overflow-y: auto;
}

.composer-hero {
  display: flex;
  align-items: center;
  gap: 0.625rem;
}
.composer-hero-icon {
  color: var(--ui-accent, #f97316);
}
.composer-hero-title {
  font-weight: 600;
  font-size: 0.9375rem;
}
.composer-hero-sub {
  font-size: 0.75rem;
  color: var(--ui-text-muted, #6b7280);
}

.composer-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.composer-field--grow {
  flex: 1;
}
.composer-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--ui-text-muted, #6b7280);
}
.composer-input {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border-radius: 0.625rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.12));
  background: var(--ui-surface-sunken, #f9fafb);
  font-size: 0.8125rem;
  color: var(--ui-text, #1f2937);
}
.composer-textarea {
  resize: vertical;
  min-height: 6rem;
  font-family: inherit;
}

.composer-estimate {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.625rem 0.75rem;
  border-radius: 0.625rem;
  background: var(--ui-surface-sunken, #f9fafb);
}
.composer-estimate-row {
  display: flex;
  justify-content: space-between;
  font-size: 0.8125rem;
}
.composer-estimate-row--muted {
  color: var(--ui-text-muted, #6b7280);
  font-size: 0.75rem;
}
.composer-estimate-value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.composer-error {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.5rem 0.625rem;
  border-radius: 0.625rem;
  background: rgba(239, 68, 68, 0.1);
  color: #b91c1c;
  font-size: 0.75rem;
}

.composer-submit {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.625rem;
  border-radius: 0.75rem;
  background: var(--ui-accent, #f97316);
  color: #fff;
  font-weight: 600;
  font-size: 0.875rem;
  transition: opacity 0.15s ease;
}
.composer-submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.composer-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.375rem;
  font-size: 0.6875rem;
  color: var(--ui-text-muted, #6b7280);
  line-height: 1.4;
}

.video-studio-canvas {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.08));
  background: var(--ui-surface, #fff);
  overflow-y: auto;
}

.canvas-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.canvas-kicker {
  font-weight: 600;
  font-size: 0.9375rem;
}
.canvas-toolbar-actions {
  display: flex;
  gap: 0.375rem;
}
.canvas-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.5rem;
  border: 1px solid var(--ui-border, rgba(0, 0, 0, 0.08));
  color: var(--ui-text-muted, #6b7280);
}
.canvas-btn:hover {
  background: var(--ui-surface-hover, rgba(0, 0, 0, 0.04));
}

.canvas-empty,
.canvas-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  flex: 1;
  padding: 2rem;
  text-align: center;
  color: var(--ui-text-muted, #6b7280);
}
.canvas-empty-icon {
  opacity: 0.4;
}
.canvas-empty-title {
  font-weight: 600;
}
.canvas-empty-sub {
  font-size: 0.8125rem;
}

.canvas-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 0.875rem;
}
</style>
