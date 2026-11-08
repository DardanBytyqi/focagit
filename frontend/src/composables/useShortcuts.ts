import { onBeforeUnmount, onMounted, reactive } from 'vue'
import { GetShortcuts, SaveShortcuts } from '@wails/go/main/App'

// One table, so the keys that work and the keys the tooltips mention cannot drift apart.
export const defaultKeys = {
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

export type Action = keyof typeof defaultKeys

const state = reactive<{ bindings: Record<Action, string> }>({ bindings: { ...defaultKeys } })

function byCombo(): Map<string, Action> {
  return new Map(
    (Object.entries(state.bindings) as [Action, string][])
      .filter(([, combo]) => combo)
      .map(([action, combo]) => [combo, action]),
  )
}

/** A tooltip with its shortcut, as menus show them. */
export function withKey(text: string, action: Action): string {
  const combo = state.bindings[action]
  return combo ? `${text}  (${combo.replace('Ctrl+=', 'Ctrl++')})` : text
}

/** The event written the way the table writes keys; letters upper-cased, + read as =. */
export function comboOf(e: KeyboardEvent): string {
  let key = e.key.length === 1 ? e.key.toUpperCase() : e.key
  if (key === '+') key = '='
  // Shift turns digits and punctuation into other characters on most layouts.
  if (e.ctrlKey && /^Digit\d$/.test(e.code)) key = e.code.slice(5)
  return [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', key]
    .filter(Boolean)
    .join('+')
}

function diffFromDefault(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const action of Object.keys(state.bindings) as Action[]) {
    if (state.bindings[action] !== defaultKeys[action]) out[action] = state.bindings[action]
  }
  return out
}

async function persist() {
  try {
    await SaveShortcuts(diffFromDefault())
  } catch {
    // ignored: the binding still works for this session
  }
}

async function loadShortcuts() {
  try {
    const saved = await GetShortcuts()
    state.bindings = { ...defaultKeys, ...saved } as Record<Action, string>
  } catch {
    state.bindings = { ...defaultKeys }
  }
}

/** Clears combo from whichever action already held it. */
async function setBinding(action: Action, combo: string) {
  const next = { ...state.bindings, [action]: combo }
  for (const other of Object.keys(next) as Action[]) {
    if (other !== action && next[other] === combo) next[other] = ''
  }
  state.bindings = next
  await persist()
}

async function resetBinding(action: Action) {
  state.bindings = { ...state.bindings, [action]: defaultKeys[action] }
  await persist()
}

async function resetAllShortcuts() {
  state.bindings = { ...defaultKeys }
  await persist()
}

export function useShortcutSettings() {
  return { state, defaultKeys, loadShortcuts, setBinding, resetBinding, resetAllShortcuts }
}

/** Runs the handler for each shortcut pressed, unless a dialog has the keyboard. */
export function useShortcuts(handlers: Partial<Record<Action, () => void>>) {
  function onKey(e: KeyboardEvent) {
    const action = byCombo().get(comboOf(e))
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
