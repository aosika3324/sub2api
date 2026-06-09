export const IMAGE_STUDIO_SCROLL_PREFIX = 'image-studio-scroll:'

export function readImageStudioScroll(key: string): number | null {
  try {
    const raw = window.sessionStorage.getItem(`${IMAGE_STUDIO_SCROLL_PREFIX}${key}`)
    if (raw == null) return null
    const top = Number(raw)
    return Number.isFinite(top) ? top : null
  } catch {
    return null
  }
}

export function writeImageStudioScroll(key: string, top: number): void {
  try {
    window.sessionStorage.setItem(`${IMAGE_STUDIO_SCROLL_PREFIX}${key}`, String(top))
  } catch {
    // Ignore storage failures; in-memory scroll memory still works.
  }
}

export function clearImageStudioScroll(): void {
  try {
    for (let i = window.sessionStorage.length - 1; i >= 0; i -= 1) {
      const key = window.sessionStorage.key(i)
      if (key?.startsWith(IMAGE_STUDIO_SCROLL_PREFIX)) {
        window.sessionStorage.removeItem(key)
      }
    }
  } catch {
    // Non-fatal: clearing server history should not depend on browser storage.
  }
}
