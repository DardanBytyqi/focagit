import { reactive } from 'vue'
import type { prefs } from '@wails/go/models'
import { GetAppearance, SaveAppearance } from '@wails/go/main/App'

/** The default has to exist here too: it is what the window shows before Go answers. */
const fallback: prefs.Appearance = { theme: 'dark', density: 'normal', fontSize: 13 }

const state = reactive<{ look: prefs.Appearance }>({ look: { ...fallback } })

/** Follows the desktop when the theme is "system". */
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)')

function apply() {
  const root = document.documentElement
  const theme =
    state.look.theme === 'system' ? (prefersDark.matches ? 'dark' : 'light') : state.look.theme

  // The dark palette is the bare :root block, so it needs no attribute of its own.
  if (theme === 'light') root.setAttribute('data-theme', 'light')
  else root.removeAttribute('data-theme')

  root.setAttribute('data-density', state.look.density)
  root.style.setProperty('--fs', `${state.look.fontSize}px`)
  root.style.setProperty('--fs-small', `${state.look.fontSize - 1}px`)
}

// A desktop that switches to dark at sunset should take the window with it.
prefersDark.addEventListener('change', () => {
  if (state.look.theme === 'system') apply()
})

async function loadAppearance() {
  try {
    state.look = await GetAppearance()
  } catch {
    // Preferences are a convenience; the window still has to open.
    state.look = { ...fallback }
  }
  apply()
}

/** Shows the change at once, then stores whatever Go accepted. */
async function setAppearance(look: prefs.Appearance) {
  state.look = look
  apply()
  try {
    state.look = await SaveAppearance(look)
  } catch {
    // Keep showing what was asked for; only the storing failed.
  }
  apply()
}

export function useAppearance() {
  return { state, loadAppearance, setAppearance }
}
