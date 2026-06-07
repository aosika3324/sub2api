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

/**
 * Aspect-ratio preset. `size` is either a concrete "WxH" pixel pair or the
 * sentinel `"auto"` (let the backend pick). The composer's W×H inputs and the
 * aspect grid both drive the same `size` payload, so a preset click simply
 * fills width/height (or toggles auto mode).
 */
export interface AspectPreset {
  label: string
  size: string
}

export const ASPECT_PRESETS: AspectPreset[] = [
  { label: '1:1', size: '1024x1024' },
  { label: '2:3', size: '680x1024' },
  { label: '3:2', size: '1024x680' },
  { label: '3:4', size: '768x1024' },
  { label: '4:3', size: '1024x768' },
  { label: '9:16', size: '576x1024' },
  { label: '16:9', size: '1024x576' },
  { label: '1:1 (2K)', size: '2048x2048' },
  { label: '16:9 (2K)', size: '2048x1152' },
  { label: '9:16 (2K)', size: '1152x2048' },
  { label: '16:9 (4K)', size: '3840x2160' },
  { label: '9:16 (4K)', size: '2160x3840' },
  { label: 'auto', size: 'auto' },
]

/** Generation count options — the backend accepts 1–10 images per request. */
export const COUNT_OPTIONS = Array.from({ length: 10 }, (_, i) => i + 1)

// Both supported models share the gpt-image family option matrix.
const GPT_IMAGE_MATRIX: ModelMatrix = {
  sizes: [
    { value: '1K', label: '1K' },
    { value: '2K', label: '2K' },
    { value: '4K', label: '4K' },
  ],
  qualities: [
    { value: 'auto', labelKey: 'imageStudio.qualityAuto' },
    { value: 'low', labelKey: 'imageStudio.qualityLow' },
    { value: 'medium', labelKey: 'imageStudio.qualityMedium' },
    { value: 'high', labelKey: 'imageStudio.qualityHigh' },
  ],
  defaultSize: '1K',
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

/**
 * Parse a "WxH" size string into its width/height components. Shares the same
 * regex as {@link aspectRatioFromSize} so the two stay consistent. Returns
 * `null` for the `auto` sentinel or any malformed input.
 */
export function parseSize(size: string | undefined | null): { w: number; h: number } | null {
  if (!size) return null
  const match = /^(\d+)\s*[x×]\s*(\d+)$/i.exec(size.trim())
  if (!match) return null
  const w = Number(match[1])
  const h = Number(match[2])
  if (!w || !h) return null
  return { w, h }
}

/** Format a width/height pair back into the canonical "WxH" payload string. */
export function formatSize(w: number, h: number): string {
  return `${w}x${h}`
}

/** Whether a size string is the `auto` sentinel (case-insensitive). */
export function isAutoSize(size: string | undefined | null): boolean {
  return (size ?? '').trim().toLowerCase() === 'auto'
}
