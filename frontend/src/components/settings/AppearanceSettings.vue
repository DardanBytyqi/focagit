<script setup lang="ts">
import { SliderRange, SliderRoot, SliderThumb, SliderTrack } from 'reka-ui'
import { useAppearance } from '@/composables/useAppearance'

const { state, setAppearance } = useAppearance()

const themes = [
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
  { value: 'system', label: 'Follow the desktop' },
]

/** Every control writes the whole object, so there is one path into the store. */
function set<K extends keyof typeof state.look>(key: K, value: (typeof state.look)[K]) {
  setAppearance({ ...state.look, [key]: value })
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

    <label>Text size — {{ state.look.fontSize }}px</label>
    <SliderRoot
      class="slider"
      :model-value="[state.look.fontSize]"
      :min="10"
      :max="20"
      :step="1"
      @update:model-value="(v) => v && set('fontSize', v[0])"
    >
      <SliderTrack class="track">
        <SliderRange class="range" />
      </SliderTrack>
      <SliderThumb class="thumb" />
    </SliderRoot>

    <!-- Changes are live and stored as you make them; there is nothing to confirm. -->
    <p class="hint">Every change applies at once and is remembered.</p>
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

.segmented {
  display: flex;
  border: 1px solid var(--border-strong);
  border-radius: 5px;
  overflow: hidden;
}

.segmented button {
  flex: 1;
  padding: 5px 8px;
  border: none;
  border-radius: 0;
  color: var(--text-muted);
  background: var(--bg);
  font-size: var(--fs-small);
}

.segmented button + button {
  border-left: 1px solid var(--border);
}

.segmented button:hover {
  background: var(--bg-hover);
  border-color: var(--border);
}

.segmented button.on {
  background: var(--accent);
  color: #fff;
}

.slider {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
  height: 18px;
  touch-action: none;
}

.track {
  position: relative;
  flex: 1;
  height: 4px;
  background: var(--border-strong);
  border-radius: 2px;
}

.range {
  position: absolute;
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
}

.thumb {
  display: block;
  width: 13px;
  height: 13px;
  background: var(--text);
  border: 1px solid var(--bg);
  border-radius: 50%;
  outline: none;
}

.thumb:hover,
.thumb:focus-visible {
  background: var(--accent);
}
</style>
