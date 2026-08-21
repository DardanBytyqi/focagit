<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from 'reka-ui'
import type { git } from '@wails/go/models'

const props = defineProps<{
  refs: git.Ref[]
  stashes: git.Stash[]
  changeCount: number
  /** Stash is a mode too, so no nav item is highlighted while one is shown. */
  mode: 'commit' | 'changes' | 'stash'
  /** The branch a merge would land on; empty when HEAD is detached. */
  head: string
}>()

const emit = defineEmits<{
  mode: [m: 'commit' | 'changes']
  reveal: [r: git.Ref]
  checkout: [r: git.Ref]
  merge: [r: git.Ref]
  mergeDrop: [source: git.Ref, target: git.Ref, x: number, y: number]
  create: [start: string, label: string]
  rename: [name: string]
  remove: [name: string]
  addRemote: []
  showStash: [s: git.Stash]
  applyStash: [s: git.Stash]
  popStash: [s: git.Stash]
  dropStash: [s: git.Stash]
}>()

/** The ref under the pointer during a drag, and the row it is over. */
const dragged = ref<git.Ref | null>(null)
const over = ref('')

// Only a local branch can receive a merge: it is the one that moves.
function canDrop(target: git.Ref) {
  const src = dragged.value
  return !!src && target.kind === 'branch' && target.name !== src.name
}

function onDragStart(e: DragEvent, r: git.Ref) {
  if (!canCheckout(r)) {
    e.preventDefault()
    return
  }
  dragged.value = r
  if (!e.dataTransfer) return
  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', r.short)
  e.dataTransfer.setDragImage(dragChip(r.short), 14, 14)
}

// Otherwise the browser drags a photograph of the row, cut off at the edge.
function dragChip(label: string) {
  const chip = document.createElement('div')
  chip.className = 'drag-chip'
  chip.textContent = label
  document.body.appendChild(chip)
  // It has to be in the document to be photographed, and gone right after.
  setTimeout(() => chip.remove(), 0)
  return chip
}

function onDragOver(e: DragEvent, r: git.Ref) {
  if (!canDrop(r)) return
  // Without preventDefault the browser refuses the drop and shows no cursor.
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
  over.value = r.name
}

function onDrop(e: DragEvent, r: git.Ref) {
  if (!canDrop(r)) return
  e.preventDefault()
  // The menu opens where the pointer let go, not in the middle of the window.
  emit('mergeDrop', dragged.value as git.Ref, r, e.clientX, e.clientY)
  endDrag()
}

function endDrag() {
  dragged.value = null
  over.value = ''
}

// A merge needs a branch to merge and a branch to merge into.
function canMerge(r: git.Ref) {
  return !r.head && !!props.head && (r.kind === 'branch' || r.kind === 'remote')
}

// Only branches can be checked out directly; a tag would detach HEAD.
function canCheckout(r: git.Ref) {
  return r.kind === 'branch' || r.kind === 'remote'
}

/** Sections mirror Fork's sidebar. */
const sections = computed(() => [
  { key: 'branch', title: 'Branches', refs: props.refs.filter((r) => r.kind === 'branch') },
  { key: 'remote', title: 'Remotes', refs: props.refs.filter((r) => r.kind === 'remote') },
  { key: 'tag', title: 'Tags', refs: props.refs.filter((r) => r.kind === 'tag') },
])

const collapsed = ref<Record<string, boolean>>({})

function toggle(key: string) {
  collapsed.value[key] = !collapsed.value[key]
}
</script>

<template>
  <aside class="sidebar">
    <!-- Working tree and history are two views, not a row inside one. -->
    <nav class="nav">
      <div
        class="nav-item"
        :class="{ active: mode === 'changes' }"
        @click="emit('mode', 'changes')"
      >
        <span class="nav-icon">◧</span>
        <span>Changes</span>
        <span v-if="changeCount" class="pill">{{ changeCount }}</span>
      </div>
      <div class="nav-item" :class="{ active: mode === 'commit' }" @click="emit('mode', 'commit')">
        <span class="nav-icon">≡</span>
        <span>All Commits</span>
      </div>
    </nav>

    <div v-for="s in sections" :key="s.key" class="section">
      <div class="section-header" @click="toggle(s.key)">
        <span class="chevron" :class="{ closed: collapsed[s.key] }">▾</span>
        {{ s.title }}
        <span class="count muted">{{ s.refs.length }}</span>
        <button
          v-if="s.key === 'remote'"
          class="add"
          title="Add a remote"
          @click.stop="emit('addRemote')"
        >
          +
        </button>
      </div>

      <div v-show="!collapsed[s.key]">
        <ContextMenuRoot v-for="r in s.refs" :key="r.name">
          <ContextMenuTrigger as-child>
            <div
              class="ref"
              :class="{ head: r.head, target: over === r.name, dragging: dragged?.name === r.name }"
              :draggable="canCheckout(r)"
              :title="dragged && canDrop(r) ? `Merge ${dragged.short} into ${r.short}` : ''"
              @click="emit('reveal', r)"
              @dblclick="canCheckout(r) && emit('checkout', r)"
              @dragstart="onDragStart($event, r)"
              @dragend="endDrag"
              @dragover="onDragOver($event, r)"
              @dragleave="over === r.name && (over = '')"
              @drop="onDrop($event, r)"
            >
              <span class="ref-name">{{ r.short }}</span>
              <span v-if="r.upstream" class="upstream muted mono">{{ r.upstream }}</span>
            </div>
          </ContextMenuTrigger>

          <ContextMenuPortal>
            <ContextMenuContent class="menu">
              <ContextMenuItem
                v-if="canCheckout(r)"
                class="menu-item"
                :disabled="r.head"
                @select="emit('checkout', r)"
              >
                Checkout {{ r.short }}
              </ContextMenuItem>

              <!-- Merging a branch into itself is the one thing it cannot do. -->
              <ContextMenuItem
                v-if="canMerge(r)"
                class="menu-item"
                @select="emit('merge', r)"
              >
                Merge {{ r.short }} into {{ head }}
              </ContextMenuItem>

              <ContextMenuItem class="menu-item" @select="emit('create', r.short, r.short)">
                New branch from {{ r.short }}…
              </ContextMenuItem>

              <template v-if="r.kind === 'branch'">
                <ContextMenuSeparator class="menu-sep" />
                <ContextMenuItem class="menu-item" @select="emit('rename', r.short)">
                  Rename…
                </ContextMenuItem>
                <ContextMenuItem
                  class="menu-item danger"
                  :disabled="r.head"
                  @select="emit('remove', r.short)"
                >
                  Delete…
                </ContextMenuItem>
              </template>
            </ContextMenuContent>
          </ContextMenuPortal>
        </ContextMenuRoot>

        <div v-if="!s.refs.length" class="empty muted">none</div>
      </div>
    </div>

    <div class="section">
      <div class="section-header" @click="toggle('stash')">
        <span class="chevron" :class="{ closed: collapsed.stash }">▾</span>
        Stashes
        <span class="count muted">{{ stashes.length }}</span>
      </div>

      <div v-show="!collapsed.stash">
        <ContextMenuRoot v-for="st in stashes" :key="st.ref">
          <ContextMenuTrigger as-child>
            <div class="ref" :title="st.message" @click="emit('showStash', st)">
              <span class="ref-name">{{ st.message }}</span>
            </div>
          </ContextMenuTrigger>

          <ContextMenuPortal>
            <ContextMenuContent class="menu">
              <!-- Pop is the common one, so it comes first. -->
              <ContextMenuItem class="menu-item" @select="emit('popStash', st)">
                Pop
              </ContextMenuItem>
              <ContextMenuItem class="menu-item" @select="emit('applyStash', st)">
                Apply and keep
              </ContextMenuItem>
              <ContextMenuSeparator class="menu-sep" />
              <ContextMenuItem class="menu-item danger" @select="emit('dropStash', st)">
                Drop…
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenuPortal>
        </ContextMenuRoot>

        <div v-if="!stashes.length" class="empty muted">none</div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.nav {
  padding: 4px 0 8px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 7px;
  height: 24px;
  padding: 0 10px;
  cursor: default;
}

.nav-item:hover {
  background: var(--bg-hover);
}

.nav-item.active {
  background: var(--bg-selected);
}

.nav-icon {
  width: 12px;
  text-align: center;
  color: var(--text-muted);
}

/* Pushed to the right edge, and amber: uncommitted work is waiting, not new. */
.pill {
  margin-left: auto;
  background: var(--amber);
  color: var(--bg);
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-raised);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 6px 0;
  flex: none;
}

.section {
  margin-bottom: 10px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  font-size: var(--fs-small);
  font-weight: 600;
  color: var(--text-muted);
  cursor: default;
}

/* Pushed to the right edge, and only there when the pointer is. */
.section-header .add {
  margin-left: auto;
  padding: 0 5px;
  line-height: 1;
  color: var(--text-dim);
  visibility: hidden;
}

.section-header:hover .add {
  visibility: visible;
}

.chevron {
  display: inline-block;
  transition: transform 0.12s ease;
  font-size: 10px;
}

.chevron.closed {
  transform: rotate(-90deg);
}

.count {
  margin-left: auto;
  font-weight: 400;
}

.ref {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 2px 10px 2px 26px;
  height: 22px;
  cursor: default;
  overflow: hidden;
  white-space: nowrap;
}

.ref:hover {
  background: var(--bg-hover);
}

/* Only the rows that can actually be dragged say so under the pointer. */
.ref[draggable='true']:active {
  cursor: grabbing;
}

/* The row being carried stays visible but clearly no longer in place. */
.ref.dragging {
  opacity: 0.45;
}

/* Where the drop would land: an outline, so the name stays readable. */
.ref.target {
  background: var(--bg-selected);
  outline: 1px solid var(--accent);
  outline-offset: -1px;
}

.ref.head .ref-name {
  font-weight: 600;
  color: var(--green);
}

/* The name wins the space: a long upstream would push it out of the row. */
.ref-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.upstream {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 45%;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11px;
}

.empty {
  padding: 2px 10px 2px 26px;
  font-size: var(--fs-small);
  font-style: italic;
}
</style>
