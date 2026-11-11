<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { type Action, comboOf, defaultKeys, useShortcutSettings } from '@/composables/useShortcuts'

const props = defineProps<{ active: boolean }>()

const { state, loadShortcuts, setBinding, resetBinding, resetAllShortcuts } = useShortcutSettings()

const labels: Record<Action, string> = {
  fetch: 'Fetch',
  pull: 'Pull',
  push: 'Push',
  branch: 'New branch',
  stash: 'Stash changes',
  history: 'History view',
  changes: 'Changes view',
  open: 'Open repository',
  closeTab: 'Close tab',
  nextTab: 'Next tab',
  prevTab: 'Previous tab',
  settings: 'Settings',
  zoomIn: 'Zoom in',
  zoomOut: 'Zoom out',
  zoomReset: 'Reset zoom',
  refresh: 'Refresh',
}
const actions = Object.keys(labels) as Action[]

const listening = ref<Action | null>(null)

watch(
  () => props.active,
  (active) => {
    if (active) void loadShortcuts()
    else listening.value = null
  },
  { immediate: true },
)

function startListening(action: Action) {
  listening.value = action
}

function onCaptureKey(e: KeyboardEvent) {
  const action = listening.value
  if (!action) return
  e.preventDefault()
  e.stopPropagation()
  if (e.key === 'Escape') {
    listening.value = null
    return
  }
  // wait for the key the modifier is held with
  if (['Control', 'Alt', 'Shift', 'Meta'].includes(e.key)) return
  setBinding(action, comboOf(e))
  listening.value = null
}

onMounted(() => window.addEventListener('keydown', onCaptureKey, true))
onBeforeUnmount(() => window.removeEventListener('keydown', onCaptureKey, true))
</script>

<template>
  <div class="pane">
    <p class="lead">Click a shortcut, then press the new key combination. Escape cancels.</p>

    <ul class="shortcuts">
      <li v-for="action in actions" :key="action">
        <span class="who">{{ labels[action] }}</span>

        <button
          type="button"
          class="combo"
          :class="{ listening: listening === action }"
          @click="startListening(action)"
        >
          {{ listening === action ? 'Press a key…' : state.bindings[action] || 'None' }}
        </button>

        <button
          type="button"
          class="reset"
          title="Reset to the default"
          :disabled="state.bindings[action] === defaultKeys[action]"
          @click="resetBinding(action)"
        >
          ×
        </button>
      </li>
    </ul>

    <button type="button" class="add-link" @click="resetAllShortcuts">Reset all to defaults</button>
  </div>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

.shortcuts {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 5px;
  max-height: 320px;
  overflow-y: auto;
}

.shortcuts li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 9px;
}

.shortcuts li + li {
  border-top: 1px solid var(--border);
}

.who {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.combo {
  flex: none;
  min-width: 140px;
  padding: 3px 8px;
  font-family: var(--font-mono);
  font-size: var(--fs-small);
  color: var(--text-muted);
  background: var(--bg);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
}

.combo.listening {
  color: var(--text);
  border-color: var(--blue);
}

.reset {
  flex: none;
  padding: 0 6px;
  color: var(--text-dim);
}

.add-link {
  margin-top: 8px;
  padding: 4px 0;
  color: var(--blue);
  background: none;
  border: none;
}

.add-link:hover {
  background: none;
  border: none;
  text-decoration: underline;
}
</style>
