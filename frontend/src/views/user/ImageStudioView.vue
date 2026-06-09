<template>
  <AppLayout>
    <!--
      Immersive full-height studio. Sits on the normal AppLayout background
      (bg-gray-50 dark:bg-dark-950 + teal mesh-gradient); the serif hero carries
      the visual identity, so the bulky page header is dropped (sr-only h1 kept).
    -->
    <div class="mx-auto max-w-[1600px]">
      <!-- Accessible title only; the serif hero carries the visual identity. -->
      <h1 class="sr-only">{{ t('imageStudio.title') }}</h1>

      <!-- Workbench grid -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-[260px_minmax(0,1fr)]">
        <!-- Left: conversations (flat / borderless) -->
        <aside
          class="order-2 h-fit min-h-0 lg:order-1 lg:h-[calc(100vh-8rem)]"
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
          class="order-1 flex min-h-[60vh] min-w-0 flex-col gap-3 lg:order-2 lg:h-[calc(100vh-7rem)]"
        >
          <!--
            History scroll area. Always-mounted with a stable dark surface so
            switching conversations never tears the whole subtree down (which
            previously caused a white flash). Newest turn sits at the bottom.
          -->
          <div
            ref="scrollRef"
            class="min-h-0 flex-1 overflow-y-auto rounded-2xl border border-gray-100 bg-gray-50/40 dark:border-dark-700/50 dark:bg-dark-900"
            @scroll="rememberScroll"
          >
            <TurnTimeline
              :generations="store.generations"
              :loading="store.loading && !store.hasLoadedGenerations"
              :generating="store.generating"
              :pending-prompt="pendingPrompt"
              @retry="handleRetry"
              @refresh="handleRefreshGeneration"
              @delete="confirmDeleteGeneration"
              @open="openLightbox"
              @use-example="handleUseExample"
            />
          </div>

          <!-- Inline error banner (e.g. 403 group not enabled) — sits above the composer -->
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

          <!-- Prompt console pinned at the bottom -->
          <ImageComposer
            ref="composerRef"
            class="flex-shrink-0"
            :groups="groups"
            :loading-groups="loadingGroups"
            :generating="store.generating"
            :balance="balance"
            @generate="handleGenerate"
          />
        </section>
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
import type { ComposerSubmitPayload } from '@/components/user/imageStudio/ImageComposer.vue'

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

const composerRef = ref<InstanceType<typeof ImageComposer> | null>(null)
const scrollRef = ref<HTMLElement | null>(null)
const deleteConvTarget = ref<ImageStudioConversation | null>(null)
const deleteGenTarget = ref<ImageStudioGeneration | null>(null)

const balance = computed(() => authStore.user?.balance ?? 0)

// ==================== Auto-scroll (chat: newest at the bottom) ====================

const scrollPositions = new Map<string, number>()
const persistedScrollPrefix = 'image-studio-scroll:'

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
  writePersistedScroll(key, el.scrollTop)
}

async function restoreScroll(key = scrollKey.value) {
  await nextTick()
  const el = scrollRef.value
  if (!el) return
  const top = scrollPositions.get(key) ?? readPersistedScroll(key)
  if (top == null) {
    el.scrollTo({ top: el.scrollHeight, behavior: 'auto' })
    return
  }
  el.scrollTo({ top, behavior: 'auto' })
}

function readPersistedScroll(key: string) {
  try {
    const raw = window.sessionStorage.getItem(`${persistedScrollPrefix}${key}`)
    if (raw == null) return null
    const top = Number(raw)
    return Number.isFinite(top) ? top : null
  } catch {
    return null
  }
}

function writePersistedScroll(key: string, top: number) {
  try {
    window.sessionStorage.setItem(`${persistedScrollPrefix}${key}`, String(top))
  } catch {
    // Ignore storage failures; in-memory scroll memory still works.
  }
}

function clearPersistedScrollPositions() {
  try {
    for (let i = window.sessionStorage.length - 1; i >= 0; i -= 1) {
      const key = window.sessionStorage.key(i)
      if (key?.startsWith(persistedScrollPrefix)) {
        window.sessionStorage.removeItem(key)
      }
    }
  } catch {
    // Non-fatal: clearing server history should not depend on browser storage.
  }
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
    if (store.generating || isNearBottom()) {
      scrollToBottom()
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
  runGenerate({
    group_id: generation.group_id,
    prompt: generation.prompt,
    model: generation.model,
    size: generation.size,
    quality: generation.quality,
    n: generation.n,
    referenceImage: referenceImages[0] ?? null,
    referenceImages,
  })
}

function confirmDeleteGeneration(generation: ImageStudioGeneration) {
  deleteGenTarget.value = generation
}

async function handleRefreshGeneration(generation: ImageStudioGeneration) {
  try {
    await store.refreshGeneration(generation.id)
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
    scrollPositions.clear()
    clearPersistedScrollPositions()
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
    await Promise.all([store.loadConversations(), store.loadGenerations()])
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
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
