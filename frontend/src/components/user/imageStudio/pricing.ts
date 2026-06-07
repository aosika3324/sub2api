/**
 * Image Studio — client-side option matrices and cost estimation.
 *
 * The backend is the source of truth for billing. Client-side price estimates
 * are currently DISABLED: we do not have authoritative per-image prices for the
 * supported models (gpt-image-1.5 / gpt-image-2), so `estimateCost` always
 * returns `null` and the server computes the real charge. The composer keeps the
 * "≈$x.xx" markup so it reappears automatically if price tables are wired later.
 */

export type ModelId = 'gpt-image-1.5' | 'gpt-image-2'

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
  { value: 'gpt-image-1.5', label: 'gpt-image-1.5' },
  { value: 'gpt-image-2', label: 'gpt-image-2' },
]

// Both supported models share the gpt-image family option matrix.
const GPT_IMAGE_MATRIX: ModelMatrix = {
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
}

const MATRICES: Record<ModelId, ModelMatrix> = {
  'gpt-image-1.5': GPT_IMAGE_MATRIX,
  'gpt-image-2': GPT_IMAGE_MATRIX,
}

export function optionsForModel(model: string): ModelMatrix {
  return MATRICES[(model as ModelId)] ?? MATRICES['gpt-image-2']
}

export function defaultsForModel(model: string): { size: string; quality: string } {
  const m = optionsForModel(model)
  return { size: m.defaultSize, quality: m.defaultQuality }
}

/**
 * Per-generation cost estimate.
 *
 * Client-side estimation is currently disabled: we have no authoritative
 * per-image price tables for the supported models (gpt-image-1.5 / gpt-image-2),
 * so this always returns `null` and the server is the source of truth for the
 * real charge. The signature is kept stable so price tables can be wired back in
 * later without touching callers. Params are intentionally unused for now.
 */
export function estimateCost(
  _model: string,
  _size: string,
  _quality: string,
  _n: number
): number | null {
  return null
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
