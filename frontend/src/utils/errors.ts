// Wails flattens a Go error into a string, so the sentinel text in
// internal/git/branch.go is the contract between the two sides.

const NOT_MERGED = 'branch is not fully merged'
const WOULD_OVERWRITE = 'local changes would be overwritten'
const NO_UPSTREAM = 'branch has no upstream'
const REJECTED = 'push rejected'
const STALE_INFO = 'remote has changed since the last fetch'
const AUTH = 'authentication failed'

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

/** True when the branch was never told which remote branch it follows. */
export function isNoUpstream(e: unknown): boolean {
  return String(e).includes(NO_UPSTREAM)
}

/** True when the remote holds commits we do not have, so pushing would lose them. */
export function isRejected(e: unknown): boolean {
  return String(e).includes(REJECTED)
}

/** True when the lease expired: the remote moved since our last fetch. */
export function isStaleInfo(e: unknown): boolean {
  return String(e).includes(STALE_INFO)
}

/** True when git could not prove who we are. */
export function isAuthFailure(e: unknown): boolean {
  return String(e).includes(AUTH)
}
