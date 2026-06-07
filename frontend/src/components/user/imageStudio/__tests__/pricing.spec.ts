import { describe, it, expect } from 'vitest'
import {
  estimateCost,
  aspectRatioFromSize,
  optionsForModel,
  defaultsForModel,
} from '../pricing'

describe('estimateCost', () => {
  // Client-side estimation is disabled: no authoritative price tables for
  // gpt-image-1.5 / gpt-image-2, so every combination returns null and the
  // server computes the real charge.
  it('returns null for gpt-image-1.5 across all size/quality combos', () => {
    expect(estimateCost('gpt-image-1.5', '1024x1024', 'auto', 1)).toBeNull()
    expect(estimateCost('gpt-image-1.5', '1024x1024', 'high', 1)).toBeNull()
    expect(estimateCost('gpt-image-1.5', '1024x1536', 'low', 2)).toBeNull()
    expect(estimateCost('gpt-image-1.5', '1536x1024', 'medium', 4)).toBeNull()
  })

  it('returns null for gpt-image-2 across all size/quality combos', () => {
    expect(estimateCost('gpt-image-2', '1024x1024', 'auto', 1)).toBeNull()
    expect(estimateCost('gpt-image-2', '1024x1024', 'high', 1)).toBeNull()
    expect(estimateCost('gpt-image-2', '1024x1536', 'low', 2)).toBeNull()
    expect(estimateCost('gpt-image-2', '1536x1024', 'medium', 4)).toBeNull()
  })

  it('returns null for unknown models too', () => {
    expect(estimateCost('mystery', '1024x1024', 'high', 1)).toBeNull()
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

describe('model option matrices', () => {
  it('both models share the gpt-image sizes and qualities', () => {
    const v15 = optionsForModel('gpt-image-1.5')
    expect(v15.sizes.map((s) => s.value)).toContain('1024x1536')
    expect(v15.qualities.map((q) => q.value)).toEqual(['auto', 'low', 'medium', 'high'])

    const v2 = optionsForModel('gpt-image-2')
    expect(v2.sizes.map((s) => s.value)).toEqual(v15.sizes.map((s) => s.value))
    expect(v2.qualities.map((q) => q.value)).toEqual(['auto', 'low', 'medium', 'high'])
  })

  it('falls back to the gpt-image matrix for unknown models', () => {
    expect(optionsForModel('mystery').sizes).toEqual(optionsForModel('gpt-image-2').sizes)
  })

  it('provides valid defaults per model', () => {
    expect(defaultsForModel('gpt-image-1.5')).toEqual({ size: '1024x1024', quality: 'auto' })
    expect(defaultsForModel('gpt-image-2')).toEqual({ size: '1024x1024', quality: 'auto' })
  })
})
