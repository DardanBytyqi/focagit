import type { patch } from '@wails/go/models'

// Selected diff lines are held in a Set, which needs a primitive key.
export function lineKey(hunk: number, line: number): string {
  return `${hunk}:${line}`
}

export function parseLineKey(key: string): patch.LineRef {
  const [hunk, line] = key.split(':')
  return { hunk: Number(hunk), line: Number(line) }
}

/** Decodes a whole selection into the shape internal/patch expects. */
export function lineRefs(keys: Iterable<string>): patch.LineRef[] {
  return Array.from(keys, parseLineKey)
}
