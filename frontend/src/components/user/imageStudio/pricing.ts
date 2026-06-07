/**
 * Image Studio — client-side option matrices and best-effort cost estimation.
 *
 * The backend is the source of truth for billing; these estimates are a UX
 * convenience so the Generate button can show "≈$x.xx" before submitting.
 * We only return a number when we are confident about the per-image price for
 * the given model/size/quality combination — otherwise `estimateCost` returns
 * `null` and the caller falls back to a plain "Generate" label.
 */

export type ModelId = 'gpt-image-1' | 'dall-e-3'

export interface SizeOption {
  value: string
  label: string
}

export interface QualityOption {
  value: string
  /** i18n key for the human label, when one exists. */
  labelKey?: string
}

interface ModelMatrix {
  sizes: SizeOption[]
  qualities: QualityOption[]
  defaultSize: string
  defaultQuality: string
}

export const MODEL_OPTIONS: Array<{ value: ModelId; label: string }> = [
  { value: 'gpt-image-1', label: 'gpt-image-1' },
  { value: 'dall-e-3', label: 'dall-e-3' },
]

const MATRICES: Record<ModelId, ModelMatrix> = {
  'gpt-image-1': {
    sizes: [
      { value: '1024x1024', label: '1024 × 1024' },
      { value: '1024x1536', label: '1024 × 1536' },
      { value: '1536x1024', label: '1536 × 1024' },
    ],
    qualities: [
      { value: 'auto', labelKey: 'imageStudio.qualityAuto' },
      { value: 'low', labelKey: 'imageStudio.qualityLow' },
      { value: 'medium', labelKey: 'imageStudio.qualityMedium' },
      { value: 'high', labelKey: 'imageStudio.qualityHigh' },
    ],
    defaultSize: '1024x1024',
    defaultQuality: 'auto',
  },
  'dall-e-3': {
    sizes: [
      { value: '1024x1024', label: '1024 × 1024' },
      { value: '1792x1024', label: '1792 × 1024' },
      { value: '1024x1792', label: '1024 × 1792' },
    ],
    qualities: [
      { value: 'standard', labelKey: 'imageStudio.qualityStandard' },
      { value: 'hd', labelKey: 'imageStudio.qualityHd' },
    ],
    defaultSize: '1024x1024',
    defaultQuality: 'standard',
  },
}

export function optionsForModel(model: string): ModelMatrix {
  return MATRICES[(model as ModelId)] ?? MATRICES['gpt-image-1']
}

export function defaultsForModel(model: string): { size: string; quality: string } {
  const m = optionsForModel(model)
  return { size: m.defaultSize, quality: m.defaultQuality }
}

// ---- Per-image price tables (USD) ----
// Sourced from OpenAI's published image pricing. These are best-effort and only
// used to render an approximate estimate; the server computes the real charge.

const GPT_IMAGE_1_PRICES: Record<string, Record<string, number>> = {
  '1024x1024': { low: 0.011, medium: 0.042, high: 0.167 },
  '1024x1536': { low: 0.016, medium: 0.063, high: 0.25 },
  '1536x1024': { low: 0.016, medium: 0.063, high: 0.25 },
}

const DALL_E_3_PRICES: Record<string, Record<string, number>> = {
  '1024x1024': { standard: 0.04, hd: 0.08 },
  '1792x1024': { standard: 0.08, hd: 0.12 },
  '1024x1792': { standard: 0.08, hd: 0.12 },
}

/**
 * Best-effort per-generation cost estimate. Returns `null` when the
 * model/size/quality combination is not confidently priceable (e.g. the
 * gpt-image-1 "auto" quality, where the server picks the tier).
 */
export function estimateCost(
  model: string,
  size: string,
  quality: string,
  n: number
): number | null {
  const count = Number.isFinite(n) && n > 0 ? n : 1
  let perImage: number | undefined

  if (model === 'dall-e-3') {
    perImage = DALL_E_3_PRICES[size]?.[quality]
  } else if (model === 'gpt-image-1') {
    // "auto" lets the server choose the tier — not confidently priceable.
    if (quality === 'auto') return null
    perImage = GPT_IMAGE_1_PRICES[size]?.[quality]
  }

  if (perImage == null) return null
  return Math.round(perImage * count * 1000) / 1000
}

/**
 * Parse a size string like "1024x1536" into an aspect ratio (width / height).
 * Returns `null` for unknown/malformed sizes so callers can fall back to a
 * sensible default frame.
 */
export function aspectRatioFromSize(size: string | undefined | null): number | null {
  if (!size) return null
  const match = /^(\d+)\s*[x×]\s*(\d+)$/i.exec(size.trim())
  if (!match) return null
  const w = Number(match[1])
  const h = Number(match[2])
  if (!w || !h) return null
  return w / h
}
