import { describe, it, expect } from 'vitest'
import {
  estimateCost,
  aspectRatioFromSize,
  optionsForModel,
  defaultsForModel,
} from '../pricing'

describe('estimateCost', () => {
  it('prices dall-e-3 standard/hd per image × n', () => {
    expect(estimateCost('dall-e-3', '1024x1024', 'standard', 1)).toBe(0.04)
    expect(estimateCost('dall-e-3', '1024x1024', 'hd', 1)).toBe(0.08)
    // landscape hd × 2
    expect(estimateCost('dall-e-3', '1792x1024', 'hd', 2)).toBe(0.24)
  })

  it('prices gpt-image-1 by size + quality tier', () => {
    expect(estimateCost('gpt-image-1', '1024x1024', 'high', 1)).toBe(0.167)
    expect(estimateCost('gpt-image-1', '1024x1536', 'low', 1)).toBe(0.016)
  })

  it('returns null for gpt-image-1 auto quality (server picks tier)', () => {
    expect(estimateCost('gpt-image-1', '1024x1024', 'auto', 1)).toBeNull()
  })

  it('returns null for unknown size/quality combinations', () => {
    expect(estimateCost('gpt-image-1', '9999x9999', 'high', 1)).toBeNull()
    expect(estimateCost('dall-e-3', '1024x1024', 'auto', 1)).toBeNull()
  })

  it('treats non-positive n as a single image', () => {
    expect(estimateCost('dall-e-3', '1024x1024', 'standard', 0)).toBe(0.04)
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
  it('exposes model-specific sizes and qualities', () => {
    const gpt = optionsForModel('gpt-image-1')
    expect(gpt.sizes.map((s) => s.value)).toContain('1024x1536')
    expect(gpt.qualities.map((q) => q.value)).toEqual(['auto', 'low', 'medium', 'high'])

    const dalle = optionsForModel('dall-e-3')
    expect(dalle.sizes.map((s) => s.value)).toContain('1792x1024')
    expect(dalle.qualities.map((q) => q.value)).toEqual(['standard', 'hd'])
  })

  it('falls back to gpt-image-1 for unknown models', () => {
    expect(optionsForModel('mystery').sizes).toEqual(optionsForModel('gpt-image-1').sizes)
  })

  it('provides valid defaults per model', () => {
    expect(defaultsForModel('gpt-image-1')).toEqual({ size: '1024x1024', quality: 'auto' })
    expect(defaultsForModel('dall-e-3')).toEqual({ size: '1024x1024', quality: 'standard' })
  })
})
