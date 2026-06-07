/**
 * useAuthedImage
 * Fetches a Bearer-protected image asset and returns an object URL
 * safe for use in <img :src="src">.
 *
 * Handles revocation on URL change and on component unmount.
 */

import { ref, watch, onUnmounted, isRef, type Ref } from 'vue'
import { fetchAssetBlob } from '@/api/imageStudio'

export function useAuthedImage(assetUrl: Ref<string | undefined | null> | string | undefined | null) {
  const src = ref<string | undefined>(undefined)
  const loading = ref(false)
  const error = ref<unknown>(null)

  // Normalise to a ref so we can watch it uniformly
  const urlRef: Ref<string | undefined | null> = isRef(assetUrl)
    ? assetUrl
    : ref(assetUrl)

  let currentObjectUrl: string | undefined

  function revokeCurrent() {
    if (currentObjectUrl) {
      URL.revokeObjectURL(currentObjectUrl)
      currentObjectUrl = undefined
    }
  }

  async function load(url: string | undefined | null) {
    revokeCurrent()
    src.value = undefined
    error.value = null

    if (!url) return

    loading.value = true
    try {
      const blob = await fetchAssetBlob(url)
      const objectUrl = URL.createObjectURL(blob)
      currentObjectUrl = objectUrl
      src.value = objectUrl
    } catch (err) {
      error.value = err
    } finally {
      loading.value = false
    }
  }

  // Watch for URL changes
  watch(urlRef, (newUrl) => {
    load(newUrl)
  }, { immediate: true })

  // Clean up on component unmount
  onUnmounted(() => {
    revokeCurrent()
  })

  return { src, loading, error }
}
