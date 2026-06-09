<template>
  <AppLayout>
    <!--
      Immersive full-height studio. Sits on the normal AppLayout background
      (bg-gray-50 dark:bg-dark-950 + teal mesh-gradient); the serif hero carries
      the visual identity, so the bulky page header is dropped (sr-only h1 kept).
    -->
    <div class="studio-page">
      <!-- Accessible title only; the serif hero carries the visual identity. -->
      <h1 class="sr-only">{{ t('imageStudio.title') }}</h1>

      <!-- Workbench grid -->
      <div class="studio-layout">
        <!-- Left: conversations (flat / borderless) -->
        <aside
          class="studio-sidebar"
        >
          <ConversationList
            :conversations="store.conversations"
            :active-conversation-id="store.activeConversationId"
            :loading="store.loading"
            :creating="creating"
            @create="handleCreateConversation"
            @select="handleSelectConversation"
            @rename="handleRenameConversation"
            @delete="confirmDeleteConversation"
            @clear="confirmClearHistory"
          />
        </aside>

        <!-- Main column: chat-style — history (scrolls) on top, composer pinned bottom -->
        <section
          class="studio-canvas"
        >
          <div class="canvas-toolbar">
            <div class="min-w-0">
              <p class="canvas-kicker">{{ t('imageStudio.workbenchTitle') }}</p>
              <div class="canvas-mode-pills">
                <span>{{ t('imageStudio.capabilityGenerate') }}</span>
                <span>{{ t('imageStudio.capabilityEdit') }}</span>
                <span>{{ t('imageStudio.capabilityCompose') }}</span>
              </div>
            </div>
            <button
              type="button"
              class="canvas-refresh"
              :title="t('imageStudio.refreshStatus')"
              :aria-label="t('imageStudio.refreshStatus')"
              @click="store.refreshPendingGenerations().catch(() => {})"
            >
              <Icon name="refresh" size="sm" />
            </button>
          </div>
          <div class="retention-banner" role="note">
            <Icon name="exclamationTriangle" size="sm" class="retention-banner-icon" />
            <div class="min-w-0">
              <p class="retention-banner-title">{{ t('imageStudio.retentionNoticeTitle') }}</p>
              <p class="retention-banner-copy">{{ t('imageStudio.retentionNoticeBody') }}</p>
            </div>
          </div>
          <!--
            History scroll area. Always-mounted with a stable dark surface so
            switching conversations never tears the whole subtree down (which
            previously caused a white flash). Newest turn sits at the bottom.
          -->
          <div
            ref="scrollRef"
            class="canvas-scroll"
            @scroll="rememberScroll"
          >
            <TurnTimeline
              :generations="store.generations"
              :loading="store.loading && !store.hasLoadedGenerations"
              :generating="store.generating"
              :pending-prompt="pendingPrompt"
              :has-more="store.generations.length < store.generationTotal"
              :loading-more="store.loadingMoreGenerations"
              @retry="handleRetry"
              @refresh="handleRefreshGeneration"
              @delete="confirmDeleteGeneration"
              @open="openLightbox"
              @edit="handleQuickEdit"
              @reference="handleAddReference"
              @download="handleDownloadImage"
              @use-example="handleUseExample"
              @load-more="handleLoadMoreGenerations"
            />
          </div>

          <!-- Inline error banner (e.g. 403 group not enabled) — sits above the composer -->
        </section>

        <aside class="studio-inspector">
          <div class="flex min-h-0 flex-col gap-3">
            <div
              v-if="inlineError"
              class="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/10 dark:text-red-300"
            >
              <Icon name="exclamationTriangle" size="sm" class="mt-0.5 flex-shrink-0" />
              <span class="flex-1">{{ inlineError }}</span>
              <button
                type="button"
                class="flex-shrink-0 rounded p-0.5 hover:bg-red-100 dark:hover:bg-red-900/30"
                @click="inlineError = ''"
              >
                <Icon name="x" size="xs" />
              </button>
            </div>

            <ImageComposer
              ref="composerRef"
              :groups="groups"
              :loading-groups="loadingGroups"
              :generating="store.generating"
              :balance="balance"
              :history-images="historyImages"
              @generate="handleGenerate"
              @select-reference="handleSelectHistoryReference"
            />
          </div>
        </aside>
      </div>
    </div>

    <!-- Delete conversation confirm -->
    <ConfirmDialog
      :show="deleteConvTarget !== null"
      :title="t('imageStudio.deleteConversationTitle')"
      :message="t('imageStudio.deleteConversationMessage', { title: deleteConvTarget?.title || '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDeleteConversation"
      @cancel="deleteConvTarget = null"
    />

    <!-- Delete generation confirm -->
    <ConfirmDialog
      :show="deleteGenTarget !== null"
      :title="t('imageStudio.deleteGenerationTitle')"
      :message="t('imageStudio.deleteGenerationMessage')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDeleteGeneration"
      @cancel="deleteGenTarget = null"
    />

    <!-- Clear history confirm -->
    <ConfirmDialog
      :show="clearHistoryOpen"
      :title="t('imageStudio.clearHistoryTitle')"
      :message="t('imageStudio.clearHistoryMessage')"
      :confirm-text="t('imageStudio.clearHistory')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleClearHistory"
      @cancel="clearHistoryOpen = false"
    />

    <!-- Lightbox -->
    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="lightboxSrc"
          class="fixed inset-0 z-[100000050] flex items-center justify-center bg-black/80 p-6"
          @click="lightboxSrc = ''"
        >
          <img
            :src="lightboxSrc"
            class="max-h-full max-w-full rounded-lg object-contain shadow-2xl"
            @click.stop
          />
          <button
            type="button"
            class="absolute right-5 top-5 rounded-full bg-white/10 p-2 text-white hover:bg-white/20"
            @click="lightboxSrc = ''"
          >
            <Icon name="x" size="md" />
          </button>
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useImageStudioStore } from '@/stores/imageStudio'
import imageStudioAPI from '@/api/imageStudio'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { userGroupsAPI } from '@/api'
import type {
  Group,
  ImageStudioConversation,
  ImageStudioGeneration,
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ConversationList from '@/components/user/imageStudio/ConversationList.vue'
import TurnTimeline from '@/components/user/imageStudio/TurnTimeline.vue'
import ImageComposer from '@/components/user/imageStudio/ImageComposer.vue'
import type {
  ComposerHistoryImage,
  ComposerSubmitPayload,
} from '@/components/user/imageStudio/ImageComposer.vue'
import {
  clearImageStudioScroll,
  readImageStudioScroll,
  writeImageStudioScroll,
} from '@/components/user/imageStudio/scrollMemory'

const { t } = useI18n()
const store = useImageStudioStore()
const authStore = useAuthStore()
const appStore = useAppStore()

const groups = ref<Group[]>([])
const loadingGroups = ref(false)
const creating = ref(false)
const inlineError = ref('')
const lightboxSrc = ref('')
const pendingPrompt = ref('')
const clearHistoryOpen = ref(false)
const pendingPollTimer = ref<number | null>(null)
const historyReferenceGenerations = ref<ImageStudioGeneration[]>([])

const composerRef = ref<InstanceType<typeof ImageComposer> | null>(null)
const scrollRef = ref<HTMLElement | null>(null)
const deleteConvTarget = ref<ImageStudioConversation | null>(null)
const deleteGenTarget = ref<ImageStudioGeneration | null>(null)

const balance = computed(() => authStore.user?.balance ?? 0)
const historyImages = computed<ComposerHistoryImage[]>(() =>
  historyReferenceGenerations.value.flatMap((generation) =>
    (generation.images ?? []).map((url, idx) => ({
      key: `${generation.id}:${idx}`,
      url,
      prompt: generation.prompt,
    }))
  )
)

// ==================== Auto-scroll (chat: newest at the bottom) ====================

const scrollPositions = new Map<string, number>()

const scrollKey = computed(() =>
  store.activeConversationId === null ? 'all' : `conversation:${store.activeConversationId}`
)

function isNearBottom() {
  const el = scrollRef.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight < 120
}

function rememberScroll() {
  const el = scrollRef.value
  if (!el) return
  const key = scrollKey.value
  scrollPositions.set(key, el.scrollTop)
  writeImageStudioScroll(key, el.scrollTop)
}

async function restoreScroll(key = scrollKey.value) {
  await nextTick()
  const el = scrollRef.value
  if (!el) return
  const top = scrollPositions.get(key) ?? readImageStudioScroll(key)
  if (top == null) {
    el.scrollTo({ top: el.scrollHeight, behavior: 'auto' })
    return
  }
  el.scrollTo({ top, behavior: 'auto' })
}

async function scrollToBottom(smooth = true) {
  await nextTick()
  const el = scrollRef.value
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
}

// Keep the view pinned to the latest turn whenever the history grows or a new
// generation starts streaming. Watching length (not the array identity) covers
// both appends from generate() and full replacements from loadGenerations().
watch(
  () => [store.generations.length, store.generating] as const,
  () => {
    if (store.generating || isNearBottom()) scrollToBottom()
  },
  { flush: 'post' }
)

watch(
  () => store.generations.map((g) => `${g.id}:${g.status}:${g.images?.length ?? 0}`).join('|'),
  () => {
    if (isNearBottom()) scrollToBottom(false)
    if (store.generations.some((g) => (g.images ?? []).length > 0)) {
      loadHistoryReferenceImages()
    }
  }
)

watch(
  () => store.generations.some((g) => g.status === 'pending' || g.status === 'generating'),
  (hasPending) => {
    if (hasPending && pendingPollTimer.value == null) {
      pendingPollTimer.value = window.setInterval(() => {
        store.refreshPendingGenerations().catch(() => {})
      }, 5000)
    } else if (!hasPending && pendingPollTimer.value != null) {
      window.clearInterval(pendingPollTimer.value)
      pendingPollTimer.value = null
    }
  },
  { immediate: true }
)

// ==================== Error helpers ====================

interface ApiError {
  status?: number
  message?: string
}

function extractError(err: unknown): ApiError {
  if (err && typeof err === 'object') {
    const e = err as ApiError & { response?: { status?: number; data?: { message?: string } } }
    return {
      status: e.status ?? e.response?.status,
      message: e.message ?? e.response?.data?.message,
    }
  }
  return {}
}

function surfaceGenerateError(err: unknown) {
  const { status, message } = extractError(err)
  if (status === 403) {
    inlineError.value = t('imageStudio.errorGroupNotEnabled')
  } else {
    inlineError.value = message || t('imageStudio.errorGeneric')
  }
}

// ==================== Loading ====================

async function loadGroups() {
  loadingGroups.value = true
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch {
    // Non-fatal — composer simply shows the "no group" hint.
    groups.value = []
  } finally {
    loadingGroups.value = false
  }
}

async function loadHistoryReferenceImages() {
  try {
    const resp = await imageStudioAPI.listGenerations(1, 60)
    historyReferenceGenerations.value = resp.items.filter((generation) =>
      (generation.images ?? []).length > 0
    )
  } catch {
    historyReferenceGenerations.value = []
  }
}

// ==================== Conversation handlers ====================

async function handleCreateConversation() {
  creating.value = true
  inlineError.value = ''
  rememberScroll()
  try {
    const conv = await store.createConversation()
    // A brand-new conversation is empty by definition. Switch to it and show
    // the empty state immediately — skipping loadGenerations avoids a redundant
    // request and the blank skeleton/stale-content flash on the canvas.
    store.selectConversation(conv.id)
    store.resetGenerations()
    await scrollToBottom(false)
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  } finally {
    creating.value = false
  }
}

async function handleSelectConversation(id: number | null) {
  inlineError.value = ''
  rememberScroll()
  store.selectConversation(id)
  try {
    await store.loadGenerations(id ?? undefined)
    await restoreScroll()
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

async function handleRenameConversation(payload: { id: number; title: string }) {
  try {
    await store.renameConversation(payload.id, payload.title)
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

function confirmDeleteConversation(conv: ImageStudioConversation) {
  deleteConvTarget.value = conv
}

async function handleDeleteConversation() {
  const target = deleteConvTarget.value
  deleteConvTarget.value = null
  if (!target) return
  try {
    const wasActive = store.activeConversationId === target.id
    await store.deleteConversation(target.id)
    appStore.showSuccess(t('imageStudio.conversationDeleted'))
    if (wasActive) {
      await store.loadGenerations()
    }
    loadHistoryReferenceImages()
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

// ==================== Generation handlers ====================

async function runGenerate(payload: ComposerSubmitPayload) {
  inlineError.value = ''
  pendingPrompt.value = payload.prompt
  try {
    await store.generate({
      conversation_id: store.activeConversationId ?? undefined,
      ...payload,
    })
    loadHistoryReferenceImages()
    composerRef.value?.resetPrompt()
    composerRef.value?.resetReference?.()
  } catch (err) {
    surfaceGenerateError(err)
    Promise.all([
      store.loadConversations(),
      store.loadGenerations(store.activeConversationId ?? undefined),
    ]).catch(() => {})
  } finally {
    pendingPrompt.value = ''
  }
}

function handleGenerate(payload: ComposerSubmitPayload) {
  runGenerate(payload)
}

function handleUseExample(prompt: string) {
  composerRef.value?.fillPrompt(prompt)
}

/**
 * Fetch a persisted input asset and wrap it into a File so a retry of an
 * image-to-image generation stays image-to-image. Returns null (→ fall back to
 * text-to-image) if anything goes wrong.
 */
async function fetchInputAsFile(url: string): Promise<File | null> {
  try {
    const blob = await imageStudioAPI.fetchAssetBlob(url)
    const type = blob.type || 'image/png'
    const ext = type.split('/')[1] || 'png'
    return new File([blob], `source.${ext}`, { type })
  } catch {
    return null
  }
}

async function handleRetry(generation: ImageStudioGeneration) {
  const referenceImages: File[] = []
  const sources = generation.input_images ?? []
  if (sources.length > 0) {
    const fetched = await Promise.all(sources.map((source) => fetchInputAsFile(source)))
    for (const file of fetched) {
      if (file) referenceImages.push(file)
    }
    if (referenceImages.length !== sources.length) {
      // The reference image could not be re-fetched. Warn instead of silently
      // degrading an image-to-image retry into a text-to-image one.
      appStore.showWarning(t('imageStudio.retryReferenceFetchFailed'))
    }
  }
  const retryMode: ComposerSubmitPayload['mode'] =
    referenceImages.length >= 2 ? 'compose' : referenceImages.length === 1 ? 'edit' : 'generate'
  runGenerate({
    group_id: generation.group_id,
    mode: retryMode,
    prompt: generation.prompt,
    model: generation.model,
    size: generation.size,
    quality: generation.quality,
    n: generation.n,
    referenceImage: referenceImages[0] ?? null,
    referenceImages,
  })
}

async function handleQuickEdit(payload: { generation: ImageStudioGeneration; url: string }) {
  inlineError.value = ''
  const file = await fetchInputAsFile(payload.url)
  if (!file) {
    appStore.showError(t('imageStudio.imageLoadFailed'))
    return
  }
  composerRef.value?.loadReferenceFiles?.([file], 'edit')
  composerRef.value?.focusPrompt?.()
  appStore.showInfo(t('imageStudio.quickEditReady'))
}

async function appendReferenceFromUrl(url: string) {
  const file = await fetchInputAsFile(url)
  if (!file) {
    appStore.showError(t('imageStudio.imageLoadFailed'))
    return
  }
  composerRef.value?.appendReferenceFiles?.([file])
  composerRef.value?.focusPrompt?.()
  appStore.showInfo(t('imageStudio.referenceAdded'))
}

function handleAddReference(payload: { generation: ImageStudioGeneration; url: string }) {
  appendReferenceFromUrl(payload.url)
}

function handleSelectHistoryReference(payload: ComposerHistoryImage) {
  appendReferenceFromUrl(payload.url)
}

async function handleDownloadImage(payload: { generation: ImageStudioGeneration; url: string; index: number }) {
  try {
    const blob = await imageStudioAPI.fetchAssetBlob(payload.url)
    triggerBlobDownload(
      blob,
      buildImageDownloadName(payload.generation, payload.index, imageExtensionFromBlob(blob))
    )
    appStore.showSuccess(t('imageStudio.downloadStarted'))
  } catch {
    appStore.showError(t('imageStudio.downloadFailed'))
  }
}

function triggerBlobDownload(blob: Blob, filename: string) {
  const objectUrl = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = objectUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000)
}

function buildImageDownloadName(
  generation: ImageStudioGeneration,
  index: number,
  extension: string
) {
  const promptPart = generation.prompt
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 42)
  const suffix = promptPart ? `-${promptPart}` : ''
  return `image-studio-${generation.id}-${index + 1}${suffix}.${extension}`
}

function imageExtensionFromBlob(blob: Blob) {
  const type = (blob.type || '').toLowerCase()
  if (type.includes('jpeg') || type.includes('jpg')) return 'jpg'
  if (type.includes('webp')) return 'webp'
  if (type.includes('gif')) return 'gif'
  return 'png'
}

function confirmDeleteGeneration(generation: ImageStudioGeneration) {
  deleteGenTarget.value = generation
}

async function handleLoadMoreGenerations() {
  const el = scrollRef.value
  const previousHeight = el?.scrollHeight ?? 0
  const previousTop = el?.scrollTop ?? 0
  try {
    await store.loadMoreGenerations()
    await nextTick()
    if (el) {
      el.scrollTo({
        top: previousTop + (el.scrollHeight - previousHeight),
        behavior: 'auto',
      })
      rememberScroll()
    }
    loadHistoryReferenceImages()
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

async function handleRefreshGeneration(generation: ImageStudioGeneration) {
  try {
    await store.refreshGeneration(generation.id)
    loadHistoryReferenceImages()
    appStore.showInfo(t('imageStudio.statusRefreshed'))
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

async function handleDeleteGeneration() {
  const target = deleteGenTarget.value
  deleteGenTarget.value = null
  if (!target) return
  try {
    await store.deleteGeneration(target.id)
    historyReferenceGenerations.value = historyReferenceGenerations.value.filter(
      (generation) => generation.id !== target.id
    )
    appStore.showSuccess(t('imageStudio.generationDeleted'))
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

function confirmClearHistory() {
  clearHistoryOpen.value = true
}

async function handleClearHistory() {
  clearHistoryOpen.value = false
  try {
    await store.clearHistory()
    historyReferenceGenerations.value = []
    scrollPositions.clear()
    clearImageStudioScroll()
    appStore.showSuccess(t('imageStudio.historyCleared'))
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
}

// ==================== Lightbox ====================

function openLightbox(src: string) {
  lightboxSrc.value = src
}

// ==================== Mount ====================

onMounted(async () => {
  loadGroups()
  try {
    await Promise.all([
      store.loadConversations(),
      store.loadGenerations(),
      loadHistoryReferenceImages(),
    ])
    await scrollToBottom(false)
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  }
})

onBeforeUnmount(() => {
  if (pendingPollTimer.value != null) {
    window.clearInterval(pendingPollTimer.value)
    pendingPollTimer.value = null
  }
})
</script>

<style scoped>
.studio-page {
  width: min(100%, 1760px);
  margin-inline: auto;
  min-height: 0;
}

.studio-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
}

.studio-sidebar {
  order: 3;
  min-height: 0;
}

.studio-canvas {
  order: 2;
  display: flex;
  min-height: 56vh;
  min-width: 0;
  flex-direction: column;
}

.studio-inspector {
  order: 1;
  min-width: 0;
}

.canvas-toolbar {
  @apply mb-3 flex items-center justify-between gap-3 rounded-2xl border border-gray-100 bg-white/80 px-4 py-3 shadow-sm;
  @apply dark:border-dark-700/50 dark:bg-dark-800/80;
}

.canvas-kicker {
  @apply text-sm font-semibold text-gray-900 dark:text-white;
}

.canvas-mode-pills {
  @apply mt-2 flex flex-wrap gap-1.5;
}

.canvas-mode-pills span {
  @apply rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600;
  @apply dark:bg-dark-700 dark:text-gray-300;
}

.canvas-refresh {
  @apply flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 transition-colors;
  @apply hover:bg-primary-50 hover:text-primary-600;
  @apply focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30;
  @apply dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-primary-900/20 dark:hover:text-primary-300;
}

.canvas-scroll {
  @apply min-h-0 flex-1 scroll-pb-10 overflow-y-auto rounded-2xl border border-gray-100 bg-gray-50/40;
  @apply dark:border-dark-700/50 dark:bg-dark-900;
}

.retention-banner {
  @apply mb-3 flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-900 shadow-sm;
  @apply dark:border-amber-800/50 dark:bg-amber-950/30 dark:text-amber-100;
}

.retention-banner-icon {
  @apply mt-0.5 flex-shrink-0 text-amber-600 dark:text-amber-300;
}

.retention-banner-title {
  @apply text-sm font-semibold;
}

.retention-banner-copy {
  @apply mt-0.5 text-xs leading-relaxed text-amber-800 dark:text-amber-200/80;
}

@media (min-width: 1024px) and (max-width: 1279px) {
  .studio-layout {
    grid-template-columns: 260px minmax(0, 1fr);
  }

  .studio-sidebar {
    order: 1;
  }

  .studio-canvas {
    order: 2;
  }

  .studio-inspector {
    order: 3;
    grid-column: 1 / -1;
  }
}

@media (min-width: 1280px) {
  .studio-page {
    height: calc(100vh - 8rem);
    max-height: calc(100vh - 8rem);
    overflow: hidden;
  }

  .studio-layout {
    grid-template-columns: 260px minmax(0, 1fr) min(430px, 27vw);
    height: 100%;
    align-items: stretch;
  }

  .studio-sidebar {
    order: 1;
    height: 100%;
    overflow: hidden;
  }

  .studio-canvas {
    order: 2;
    height: 100%;
    min-height: 0;
  }

  .studio-inspector {
    order: 3;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .studio-inspector > div {
    height: 100%;
  }
}

@media (min-width: 1680px) {
  .studio-layout {
    grid-template-columns: 280px minmax(0, 1fr) 460px;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
