// Wails flattens a Go error into a string, so the sentinel text in
// internal/git/branch.go is the contract between the two sides.

const NOT_MERGED = 'branch is not fully merged'
const WOULD_OVERWRITE = 'local changes would be overwritten'

/** Strips the "git <args>: exit status N:" prefix git errors carry. */
export function errorText(e: unknown): string {
  const s = String(e).replace(/^Error:\s*/, '')
  return s.replace(/^git [^:]*: exit status \d+:\s*/, '')
}

/** True when the branch still holds commits nowhere else, so -D is the way out. */
export function isNotMerged(e: unknown): boolean {
  return String(e).includes(NOT_MERGED)
}

/** True when the operation would destroy uncommitted work. */
export function isWouldOverwrite(e: unknown): boolean {
  return String(e).includes(WOULD_OVERWRITE)
}
