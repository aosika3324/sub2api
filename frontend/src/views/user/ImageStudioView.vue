<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl">
      <!-- Page header -->
      <div class="mb-5">
        <h1 class="flex items-center gap-2 text-xl font-bold text-gray-900 dark:text-white">
          <Icon name="sparkles" size="md" class="text-primary-500" />
          {{ t('imageStudio.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('imageStudio.subtitle') }}
        </p>
      </div>

      <!-- Workbench grid -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-[260px_minmax(0,1fr)]">
        <!-- Left: conversations -->
        <aside
          class="card order-2 h-fit p-3 lg:order-1 lg:sticky lg:top-24 lg:max-h-[calc(100vh-7rem)]"
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
          />
        </aside>

        <!-- Main column: composer (top) + results (below) -->
        <section class="order-1 flex min-w-0 flex-col gap-5 lg:order-2">
          <!-- Prompt console -->
          <ImageComposer
            ref="composerRef"
            :groups="groups"
            :loading-groups="loadingGroups"
            :generating="store.generating"
            :balance="balance"
            @generate="handleGenerate"
          />

          <!-- Inline error banner (e.g. 403 group not enabled) -->
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

          <!-- Results gallery (newest first) -->
          <div class="min-h-[220px] flex-1">
            <TurnTimeline
              :generations="store.generations"
              :loading="store.loading"
              :generating="store.generating"
              :pending-prompt="pendingPrompt"
              @retry="handleRetry"
              @delete="confirmDeleteGeneration"
              @open="openLightbox"
              @use-example="handleUseExample"
            />
          </div>
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
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useImageStudioStore } from '@/stores/imageStudio'
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

const composerRef = ref<InstanceType<typeof ImageComposer> | null>(null)
const deleteConvTarget = ref<ImageStudioConversation | null>(null)
const deleteGenTarget = ref<ImageStudioGeneration | null>(null)

const balance = computed(() => authStore.user?.balance ?? 0)

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
  try {
    const conv = await store.createConversation()
    await handleSelectConversation(conv.id)
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
  } finally {
    creating.value = false
  }
}

async function handleSelectConversation(id: number | null) {
  inlineError.value = ''
  store.selectConversation(id)
  try {
    await store.loadGenerations(id ?? undefined)
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
  } catch (err) {
    surfaceGenerateError(err)
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

function handleRetry(generation: ImageStudioGeneration) {
  runGenerate({
    group_id: generation.group_id,
    prompt: generation.prompt,
    model: generation.model,
    size: generation.size,
    quality: generation.quality,
    n: generation.n,
  })
}

function confirmDeleteGeneration(generation: ImageStudioGeneration) {
  deleteGenTarget.value = generation
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

// ==================== Lightbox ====================

function openLightbox(src: string) {
  lightboxSrc.value = src
}

// ==================== Mount ====================

onMounted(async () => {
  loadGroups()
  try {
    await Promise.all([store.loadConversations(), store.loadGenerations()])
  } catch (err) {
    appStore.showError(extractError(err).message || t('imageStudio.errorGeneric'))
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
