/**
 * Video Studio Store
 * Manages in-app (JWT) Veo video generations and their async polling.
 *
 * Veo is an async long task: submit returns a `processing` row, and the client
 * polls (batched) until each row settles to `succeeded`/`failed`. This store
 * owns the generation list, the submit action, and the batch-poll reconciliation
 * loop — mirroring the image studio store's pending-poll pattern.
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'
import videoStudioAPI from '@/api/videoStudio'
import { useAuthStore } from '@/stores/auth'
import type { VideoStudioGeneration, GenerateVideoStudioRequest } from '@/types'

export const useVideoStudioStore = defineStore('videoStudio', () => {
  // ==================== State ====================

  const generations = ref<VideoStudioGeneration[]>([])
  const loading = ref(false)
  const generating = ref(false)
  const error = ref<unknown>(null)
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  const hasLoaded = ref(false)

  // A processing generation older than this (client clock) with no server
  // progress is treated as failed locally so the UI stops polling a row the
  // backend's stale sweep will reclaim (or already has). Kept above the server's
  // 30-min stale timeout so it only ever fires for genuinely stuck rows.
  const STALE_PROCESSING_MS = 35 * 60 * 1000

  // ==================== Generations ====================

  /**
   * Load the first page of generations and replace the local list.
   */
  async function loadGenerations(p = 1, size = 20): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const resp = await videoStudioAPI.listGenerations(p, size)
      generations.value = resp.items
      page.value = resp.page
      pageSize.value = resp.page_size
      total.value = resp.total
      hasLoaded.value = true
    } catch (err) {
      error.value = err
      throw err
    } finally {
      loading.value = false
    }
  }

  /**
   * Submit a new video generation. On success prepends the new (processing) row
   * and updates the auth balance with the post-charge estimate.
   */
  async function generate(req: GenerateVideoStudioRequest): Promise<VideoStudioGeneration> {
    generating.value = true
    error.value = null
    try {
      const resp = await videoStudioAPI.generate(req)
      const row: VideoStudioGeneration = {
        id: resp.generation_id,
        prompt: req.prompt,
        model: resp.model,
        status: resp.status,
        sample_count: 0,
        duration_seconds: 0,
        cost: 0,
        videos: [],
        created_at: new Date().toISOString(),
      }
      generations.value = [row, ...generations.value]
      total.value += 1

      // Reflect the pre-charge estimate on the balance so the user sees the hold
      // immediately; the authoritative balance is refreshed when the row settles.
      const authStore = useAuthStore()
      if (authStore.user && typeof resp.balance === 'number') {
        authStore.user.balance = resp.balance
      }
      return row
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
    await videoStudioAPI.deleteGeneration(id)
    generations.value = generations.value.filter((g) => g.id !== id)
    total.value = Math.max(0, total.value - 1)
  }

  /**
   * Batch-poll all in-flight (pending/processing) rows in one request and patch
   * the list. A row no longer returned by the batch (deleted/expired) or stuck
   * past the stale threshold is failed locally so polling stops.
   */
  async function refreshPending(): Promise<void> {
    const pending = generations.value.filter(
      (g) => g.status === 'pending' || g.status === 'processing'
    )
    if (pending.length === 0) return

    let updated: VideoStudioGeneration[] = []
    try {
      updated = await videoStudioAPI.batchGetGenerations(pending.map((g) => g.id))
    } catch {
      // Network hiccup: skip this tick, the interval retries.
      return
    }

    const byId = new Map(updated.map((g) => [g.id, g]))
    const now = Date.now()

    for (const p of pending) {
      const idx = generations.value.findIndex((g) => g.id === p.id)
      if (idx === -1) continue

      const server = byId.get(p.id)
      if (server) {
        const previousStatus = generations.value[idx].status
        generations.value[idx] = server
        if (previousStatus !== server.status && server.status === 'succeeded') {
          // Refresh the authoritative balance after the completion charge lands.
          const authStore = useAuthStore()
          authStore.refreshUser().catch(() => {})
        }
        continue
      }

      const createdAt = Date.parse(generations.value[idx].created_at)
      if (!Number.isNaN(createdAt) && now - createdAt > STALE_PROCESSING_MS) {
        generations.value[idx] = {
          ...generations.value[idx],
          status: 'failed',
          error_code: generations.value[idx].error_code || 'interrupted',
        }
      }
    }
  }

  /**
   * Whether any row is still in flight (drives the poll interval).
   */
  function hasPending(): boolean {
    return generations.value.some(
      (g) => g.status === 'pending' || g.status === 'processing'
    )
  }

  async function clearHistory(): Promise<void> {
    await videoStudioAPI.clearHistory()
    generations.value = []
    page.value = 1
    total.value = 0
    hasLoaded.value = true
  }

  // ==================== Return Store API ====================

  return {
    // State
    generations,
    loading,
    generating,
    error,
    page,
    pageSize,
    total,
    hasLoaded,

    // Actions
    loadGenerations,
    generate,
    deleteGeneration,
    refreshPending,
    hasPending,
    clearHistory,
  }
})
