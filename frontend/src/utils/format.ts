// Go's time.Time crosses the bridge as an RFC 3339 string.

const time = new Intl.DateTimeFormat(undefined, {
  hour: '2-digit',
  minute: '2-digit',
})

const sameYear = new Intl.DateTimeFormat(undefined, {
  day: '2-digit',
  month: 'short',
})

const other = new Intl.DateTimeFormat(undefined, {
  day: '2-digit',
  month: 'short',
  year: 'numeric',
})

// Commit dates in a dense list.
export function formatDate(value: unknown): string {
  if (!value) return ''
  const d = new Date(value as string)
  if (Number.isNaN(d.getTime())) return ''

  const now = new Date()
  if (d.toDateString() === now.toDateString()) return time.format(d)
  if (d.getFullYear() === now.getFullYear()) return sameYear.format(d)
  return other.format(d)
}
