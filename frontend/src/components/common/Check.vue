<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

defineProps<{ disabled?: boolean }>()

const checked = defineModel<boolean>({ required: true })
</script>

<template>
  <!-- The whole row is the control, so the text toggles it like the box does. -->
  <CheckboxRoot v-model="checked" :disabled="disabled" class="checkbox">
    <span class="box">
      <CheckboxIndicator class="mark">
        <svg viewBox="0 0 14 14" aria-hidden="true">
          <path d="M2.5 7.2 5.6 10.2 11.5 4" />
        </svg>
      </CheckboxIndicator>
    </span>
    <span class="text"><slot /></span>
  </CheckboxRoot>
</template>

<style scoped>
/* Block, not inline: two of these in a row belong on two rows. */
.checkbox {
  display: flex;
  width: fit-content;
  max-width: 100%;
  align-items: center;
  gap: 7px;
  padding: 2px 0;
  border: none;
  background: none;
  font-size: var(--fs-small);
  text-align: left;
}

.checkbox + .checkbox {
  margin-top: 6px;
}

.checkbox:hover:not(:disabled) {
  background: none;
  border: none;
}

.checkbox:disabled {
  color: var(--text-dim);
}

.box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 15px;
  height: 15px;
  flex: none;
  background: var(--bg);
  border: 1px solid var(--border-strong);
  border-radius: 3px;
  transition: background 90ms, border-color 90ms;
}

.checkbox:hover:not(:disabled) .box {
  border-color: var(--accent);
}

.checkbox[data-state='checked'] .box {
  background: var(--accent);
  border-color: var(--accent);
}

.checkbox:disabled .box {
  border-color: var(--border);
}

.checkbox:focus-visible .box {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.mark {
  display: flex;
}

.mark svg {
  width: 11px;
  height: 11px;
  fill: none;
  stroke: #fff;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
