import { createApp } from 'vue'
import App from './App.vue'
import { installErrorOverlay, suppressNativeMenus } from './boot'
import './style.css'

const app = createApp(App)

installErrorOverlay(app)
suppressNativeMenus()

app.mount('#app')
