/** Things the window needs set up once, before the application is mounted. */
import type { App } from 'vue'

/**
 * Shows what killed the application. Vue renders nothing after a thrown error,
 * so without this the window simply goes blank and says nothing.
 */
function showFatal(where: string, err: unknown) {
  const message = err instanceof Error ? `${err.message}\n\n${err.stack ?? ''}` : String(err)
  console.error(`[focagit] ${where}:`, err)

  let bar = document.getElementById('focagit-error')
  if (!bar) {
    bar = document.createElement('div')
    bar.id = 'focagit-error'
    // Inline, and in plain values: the stylesheet may be the thing that failed.
    bar.setAttribute(
      'style',
      [
        'position:fixed',
        'inset:auto 0 0 0',
        'z-index:9999',
        'max-height:45vh',
        'overflow:auto',
        'padding:10px 14px',
        'background:#3a1f22',
        'border-top:2px solid #e06c6c',
        'color:#f0d6d6',
        'font:12px/1.5 monospace',
        'white-space:pre-wrap',
        'user-select:text',
      ].join(';'),
    )
    document.body.appendChild(bar)
  }
  bar.textContent = `${where}\n\n${message}`
}

/** Routes every way an error can escape into the same visible place. */
export function installErrorOverlay(app: App) {
  app.config.errorHandler = (err, _instance, info) => showFatal(`Vue error (${info})`, err)
  window.addEventListener('error', (e) => showFatal('Uncaught error', e.error ?? e.message))
  window.addEventListener('unhandledrejection', (e) => showFatal('Unhandled rejection', e.reason))
}

/**
 * A WebKit menu offering "Reload" and "Inspect Element" gives away that this is
 * a webview. Ours are the only menus, except in text fields where the system one
 * is the way people paste.
 */
export function suppressNativeMenus() {
  window.addEventListener('contextmenu', (e) => {
    const el = e.target as HTMLElement | null
    if (el?.closest('input, textarea, [contenteditable="true"]')) return
    e.preventDefault()
  })
}
