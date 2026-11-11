<script setup lang="ts">
import { computed } from 'vue'
import {
  SelectContent,
  SelectItem,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
} from 'reka-ui'

const props = defineProps<{
  /** dot is a colour, for choices that carry a warning of their own; font previews the choice itself. */
  options: { value: string; label: string; hint?: string; dot?: string; font?: string }[]
  placeholder?: string
}>()

const value = defineModel<string>({ required: true })

// The trigger shows only the text, so the dot has to be looked up separately.
const current = computed(() => props.options.find((o) => o.value === value.value))
</script>

<template>
  <SelectRoot v-model="value">
    <SelectTrigger class="trigger">
      <span v-if="current?.dot" class="dot" :style="{ background: current.dot }"></span>
      <SelectValue
        :placeholder="placeholder ?? ''"
        :style="current?.font ? { fontFamily: current.font } : undefined"
      />
      <span class="chevron">▾</span>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent class="content" position="popper" :side-offset="4">
        <SelectViewport>
          <SelectItem v-for="o in options" :key="o.value" :value="o.value" class="item">
            <span v-if="o.dot" class="dot" :style="{ background: o.dot }"></span>
            <SelectItemText :style="o.font ? { fontFamily: o.font } : undefined">{{
              o.label
            }}</SelectItemText>
            <span v-if="o.hint" class="hint mono">{{ o.hint }}</span>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>

<style scoped>
.trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 8px;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  font-size: var(--fs);
  text-align: left;
}

.trigger:hover,
.trigger[data-state='open'] {
  background: var(--bg);
  border-color: var(--accent);
}

.chevron {
  margin-left: auto;
  color: var(--text-dim);
  font-size: 10px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
}
</style>

<style>
/* Not scoped: the list is teleported out of this component's subtree. */
/* Exactly the trigger's width: a wider list reads as a misplaced popup. */
.content {
  width: var(--reka-select-trigger-width);
  max-height: 40vh;
  padding: 4px;
  background: var(--bg-raised);
  border: 1px solid var(--border-strong);
  border-radius: 5px;
  box-shadow: 0 8px 24px var(--shadow);
  z-index: var(--z-menu);
}

.content .item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 3px;
  font-size: var(--fs-small);
  cursor: default;
  outline: none;
  overflow: hidden;
}

.content .item[data-highlighted] {
  background: var(--bg-selected);
}

.content .item .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
  align-self: center;
}

.content .item .hint {
  margin: 0 0 0 auto;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 55%;
}
</style>
