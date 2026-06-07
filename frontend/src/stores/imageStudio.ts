/**
 * Image Studio Store
 * Manages conversations, generations, and generate state for the in-app image studio
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'
import imageStudioAPI from '@/api/imageStudio'
import { useAuthStore } from '@/stores/auth'
import type {
  ImageStudioConversation,
  ImageStudioGeneration,
  GenerateImageStudioRequest,
} from '@/types'

export const useImageStudioStore = defineStore('imageStudio', () => {
  // ==================== State ====================

  const conversations = ref<ImageStudioConversation[]>([])
  const activeConversationId = ref<number | null>(null)
  const generations = ref<ImageStudioGeneration[]>([])
  const loading = ref(false)
  const generating = ref(false)
  const error = ref<unknown>(null)

  // True once the first generation load has resolved. The canvas skeleton is
  // gated on this so it only appears on the very first load — never on a
  // conversation switch/create/delete, which would otherwise flash blank
  // placeholder cards over the (already-rendered) canvas.
  const hasLoadedGenerations = ref(false)

  // ==================== Conversations ====================

  /**
   * Load paginated conversations and replace the local list.
   */
  async function loadConversations(page = 1, pageSize = 50): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const resp = await imageStudioAPI.listConversations(page, pageSize)
      conversations.value = resp.items
    } catch (err) {
      error.value = err
      throw err
    } finally {
      loading.value = false
    }
  }

  /**
   * Create a new conversation and prepend it to the list.
   */
  async function createConversation(title?: string): Promise<ImageStudioConversation> {
    const conv = await imageStudioAPI.createConversation(title)
    conversations.value = [conv, ...conversations.value]
    return conv
  }

  /**
   * Rename an existing conversation (optimistic + confirm).
   */
  async function renameConversation(id: number, title: string): Promise<void> {
    const updated = await imageStudioAPI.renameConversation(id, title)
    const idx = conversations.value.findIndex((c) => c.id === id)
    if (idx !== -1) {
      conversations.value[idx] = updated
    }
  }

  /**
   * Delete a conversation and remove it from the list.
   * If it was active, clear the active selection.
   */
  async function deleteConversation(id: number): Promise<void> {
    await imageStudioAPI.deleteConversation(id)
    conversations.value = conversations.value.filter((c) => c.id !== id)
    if (activeConversationId.value === id) {
      activeConversationId.value = null
    }
  }

  /**
   * Select (switch to) a conversation and optionally reload its generations.
   */
  function selectConversation(id: number | null): void {
    activeConversationId.value = id
  }

  // ==================== Generations ====================

  /**
   * Load generations.
   * If conversationId is provided, loads that conversation's generations;
   * otherwise loads the global generation list.
   */
  async function loadGenerations(conversationId?: number): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const resp =
        conversationId !== undefined
          ? await imageStudioAPI.listConversationGenerations(conversationId)
          : await imageStudioAPI.listGenerations()
      generations.value = resp.items
    } catch (err) {
      error.value = err
      throw err
    } finally {
      loading.value = false
      hasLoadedGenerations.value = true
    }
  }

  /**
   * Clear the active generation list locally. Used when opening a brand-new
   * (empty) conversation: we show the empty state immediately, without a
   * redundant network round-trip or a skeleton flash.
   */
  function resetGenerations(): void {
    generations.value = []
    hasLoadedGenerations.value = true
  }

  /**
   * Generate images synchronously.
   * On success: prepends the new generation to `generations` and updates the auth balance.
   * On failure: sets `error` and re-throws.
   */
  async function generate(req: GenerateImageStudioRequest): Promise<void> {
    generating.value = true
    error.value = null
    try {
      // `req` already carries `referenceImage` (when set) — the API layer turns
      // it into a multipart upload. We never persist the File on the generation.
      const resp = await imageStudioAPI.generate(req)

      // Build a local ImageStudioGeneration from the response
      const newGen: ImageStudioGeneration = {
        id: resp.generation_id,
        conversation_id: resp.conversation_id,
        group_id: req.group_id,
        prompt: req.prompt,
        model: req.model,
        size: req.size,
        quality: req.quality,
        n: req.n,
        image_count: resp.images.length,
        status: 'completed',
        cost: resp.cost,
        created_at: new Date().toISOString(),
        images: resp.images,
        input_images: resp.input_images,
      }

      generations.value = [newGen, ...generations.value]

      // Update auth store balance
      const authStore = useAuthStore()
      if (authStore.user) {
        authStore.user.balance = resp.balance
      }
    } catch (err) {
      error.value = err
      throw err
    } finally {
      generating.value = false
    }
  }

  /**
   * Delete a generation and remove it from the list.
   */
  async function deleteGeneration(id: number): Promise<void> {
    await imageStudioAPI.deleteGeneration(id)
    generations.value = generations.value.filter((g) => g.id !== id)
  }

  // ==================== Return Store API ====================

  return {
    // State
    conversations,
    activeConversationId,
    generations,
    loading,
    generating,
    error,
    hasLoadedGenerations,

    // Actions
    loadConversations,
    createConversation,
    renameConversation,
    deleteConversation,
    selectConversation,
    loadGenerations,
    resetGenerations,
    generate,
    deleteGeneration,
  }
})
