import { onBeforeUnmount, onMounted } from 'vue'

// One table, so the keys that work and the keys the tooltips mention cannot drift apart.
export const keys = {
  fetch: 'Ctrl+Shift+F',
  pull: 'Ctrl+Shift+L',
  push: 'Ctrl+Shift+P',
  branch: 'Ctrl+Shift+B',
  stash: 'Ctrl+Shift+S',
  history: 'Ctrl+1',
  changes: 'Ctrl+2',
  open: 'Ctrl+O',
  closeTab: 'Ctrl+W',
  nextTab: 'Ctrl+Tab',
  prevTab: 'Ctrl+Shift+Tab',
  settings: 'Ctrl+,',
  zoomIn: 'Ctrl+=',
  zoomOut: 'Ctrl+-',
  zoomReset: 'Ctrl+0',
  refresh: 'F5',
} as const

export type Action = keyof typeof keys

/** A tooltip with its shortcut, as menus show them. */
export function withKey(text: string, action: Action): string {
  return `${text}  (${keys[action].replace('Ctrl+=', 'Ctrl++')})`
}

/** The event written the way the table writes keys; letters upper-cased, + read as =. */
function combo(e: KeyboardEvent): string {
  let key = e.key.length === 1 ? e.key.toUpperCase() : e.key
  if (key === '+') key = '='
  // Shift turns digits and punctuation into other characters on most layouts.
  if (e.ctrlKey && /^Digit\d$/.test(e.code)) key = e.code.slice(5)
  return [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', key]
    .filter(Boolean)
    .join('+')
}

const byCombo = new Map<string, Action>(
  Object.entries(keys).map(([action, k]) => [k, action as Action]),
)

/** Runs the handler for each shortcut pressed, unless a dialog has the keyboard. */
export function useShortcuts(handlers: Partial<Record<Action, () => void>>) {
  function onKey(e: KeyboardEvent) {
    const action = byCombo.get(combo(e))
    if (!action || e.defaultPrevented) return
    // A dialog owns the keyboard: Ctrl+W there should not close the tab behind it.
    if (document.querySelector('[role="dialog"]')) return
    const run = handlers[action]
    if (!run) return
    e.preventDefault()
    run()
  }
  onMounted(() => window.addEventListener('keydown', onKey))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
}
