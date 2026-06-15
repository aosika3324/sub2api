/**
 * Video Studio API endpoints
 * Handles in-app (JWT) Veo video generations: submit, poll, list, and the
 * authed proxy-stream of the produced video bytes.
 *
 * Unlike the image studio, Veo is an async long task. Submit returns immediately
 * with a `processing` generation; the client polls (per-row or batched) until the
 * status flips to `succeeded`/`failed`. The produced video is NOT stored locally
 * server-side — it is proxy-streamed from the upstream signed URI on demand and
 * expires, so the UI surfaces a retention notice.
 */

import { apiClient } from './client'
import type {
  PaginatedResponse,
  VideoStudioGeneration,
  GenerateVideoStudioRequest,
  GenerateVideoStudioResponse,
} from '@/types'

/**
 * Strip the leading "/api/v1" base prefix so a backend-built absolute path
 * (e.g. "/api/v1/user/video-studio/generations/:id/video/:idx") becomes a path
 * relative to apiClient's baseURL (which is also "/api/v1"), avoiding a
 * duplicated "/api/v1/api/v1/..." request.
 */
export function normalizeVideoPath(url: string): string {
  return url.replace(/^\/api\/v1(?=\/)/, '')
}

// ==================== Generate ====================

/**
 * Submit a Veo video generation. Returns immediately with a processing
 * generation; track completion via getGeneration/batchGetGenerations.
 */
export async function generate(
  req: GenerateVideoStudioRequest
): Promise<GenerateVideoStudioResponse> {
  const { data } = await apiClient.post<GenerateVideoStudioResponse>(
    '/user/video-studio/generate',
    req,
    { timeout: 0 }
  )
  return data
}

// ==================== Generations ====================

/**
 * List all video generations for the current user (paginated). Processing rows
 * are reconciled server-side on read, so the returned status is fresh.
 */
export async function listGenerations(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<VideoStudioGeneration>> {
  const { data } = await apiClient.get<PaginatedResponse<VideoStudioGeneration>>(
    '/user/video-studio/generations',
    { params: { page, page_size: pageSize } }
  )
  return data
}

/**
 * Fetch one generation by ID for progress/status polling.
 */
export async function getGeneration(id: number): Promise<VideoStudioGeneration> {
  const { data } = await apiClient.get<VideoStudioGeneration>(
    `/user/video-studio/generations/${id}`
  )
  return data
}

/**
 * Batch-fetch generation statuses in one round-trip (replaces per-pending N+1
 * polling). Ownership is enforced server-side; unknown/foreign IDs are omitted.
 */
export async function batchGetGenerations(ids: number[]): Promise<VideoStudioGeneration[]> {
  if (ids.length === 0) return []
  const { data } = await apiClient.get<{ items: VideoStudioGeneration[] }>(
    '/user/video-studio/generations-batch',
    { params: { ids: ids.join(',') } }
  )
  return data.items ?? []
}

/**
 * Delete a generation by ID.
 */
export async function deleteGeneration(id: number): Promise<void> {
  await apiClient.delete(`/user/video-studio/generations/${id}`)
}

/**
 * Clear all video generations for the current user.
 */
export async function clearHistory(): Promise<void> {
  await apiClient.delete('/user/video-studio/history')
}

// ==================== Assets ====================

/**
 * Fetch a protected video sample as a Blob. The apiClient injects the Bearer
 * token automatically. The produced video is proxy-streamed from the upstream on
 * demand; a failure here usually means the upstream signed URI has expired.
 *
 * @param url - The video URL (e.g. /api/v1/user/video-studio/generations/:id/video/:idx)
 */
export async function fetchVideoBlob(url: string): Promise<Blob> {
  const path = normalizeVideoPath(url)
  const { data } = await apiClient.get<Blob>(path, { responseType: 'blob', timeout: 0 })
  if (!isBlobLike(data)) {
    throw new Error('Video asset response is not a blob')
  }
  return data
}

function isBlobLike(value: unknown): value is Blob {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as Blob).slice === 'function' &&
    typeof (value as Blob).size === 'number'
  )
}

export const videoStudioAPI = {
  generate,
  listGenerations,
  getGeneration,
  batchGetGenerations,
  deleteGeneration,
  clearHistory,
  fetchVideoBlob,
}

export default videoStudioAPI
