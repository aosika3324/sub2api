/**
 * useAuthedVideo
 * Fetches a Bearer-protected video sample and returns an object URL safe for use
 * in <video :src="src">.
 *
 * Mirrors useAuthedImage: the produced Veo video is proxy-streamed from the
 * backend (which injects the upstream api_key) and requires the JWT, so a plain
 * <video src="/api/v1/..."> would 401. We fetch the bytes as a blob through the
 * authed apiClient and hand back an object URL, revoking it on change/unmount.
 *
 * Loading is lazy/opt-in via `enabled` — a video is only fetched when the user
 * actually plays it, so a long history list does not eagerly download every clip.
 */

import { ref, watch, onUnmounted, isRef, type Ref } from 'vue'
import { videoStudioAPI } from '@/api/videoStudio'

export function useAuthedVideo(
  assetUrl: Ref<string | undefined | null> | string | undefined | null,
  enabled: Ref<boolean> | boolean = true
) {
  const src = ref<string | undefined>(undefined)
  const loading = ref(false)
  const error = ref<unknown>(null)

  const urlRef: Ref<string | undefined | null> = isRef(assetUrl) ? assetUrl : ref(assetUrl)
  const enabledRef: Ref<boolean> = isRef(enabled) ? enabled : ref(enabled)

  let currentObjectUrl: string | undefined
  // Cancellation token: a superseded in-flight load revokes its own URL and
  // skips all state writes (prevents blob leaks on rapid URL/enabled changes).
  let loadId = 0

  function revokeCurrent() {
    if (currentObjectUrl) {
      URL.revokeObjectURL(currentObjectUrl)
      currentObjectUrl = undefined
    }
  }

  async function load(url: string | undefined | null) {
    const myId = ++loadId
    revokeCurrent()
    src.value = undefined
    error.value = null

    if (!url) return

    loading.value = true
    try {
      const blob = await videoStudioAPI.fetchVideoBlob(url)
      const objectUrl = URL.createObjectURL(blob)
      if (myId !== loadId) {
        URL.revokeObjectURL(objectUrl)
        return
      }
      currentObjectUrl = objectUrl
      src.value = objectUrl
    } catch (err) {
      if (myId === loadId) error.value = err
    } finally {
      if (myId === loadId) loading.value = false
    }
  }

  watch(
    [urlRef, enabledRef],
    ([newUrl, isEnabled]) => {
      if (!isEnabled) return
      load(newUrl)
    },
    { immediate: true }
  )

  onUnmounted(() => {
    revokeCurrent()
  })

  return { src, loading, error }
}
