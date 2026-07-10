// Deep-merge helper for layering fork-specific locale keys (./fork.ts) over the
// upstream modular locale base without clobbering sibling keys in shared
// namespaces (e.g. `admin`). Plain-object values merge recursively; everything
// else (strings, arrays) is overwritten by the override side.
type AnyRecord = Record<string, unknown>

function isPlainObject(v: unknown): v is AnyRecord {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

export function mergeLocale<T extends AnyRecord>(base: T, override: AnyRecord): T {
  const out: AnyRecord = { ...base }
  for (const key of Object.keys(override)) {
    const o = override[key]
    const b = out[key]
    if (isPlainObject(o) && isPlainObject(b)) {
      out[key] = mergeLocale(b, o)
    } else {
      out[key] = o
    }
  }
  return out as T
}
