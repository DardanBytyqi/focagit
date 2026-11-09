import '@fontsource-variable/red-hat-text'
import '@fontsource-variable/red-hat-mono'
import '@fontsource-variable/inter'
import '@fontsource/space-mono/400.css'
import '@fontsource/ibm-plex-mono/400.css'
import { createApp } from 'vue'
import App from './App.vue'
import { installErrorOverlay, suppressNativeMenus } from './boot'
import './style.css'

const app = createApp(App)

installErrorOverlay(app)
suppressNativeMenus()

app.mount('#app')
