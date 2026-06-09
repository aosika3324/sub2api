/**
 * useAuthedImage
 * Fetches a Bearer-protected image asset and returns an object URL
 * safe for use in <img :src="src">.
 *
 * Handles revocation on URL change and on component unmount.
 *
 * Pass a `Ref<string>` for reactive updates (the source is watched and
 * re-fetched on change). Passing a plain string treats it as static —
 * it is fetched once and never re-watched.
 */

import { ref, watch, onUnmounted, isRef, type Ref } from 'vue'
import { fetchAssetBlob, toAssetBrowserURL } from '@/api/imageStudio'

export function useAuthedImage(
  assetUrl: Ref<string | undefined | null> | string | undefined | null,
  enabled: Ref<boolean> | boolean = true
) {
  const src = ref<string | undefined>(undefined)
  const fallbackSrc = ref<string | undefined>(undefined)
  const loading = ref(false)
  const error = ref<unknown>(null)

  // Normalise to a ref so we can watch it uniformly
  const urlRef: Ref<string | undefined | null> = isRef(assetUrl)
    ? assetUrl
    : ref(assetUrl)
  const enabledRef: Ref<boolean> = isRef(enabled) ? enabled : ref(enabled)

  let currentObjectUrl: string | undefined

  // Cancellation token: each load() bumps loadId. An in-flight load whose
  // myId no longer matches loadId has been superseded — it must revoke the
  // object URL it just created (otherwise rapid URL changes leak blobs) and
  // skip all state writes.
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
    fallbackSrc.value = undefined
    error.value = null

    if (!url) return
    fallbackSrc.value = toAssetBrowserURL(url)

    loading.value = true
    try {
      const blob = await fetchAssetBlob(url)
      const objectUrl = URL.createObjectURL(blob)
      if (myId !== loadId) {
        // Superseded by a newer load — revoke our own URL and bail.
        URL.revokeObjectURL(objectUrl)
        return
      }
      currentObjectUrl = objectUrl
      src.value = objectUrl
    } catch (err) {
      if (myId === loadId) {
        error.value = err
        src.value = fallbackSrc.value
      }
    } finally {
      if (myId === loadId) loading.value = false
    }
  }

  // Watch for URL changes
  watch([urlRef, enabledRef], ([newUrl, isEnabled]) => {
    if (!isEnabled) return
    load(newUrl)
  }, { immediate: true })

  // Clean up on component unmount
  onUnmounted(() => {
    revokeCurrent()
  })

  return { src, fallbackSrc, loading, error }
}
