import { describe, it, expect, beforeEach } from 'vitest'
import {
  IMAGE_STUDIO_SCROLL_PREFIX,
  clearImageStudioScroll,
  readImageStudioScroll,
  writeImageStudioScroll,
} from '../scrollMemory'

describe('image studio scroll memory', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
  })

  it('persists and reads scroll positions by scoped key', () => {
    writeImageStudioScroll('all', 320)
    writeImageStudioScroll('conversation:7', 88)

    expect(readImageStudioScroll('all')).toBe(320)
    expect(readImageStudioScroll('conversation:7')).toBe(88)
  })

  it('returns null for missing or invalid values', () => {
    window.sessionStorage.setItem(`${IMAGE_STUDIO_SCROLL_PREFIX}bad`, 'not-a-number')

    expect(readImageStudioScroll('missing')).toBeNull()
    expect(readImageStudioScroll('bad')).toBeNull()
  })

  it('clears only image studio scroll keys', () => {
    writeImageStudioScroll('all', 320)
    writeImageStudioScroll('conversation:7', 88)
    window.sessionStorage.setItem('unrelated', 'keep')

    clearImageStudioScroll()

    expect(readImageStudioScroll('all')).toBeNull()
    expect(readImageStudioScroll('conversation:7')).toBeNull()
    expect(window.sessionStorage.getItem('unrelated')).toBe('keep')
  })
})
