// Wails flattens a Go error into a string, so the sentinel text is the contract.

const NOT_MERGED = 'branch is not fully merged'
const WOULD_OVERWRITE = 'local changes would be overwritten'
const NO_UPSTREAM = 'branch has no upstream'
const REJECTED = 'push rejected'
const STALE_INFO = 'remote has changed since the last fetch'
const AUTH = 'authentication failed'
const UNKNOWN_HOST = 'the server is not in known_hosts'

/** Strips the "git <args>: exit status N:" prefix git errors carry. */
export function errorText(e: unknown): string {
  const s = String(e).replace(/^Error:\s*/, '')
  // Lazy, because an argument can hold colons of its own: a URL does.
  return s.replace(/^git .*?: exit status \d+:\s*/, '')
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

/** True when ssh does not know the server, which only the user can settle. */
export function isUnknownHost(e: unknown): boolean {
  return String(e).includes(UNKNOWN_HOST)
}

/** The host out of an SSH or HTTPS remote URL, for telling the user what to trust. */
export function remoteHost(url: string): string {
  const ssh = url.match(/^[^@]+@([^:/]+)/)
  if (ssh) return ssh[1]
  const https = url.match(/^https?:\/\/([^/]+)/)
  return https ? https[1] : ''
}
