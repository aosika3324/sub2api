/**
 * Image Studio API endpoints
 * Handles conversations, generations, and assets for the in-app image studio
 */

import { apiClient } from './client'
import type {
  PaginatedResponse,
  ImageStudioConversation,
  ImageStudioGeneration,
  GenerateImageStudioRequest,
  GenerateImageStudioResponse,
} from '@/types'

// ==================== Generate ====================

/**
 * Generate images synchronously — awaits the result.
 */
export async function generate(
  req: GenerateImageStudioRequest
): Promise<GenerateImageStudioResponse> {
  const { data } = await apiClient.post<GenerateImageStudioResponse>(
    '/user/image-studio/generate',
    req
  )
  return data
}

// ==================== Conversations ====================

/**
 * List all image studio conversations (paginated).
 */
export async function listConversations(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<ImageStudioConversation>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageStudioConversation>>(
    '/user/image-studio/conversations',
    { params: { page, page_size: pageSize } }
  )
  return data
}

/**
 * Create a new image studio conversation.
 */
export async function createConversation(title?: string): Promise<ImageStudioConversation> {
  const { data } = await apiClient.post<ImageStudioConversation>(
    '/user/image-studio/conversations',
    title ? { title } : {}
  )
  return data
}

/**
 * Rename an existing conversation.
 */
export async function renameConversation(
  id: number,
  title: string
): Promise<ImageStudioConversation> {
  const { data } = await apiClient.patch<ImageStudioConversation>(
    `/user/image-studio/conversations/${id}`,
    { title }
  )
  return data
}

/**
 * Delete a conversation.
 */
export async function deleteConversation(id: number): Promise<void> {
  await apiClient.delete(`/user/image-studio/conversations/${id}`)
}

// ==================== Generations ====================

/**
 * List all generations for a specific conversation (paginated).
 */
export async function listConversationGenerations(
  conversationId: number,
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<ImageStudioGeneration>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageStudioGeneration>>(
    `/user/image-studio/conversations/${conversationId}/generations`,
    { params: { page, page_size: pageSize } }
  )
  return data
}

/**
 * List all generations across all conversations (paginated).
 */
export async function listGenerations(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<ImageStudioGeneration>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageStudioGeneration>>(
    '/user/image-studio/generations',
    { params: { page, page_size: pageSize } }
  )
  return data
}

/**
 * Delete a generation by ID.
 */
export async function deleteGeneration(id: number): Promise<void> {
  await apiClient.delete(`/user/image-studio/generations/${id}`)
}

// ==================== Assets ====================

/**
 * Fetch a protected image asset as a Blob.
 * The apiClient injects the Bearer token automatically.
 * @param url - The asset URL (e.g. /api/v1/user/image-studio/assets/:genID/:idx)
 */
export async function fetchAssetBlob(url: string): Promise<Blob> {
  return apiClient.get(url, { responseType: 'blob' }).then((r) => r.data as Blob)
}

export const imageStudioAPI = {
  generate,
  listConversations,
  createConversation,
  renameConversation,
  deleteConversation,
  listConversationGenerations,
  listGenerations,
  deleteGeneration,
  fetchAssetBlob,
}

export default imageStudioAPI
