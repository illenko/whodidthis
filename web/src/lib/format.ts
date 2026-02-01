export function formatNumber(n: number): string {
  return n.toLocaleString()
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

export function formatDuration(ms: number): string {
  if (!ms || isNaN(ms)) return '–'
  if (ms < 1000) return `${ms}ms`
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const rs = s % 60
  return `${m}m ${rs}s`
}
