<script setup lang="ts">
import { useAppearance } from '@/composables/useAppearance'

const { state, fallback, setAppearance } = useAppearance()

/** The range Go accepts; anything outside it is stored as the default. */
const MIN = 10
const MAX = 20

const themes = [
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
  { value: 'system', label: 'System' },
]

/** Every control writes the whole object, so there is one path into the store. */
function set<K extends keyof typeof state.look>(key: K, value: (typeof state.look)[K]) {
  setAppearance({ ...state.look, [key]: value })
}

function stepSize(by: number) {
  set('fontSize', Math.min(MAX, Math.max(MIN, state.look.fontSize + by)))
}
</script>

<template>
  <div class="pane">
    <label>Theme</label>
    <div class="segmented">
      <button
        v-for="t in themes"
        :key="t.value"
        :class="{ on: state.look.theme === t.value }"
        @click="set('theme', t.value)"
      >
        {{ t.label }}
      </button>
    </div>

    <label>Text size</label>
    <!-- One pixel per click: a slider made every size a guess. -->
    <div class="size">
      <div class="stepper">
        <button
          type="button"
          title="Smaller"
          :disabled="state.look.fontSize <= MIN"
          @click="stepSize(-1)"
        >
          −
        </button>
        <span class="value">{{ state.look.fontSize }} px</span>
        <button
          type="button"
          title="Larger"
          :disabled="state.look.fontSize >= MAX"
          @click="stepSize(1)"
        >
          +
        </button>
      </div>
      <button
        type="button"
        class="reset"
        :disabled="state.look.fontSize === fallback.fontSize"
        @click="set('fontSize', fallback.fontSize)"
      >
        Default ({{ fallback.fontSize }} px)
      </button>
    </div>
  </div>
</template>

<style scoped>
.pane label {
  display: block;
  margin: 14px 0 5px;
  font-size: var(--fs-small);
  color: var(--text-muted);
}

.pane label:first-child {
  margin-top: 0;
}

/* A track with the chosen option raised out of it. */
.segmented {
  display: flex;
  gap: 2px;
  padding: 3px;
  background: var(--bg-shell);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.segmented button {
  flex: 1;
  height: 30px;
  padding: 0 10px;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  background: none;
  font-size: var(--fs-small);
}

.segmented button:hover:not(.on) {
  background: var(--bg-hover);
  color: var(--text);
}

.segmented button.on {
  background: var(--bg-raised);
  color: var(--text);
  font-weight: 500;
  box-shadow:
    0 0 0 1px var(--border-strong),
    0 1px 3px var(--shadow);
}

.size {
  display: flex;
  align-items: center;
  gap: 10px;
}

.stepper {
  display: flex;
  align-items: center;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  overflow: hidden;
}

.stepper button {
  width: 34px;
  height: 32px;
  padding: 0;
  border: none;
  border-radius: 0;
  background: var(--bg);
  color: var(--text);
  font-size: var(--fs);
}

.stepper button:hover:not(:disabled) {
  background: var(--bg-hover);
}

.stepper .value {
  min-width: 56px;
  padding: 4px 8px;
  text-align: center;
  font-variant-numeric: tabular-nums;
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
}
</style>
