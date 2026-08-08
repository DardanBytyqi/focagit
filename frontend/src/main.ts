import { createApp } from 'vue'
import App from './App.vue'
import './style.css'

const app = createApp(App)

// A thrown error inside a component leaves Vue with nothing to render.
function showFatal(where: string, err: unknown) {
  const message = err instanceof Error ? `${err.message}\n\n${err.stack ?? ''}` : String(err)
  console.error(`[focagit] ${where}:`, err)

  let bar = document.getElementById('focagit-error')
  if (!bar) {
    bar = document.createElement('div')
    bar.id = 'focagit-error'
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

app.config.errorHandler = (err, _instance, info) => showFatal(`Vue error (${info})`, err)
window.addEventListener('error', (e) => showFatal('Uncaught error', e.error ?? e.message))
window.addEventListener('unhandledrejection', (e) => showFatal('Unhandled rejection', e.reason))

app.mount('#app')
