import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      // Depth-independent imports.
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      // The Go bindings are generated, live outside src/, and are imported from everywhere.
      '@wails': fileURLToPath(new URL('./wailsjs', import.meta.url)),
    },
  },
})
