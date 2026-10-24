// Разбор startapp-параметра occ_<hex32> в UUID.

const RE = /^occ_([0-9a-fA-F]{32})$/

export function parseStartParam(s: string | null | undefined): { occId: string } | null {
  if (!s) return null
  const m = RE.exec(s)
  if (!m) return null
  const hex = m[1]!.toLowerCase()
  const uuid = `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`
  return { occId: uuid }
}
