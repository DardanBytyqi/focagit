<script setup lang="ts">
import type { git } from '@wails/go/models'
import type { TreeNode } from '@/utils/tree'

defineProps<{
  nodes: TreeNode[]
  depth: number
  expanded: Set<string>
  selectedPath: string
  staged: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  /** A row was selected. Directories are selectable too: acting on one acts
   *  on everything beneath it. */
  pick: [node: TreeNode]
  toggleDir: [path: string]
  action: [node: TreeNode]
  discard: [node: TreeNode]
}>()

/** The single character in the gutter. */
function mark(f: git.FileStatus, staged: boolean): string {
  if (f.untracked) return '+'
  return (staged ? f.staged : f.unstaged) || '·'
}

/** CSS-safe name for the mark, since "+" and "?" cannot be class suffixes. */
function markClass(f: git.FileStatus, staged: boolean): string {
  if (f.untracked) return 'm-new'
  switch (staged ? f.staged : f.unstaged) {
    case 'A':
      return 'm-add'
    case 'M':
      return 'm-mod'
    case 'D':
      return 'm-del'
    case 'R':
    case 'C':
      return 'm-move'
    case 'U':
      return 'm-conflict'
    default:
      return 'm-none'
  }
}
</script>

<template>
  <template v-for="n in nodes" :key="n.path">
    <div
      v-if="n.dir"
      class="row dir"
      :class="{ selected: n.path === selectedPath }"
      :style="{ paddingLeft: 6 + depth * 13 + 'px' }"
      @click="emit('pick', n)"
    >
      <!-- The chevron folds the directory; the row selects it. Conflating the
           two makes it impossible to act on a directory without also
           collapsing the thing you are looking at. -->
      <span
        class="chevron"
        :class="{ closed: !expanded.has(n.path) }"
        @click.stop="emit('toggleDir', n.path)"
        ><svg viewBox="0 0 12 12"><path d="M3 4.5L6 7.5 9 4.5" /></svg
      ></span>
      <span class="name" :class="{ tail: n.name.includes('/') }" :title="n.path">
        <span dir="ltr">{{ n.name }}</span>
      </span>
      <span class="actions">
        <button
          class="tiny"
          :title="staged ? 'Unstage' : 'Stage'"
          :disabled="busy"
          @click.stop="emit('action', n)"
        >
          <template v-if="staged"
            ><svg class="glyph" viewBox="0 0 12 12"><path d="M2.5 6h7" /></svg
          ></template>
          <template v-else
            ><svg class="glyph" viewBox="0 0 12 12"><path d="M6 2.5v7M2.5 6h7" /></svg
          ></template>
        </button>
        <button
          v-if="!staged"
          class="tiny danger"
          :disabled="busy"
          title="Discard changes"
          @click.stop="emit('discard', n)"
        >
          <svg class="glyph" viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" /></svg>
        </button>
      </span>
    </div>

    <FileTree
      v-if="n.dir && expanded.has(n.path)"
      :nodes="n.children"
      :depth="depth + 1"
      :expanded="expanded"
      :selected-path="selectedPath"
      :staged="staged"
      :busy="busy"
      @pick="emit('pick', $event)"
      @toggle-dir="emit('toggleDir', $event)"
      @action="emit('action', $event)"
      @discard="emit('discard', $event)"
    />

    <div
      v-if="!n.dir && n.file"
      class="row file"
      :class="{ selected: n.path === selectedPath }"
      :style="{ paddingLeft: 6 + depth * 13 + 'px' }"
      @click="emit('pick', n)"
      @dblclick="emit('action', n)"
    >
      <span class="mark" :class="markClass(n.file, staged)">{{ mark(n.file, staged) }}</span>
      <span class="name" :title="n.path">{{ n.name }}</span>
      <span class="actions">
        <button
          class="tiny"
          :title="staged ? 'Unstage' : 'Stage'"
          :disabled="busy"
          @click.stop="emit('action', n)"
        >
          <template v-if="staged"
            ><svg class="glyph" viewBox="0 0 12 12"><path d="M2.5 6h7" /></svg
          ></template>
          <template v-else
            ><svg class="glyph" viewBox="0 0 12 12"><path d="M6 2.5v7M2.5 6h7" /></svg
          ></template>
        </button>
        <button
          v-if="!staged"
          class="tiny danger"
          :disabled="busy"
          title="Discard changes"
          @click.stop="emit('discard', n)"
        >
          <svg class="glyph" viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" /></svg>
        </button>
      </span>
    </div>
  </template>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding-right: 8px;
  border-radius: var(--radius-sm);
  font-size: var(--fs-small);
  white-space: nowrap;
  overflow: hidden;
  cursor: default;
}

.row:hover {
  background: var(--bg-hover);
}

.row.selected {
  background: var(--accent-soft);
}

.chevron {
  display: grid;
  place-items: center;
  width: 14px;
  height: 14px;
  flex: none;
  color: var(--text-muted);
  transition: transform 0.1s ease;
}

.chevron svg {
  width: 12px;
  height: 12px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.chevron:hover {
  color: var(--text);
}

.chevron.closed {
  transform: rotate(-90deg);
}

.dir .name {
  color: var(--text-muted);
}

.mark {
  width: 11px;
  flex: none;
  font-family: var(--font-mono);
  font-weight: 700;
  text-align: center;
}

.m-new {
  color: var(--green);
}

.m-add {
  color: var(--green);
}

.m-mod {
  color: var(--amber);
}

.m-del {
  color: var(--red);
}

.m-move {
  color: var(--blue);
}

.m-conflict {
  color: var(--red);
}

.m-none {
  color: var(--text-dim);
}

.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.actions {
  display: none;
  gap: 2px;
  flex: none;
}

.row:hover .actions {
  display: flex;
}

/* Drawn glyphs: + − × as characters sit off-centre and change with the font. */
.tiny {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  padding: 0;
  color: var(--text-muted);
  background: var(--bg-raised);
  border-color: var(--border-strong);
}

.tiny:hover:not(:disabled) {
  color: var(--text);
  background: var(--bg-hover);
}

.glyph {
  width: 12px;
  height: 12px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.6;
  stroke-linecap: round;
}

.tiny.danger:hover:not(:disabled) {
  color: var(--red);
  border-color: var(--red);
}

/* A folded path keeps its deepest folder: the ellipsis eats the start instead. */
.name.tail {
  direction: rtl;
  text-align: left;
}
</style>
