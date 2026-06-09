import { describe, it, expect } from 'vitest'
import {
  estimateCost,
  aspectRatioFromSize,
  optionsForModel,
  defaultsForModel,
  ASPECT_PRESETS,
  COUNT_OPTIONS,
  parseSize,
  formatSize,
  isAutoSize,
} from '../pricing'

describe('estimateCost', () => {
  // Client-side estimation is disabled: no authoritative price tables, so every
  // combination returns null and the server computes the real charge.

  it('returns null for gpt-image-2 across all size/quality combos', () => {
    expect(estimateCost('gpt-image-2', '1K', 'auto', 1)).toBeNull()
    expect(estimateCost('gpt-image-2', '1K', 'high', 1)).toBeNull()
    expect(estimateCost('gpt-image-2', '2K', 'low', 2)).toBeNull()
    expect(estimateCost('gpt-image-2', '4K', 'medium', 4)).toBeNull()
  })

  it('returns null for unknown models too', () => {
    expect(estimateCost('mystery', '1K', 'high', 1)).toBeNull()
  })
})

describe('aspectRatioFromSize', () => {
  it('parses square / portrait / landscape sizes', () => {
    expect(aspectRatioFromSize('1024x1024')).toBe(1)
    expect(aspectRatioFromSize('1024x1536')).toBeCloseTo(1024 / 1536)
    expect(aspectRatioFromSize('1536x1024')).toBeCloseTo(1536 / 1024)
  })

  it('accepts the unicode multiplication sign', () => {
    expect(aspectRatioFromSize('1792×1024')).toBeCloseTo(1792 / 1024)
  })

  it('returns null for malformed / empty input', () => {
    expect(aspectRatioFromSize('')).toBeNull()
    expect(aspectRatioFromSize(undefined)).toBeNull()
    expect(aspectRatioFromSize('abc')).toBeNull()
    expect(aspectRatioFromSize('1024x0')).toBeNull()
  })
})

describe('ASPECT_PRESETS', () => {
  it('maps the documented aspect labels to their pixel sizes', () => {
    const byLabel = Object.fromEntries(ASPECT_PRESETS.map((p) => [p.label, p.size]))
    expect(byLabel['1:1']).toBe('1024x1024')
    expect(byLabel['2:3']).toBe('680x1024')
    expect(byLabel['3:2']).toBe('1024x680')
    expect(byLabel['3:4']).toBe('768x1024')
    expect(byLabel['4:3']).toBe('1024x768')
    expect(byLabel['9:16']).toBe('576x1024')
    expect(byLabel['16:9']).toBe('1024x576')
    expect(byLabel['1:1 (2K)']).toBe('2048x2048')
    expect(byLabel['16:9 (2K)']).toBe('2048x1152')
    expect(byLabel['9:16 (2K)']).toBe('1152x2048')
    expect(byLabel['16:9 (4K)']).toBe('3840x2160')
    expect(byLabel['9:16 (4K)']).toBe('2160x3840')
    expect(byLabel['auto']).toBe('auto')
  })

  it('contains exactly 13 presets (12 sizes + auto)', () => {
    expect(ASPECT_PRESETS).toHaveLength(13)
  })
})

describe('COUNT_OPTIONS', () => {
  it('offers 1–10', () => {
    expect(COUNT_OPTIONS).toHaveLength(10)
    expect(COUNT_OPTIONS[0]).toBe(1)
    expect(COUNT_OPTIONS[9]).toBe(10)
  })
})

describe('size helpers', () => {
  it('parseSize splits a WxH pair, with unicode × support', () => {
    expect(parseSize('1024x768')).toEqual({ w: 1024, h: 768 })
    expect(parseSize('2048×1152')).toEqual({ w: 2048, h: 1152 })
  })

  it('parseSize returns null for auto / malformed input', () => {
    expect(parseSize('auto')).toBeNull()
    expect(parseSize('')).toBeNull()
    expect(parseSize(undefined)).toBeNull()
    expect(parseSize('1024x0')).toBeNull()
  })

  it('formatSize joins width/height into the canonical string', () => {
    expect(formatSize(1024, 768)).toBe('1024x768')
  })

  it('parseSize ∘ formatSize round-trips', () => {
    const s = formatSize(680, 1024)
    expect(parseSize(s)).toEqual({ w: 680, h: 1024 })
  })

  it('isAutoSize recognises the auto sentinel (case-insensitive)', () => {
    expect(isAutoSize('auto')).toBe(true)
    expect(isAutoSize('AUTO')).toBe(true)
    expect(isAutoSize('1024x1024')).toBe(false)
    expect(isAutoSize('')).toBe(false)
    expect(isAutoSize(undefined)).toBe(false)
  })
})

describe('model option matrices', () => {
  it('workbench models share the gpt-image sizes and qualities', () => {
    const v2 = optionsForModel('gpt-image-2')
    expect(v2.sizes.map((s) => s.value)).toEqual(['1K', '2K', '4K'])
    expect(v2.qualities.map((q) => q.value)).toEqual(['auto', 'low', 'medium', 'high'])
    expect(optionsForModel('gpt-5-3').sizes).toEqual(v2.sizes)
    expect(optionsForModel('codex-gpt-image-2').qualities).toEqual(v2.qualities)
  })

  it('falls back to the gpt-image matrix for unknown models', () => {
    expect(optionsForModel('mystery').sizes).toEqual(optionsForModel('gpt-image-2').sizes)
  })

  it('provides valid defaults per model', () => {
    expect(defaultsForModel('gpt-image-2')).toEqual({ size: '1K', quality: 'auto' })
    expect(defaultsForModel('auto')).toEqual({ size: '1K', quality: 'auto' })
  })
})
