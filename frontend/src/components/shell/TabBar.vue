<script setup lang="ts">
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import type { session } from '@wails/go/models'

defineProps<{
  tabs: session.Info[]
  activeId: string
  busy: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  close: [id: string]
  open: []
  clone: []
}>()
</script>

<template>
  <div v-if="tabs.length" class="tabs">
    <div
      v-for="t in tabs"
      :key="t.id"
      class="tab"
      :class="{ active: t.id === activeId }"
      :title="t.path"
      @click="emit('select', t.id)"
      @auxclick.middle="emit('close', t.id)"
    >
      <span class="name">{{ t.name }}</span>
      <span class="close" title="Close" @click.stop="emit('close', t.id)">
        <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.4">
          <path d="M3.5 3.5l5 5M8.5 3.5l-5 5" stroke-linecap="round" />
        </svg>
      </span>
    </div>

    <DropdownMenuRoot>
      <DropdownMenuTrigger class="add" title="Open another repository">
        <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.4">
          <path d="M6 2.5v7M2.5 6h7" stroke-linecap="round" />
        </svg>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent class="menu" align="start" :side-offset="2">
          <DropdownMenuItem class="menu-item" @select="emit('open')">Open…</DropdownMenuItem>
          <DropdownMenuItem class="menu-item" :disabled="busy" @select="emit('clone')">
            Clone…
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  flex: none;
  padding: 5px 6px 0;
  overflow-x: auto;
  background: var(--bg);
  scrollbar-width: none;
}

.tabs::-webkit-scrollbar {
  display: none;
}

.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  min-width: 96px;
  max-width: 190px;
  height: 28px;
  padding: 0 7px 0 11px;
  color: var(--text-dim);
  font-size: var(--fs);
  cursor: default;
  border-radius: 7px 7px 0 0;
  /* Transparent, so only the active tab carries the toolbar's colour. */
  background: transparent;
  transition: background 0.12s ease;
}

.tab:hover {
  color: var(--text-muted);
  background: var(--bg-hover);
}

/* The active tab takes the toolbar's background, so the two read as one surface. */
.tab.active {
  color: var(--text);
  background: var(--bg-toolbar);
  box-shadow: inset 0 2px 0 var(--accent);
}

.name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Always laid out, only faded, so the label never shifts when it appears. */
.close {
  flex: none;
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  opacity: 0;
}

.close svg {
  width: 12px;
  height: 12px;
  display: block;
}

.tab:hover .close,
.tab.active .close {
  opacity: 0.55;
}

.close:hover {
  background: var(--border-strong);
  opacity: 1;
}

.add {
  flex: none;
  display: grid;
  place-items: center;
  /* Square, and nudged up so it sits centred against the tabs, not on their baseline. */
  width: 26px;
  height: 26px;
  /* The global button rule pads 4px 10px, which would crush the icon off-centre. */
  padding: 0;
  margin: 0 0 1px 3px;
  color: var(--text-dim);
  cursor: default;
  background: none;
  border: 0;
  border-radius: 6px;
}

.add svg {
  width: 12px;
  height: 12px;
  display: block;
}

.add:hover,
.add[data-state='open'] {
  color: var(--text);
  background: var(--bg-hover);
}
</style>
