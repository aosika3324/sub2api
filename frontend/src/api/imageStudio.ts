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

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export function normalizeAssetPath(url: string): string {
  return url.replace(/^\/api\/v1(?=\/)/, '')
}

export function toAssetBrowserURL(url: string): string {
  if (/^https?:\/\//i.test(url)) return url
  if (url.startsWith('/api/v1/')) return url
  if (url.startsWith('/user/image-studio/')) {
    return `${API_BASE_URL.replace(/\/$/, '')}${url}`
  }
  return url
}

// ==================== Generate ====================

/**
 * Generate images synchronously — awaits the result.
 *
 * When reference images are present the request is sent as multipart/form-data:
 * each file is appended as the `image` form field and scalar params as strings.
 *
 * NOTE: the shared apiClient defaults `Content-Type` to `application/json`. With
 * that default, axios's transformRequest JSON-stringifies any FormData payload
 * (dropping the file). We therefore null out `Content-Type` for this one request
 * so the browser sets the real `multipart/form-data; boundary=…` header itself.
 *
 * Otherwise the request is the plain JSON post (client-only reference image
 * keys are omitted from the body).
 */
export async function generate(
  req: GenerateImageStudioRequest
): Promise<GenerateImageStudioResponse> {
  const referenceImages =
    req.referenceImages && req.referenceImages.length > 0
      ? req.referenceImages
      : req.referenceImage
        ? [req.referenceImage]
        : []

  if (referenceImages.length > 0) {
    const mode = req.mode ?? (referenceImages.length >= 2 ? 'compose' : 'edit')
    const fd = new FormData()
    if (req.conversation_id != null) {
      fd.append('conversation_id', String(req.conversation_id))
    }
    fd.append('group_id', String(req.group_id))
    fd.append('mode', mode)
    fd.append('prompt', req.prompt)
    fd.append('model', req.model)
    fd.append('size', req.size)
    fd.append('quality', req.quality)
    fd.append('n', String(req.n))
    referenceImages.forEach((file) => fd.append('image', file))

    const { data } = await apiClient.post<GenerateImageStudioResponse>(
      '/user/image-studio/generate',
      fd,
      // `Content-Type: null` is intentional, not dead code: it DELETES the shared
      // apiClient `application/json` default for this one request, so the browser
      // generates the real `multipart/form-data; boundary=…` header. Omitting it
      // would let axios JSON-stringify the FormData and silently drop the file.
      { headers: { 'Content-Type': null }, timeout: 0 }
    )
    return data
  }

  // Strip the client-only reference image keys from the JSON body.
  const { referenceImage: _referenceImage, referenceImages: _referenceImages, ...jsonBody } = req
  const { data } = await apiClient.post<GenerateImageStudioResponse>(
    '/user/image-studio/generate',
    jsonBody,
    { timeout: 0 }
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
 * Fetch one generation by ID for progress/status polling.
 */
export async function getGeneration(id: number): Promise<ImageStudioGeneration> {
  const { data } = await apiClient.get<ImageStudioGeneration>(
    `/user/image-studio/generations/${id}`
  )
  return data
}

/**
 * Delete a generation by ID.
 */
export async function deleteGeneration(id: number): Promise<void> {
  await apiClient.delete(`/user/image-studio/generations/${id}`)
}

/**
 * Clear all image studio conversations and generations for the current user.
 */
export async function clearHistory(): Promise<void> {
  await apiClient.delete('/user/image-studio/history')
}

// ==================== Assets ====================

/**
 * Fetch a protected image asset as a Blob.
 * The apiClient injects the Bearer token automatically.
 *
 * The backend builds asset URLs already prefixed with the API base path
 * (e.g. "/api/v1/user/image-studio/assets/:genID/:idx"), but `apiClient`'s
 * baseURL is also "/api/v1". Passing the absolute path straight through would
 * make axios produce "/api/v1/api/v1/..." (a 404). Strip the leading base
 * prefix so the final request carries exactly one "/api/v1" segment.
 *
 * @param url - The asset URL (e.g. /api/v1/user/image-studio/assets/:genID/:idx)
 */
export async function fetchAssetBlob(url: string): Promise<Blob> {
  const path = normalizeAssetPath(url)
  const { data, headers } = await apiClient.get<Blob>(path, { responseType: 'blob' })
  if (!isBlobLike(data)) {
    throw new Error('Image asset response is not an image')
  }
  const declaredType = getHeader(headers, 'content-type') || data.type
  const imageType = declaredType.toLowerCase().startsWith('image/')
    ? declaredType
    : await sniffImageType(data)
  if (!imageType) {
    throw new Error('Image asset response is not an image')
  }
  return data.type === imageType ? data : data.slice(0, data.size, imageType)
}

function isBlobLike(value: unknown): value is Blob {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as Blob).slice === 'function' &&
    typeof (value as Blob).size === 'number'
  )
}

function getHeader(headers: unknown, name: string): string {
  const maybeHeaders = headers as
    | Record<string, unknown>
    | { get?: (key: string) => unknown }
    | undefined
  const getter = (maybeHeaders as { get?: unknown } | undefined)?.get
  const fromGetter = typeof getter === 'function' ? getter.call(maybeHeaders, name) : undefined
  if (typeof fromGetter === 'string') return fromGetter
  const direct = (maybeHeaders as Record<string, unknown> | undefined)?.[name]
  if (typeof direct === 'string') return direct
  return ''
}

async function sniffImageType(blob: Blob): Promise<string> {
  const head = new Uint8Array(await readBlobArrayBuffer(blob.slice(0, 16)))
  if (
    head[0] === 0x89 &&
    head[1] === 0x50 &&
    head[2] === 0x4e &&
    head[3] === 0x47 &&
    head[4] === 0x0d &&
    head[5] === 0x0a &&
    head[6] === 0x1a &&
    head[7] === 0x0a
  ) {
    return 'image/png'
  }
  if (head[0] === 0xff && head[1] === 0xd8 && head[2] === 0xff) {
    return 'image/jpeg'
  }
  if (
    head[0] === 0x47 &&
    head[1] === 0x49 &&
    head[2] === 0x46 &&
    head[3] === 0x38
  ) {
    return 'image/gif'
  }
  if (
    head[0] === 0x52 &&
    head[1] === 0x49 &&
    head[2] === 0x46 &&
    head[3] === 0x46 &&
    head[8] === 0x57 &&
    head[9] === 0x45 &&
    head[10] === 0x42 &&
    head[11] === 0x50
  ) {
    return 'image/webp'
  }
  return ''
}

function readBlobArrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  const arrayBuffer = blob.arrayBuffer
  if (typeof arrayBuffer === 'function') {
    return arrayBuffer.call(blob)
  }
  if (typeof FileReader !== 'undefined') {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(reader.result as ArrayBuffer)
      reader.onerror = () => reject(reader.error ?? new Error('Unable to read image blob'))
      reader.readAsArrayBuffer(blob)
    })
  }
  return Promise.reject(new Error('Unable to read image blob'))
}

export const imageStudioAPI = {
  generate,
  listConversations,
  createConversation,
  renameConversation,
  deleteConversation,
  listConversationGenerations,
  listGenerations,
  getGeneration,
  deleteGeneration,
  clearHistory,
  fetchAssetBlob,
}

export default imageStudioAPI
