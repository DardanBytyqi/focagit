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
import RefIcon from '@/components/shell/RefIcon.vue'

const props = defineProps<{
  refs: git.Ref[]
  tags: git.Tag[]
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
  rebase: [r: git.Ref]
  mergeDrop: [source: git.Ref, target: git.Ref, x: number, y: number]
  create: [start: string, label: string]
  rename: [name: string]
  remove: [name: string]
  addRemote: []
  revealTag: [t: git.Tag]
  checkoutTag: [t: git.Tag]
  branchFromTag: [t: git.Tag]
  pushTag: [t: git.Tag]
  deleteTag: [t: git.Tag]
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
        <svg class="nav-icon" viewBox="0 0 16 16">
          <path d="M3 2.5h7l3 3v8H3z" />
          <path d="M8 6.5v4M6 8.5h4" />
        </svg>
        <span>Changes</span>
        <span v-if="changeCount" class="pill">{{ changeCount }}</span>
      </div>
      <div class="nav-item" :class="{ active: mode === 'commit' }" @click="emit('mode', 'commit')">
        <svg class="nav-icon" viewBox="0 0 16 16">
          <circle cx="5" cy="3.5" r="1.5" />
          <circle cx="5" cy="12.5" r="1.5" />
          <path d="M5 5v6M9 3.5h4.5M9 8h4.5M9 12.5h4.5" />
        </svg>
        <span>All Commits</span>
      </div>
    </nav>

    <div v-for="s in sections" :key="s.key" class="section">
      <div class="section-header" @click="toggle(s.key)">
        <span class="chevron" :class="{ closed: collapsed[s.key] }"
          ><svg viewBox="0 0 12 12"><path d="M3 4.5L6 7.5 9 4.5" /></svg
        ></span>
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
              :class="[
                r.kind,
                { head: r.head, target: over === r.name, dragging: dragged?.name === r.name },
              ]"
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
              <RefIcon :kind="r.kind === 'remote' ? 'remote' : 'branch'" />
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
              <ContextMenuItem v-if="canMerge(r)" class="menu-item" @select="emit('merge', r)">
                Merge {{ r.short }} into {{ head }}
              </ContextMenuItem>
              <ContextMenuItem v-if="canMerge(r)" class="menu-item" @select="emit('rebase', r)">
                Rebase {{ head }} on {{ r.short }}
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
      <div class="section-header" @click="toggle('tag')">
        <span class="chevron" :class="{ closed: collapsed.tag }"
          ><svg viewBox="0 0 12 12"><path d="M3 4.5L6 7.5 9 4.5" /></svg
        ></span>
        Tags
        <span class="count muted">{{ tags.length }}</span>
      </div>

      <div v-show="!collapsed.tag">
        <ContextMenuRoot v-for="t in tags" :key="t.name">
          <ContextMenuTrigger as-child>
            <div
              class="ref tag"
              :title="t.subject"
              @click="emit('revealTag', t)"
              @dblclick="emit('checkoutTag', t)"
            >
              <RefIcon kind="tag" />
              <span class="ref-name">{{ t.name }}</span>
            </div>
          </ContextMenuTrigger>

          <ContextMenuPortal>
            <ContextMenuContent class="menu">
              <ContextMenuItem class="menu-item" @select="emit('branchFromTag', t)">
                New branch from {{ t.name }}…
              </ContextMenuItem>
              <!-- A tag names a commit, not a branch, so this can only detach HEAD. -->
              <ContextMenuItem class="menu-item" @select="emit('checkoutTag', t)">
                Checkout {{ t.name }}
              </ContextMenuItem>
              <ContextMenuSeparator class="menu-sep" />
              <ContextMenuItem class="menu-item" @select="emit('pushTag', t)">
                Push to origin
              </ContextMenuItem>
              <ContextMenuItem class="menu-item danger" @select="emit('deleteTag', t)">
                Delete…
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenuPortal>
        </ContextMenuRoot>

        <div v-if="!tags.length" class="empty muted">none</div>
      </div>
    </div>

    <div class="section">
      <div class="section-header" @click="toggle('stash')">
        <span class="chevron" :class="{ closed: collapsed.stash }"
          ><svg viewBox="0 0 12 12"><path d="M3 4.5L6 7.5 9 4.5" /></svg
        ></span>
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
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 8px 10px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 10px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  height: 30px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  cursor: default;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.nav-item.active {
  background: var(--bg-selected);
  color: var(--text);
  font-weight: 500;
}

.nav-icon {
  width: 16px;
  height: 16px;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  color: var(--text-dim);
}

.nav-item.active .nav-icon {
  color: var(--text);
}

/* Amber: uncommitted work is waiting, not new. */
.pill {
  margin-left: auto;
  color: var(--amber);
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-shell);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 8px 0;
  flex: none;
}

.section {
  margin-bottom: 14px;
  padding: 0 8px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 8px;
  font-size: calc(var(--fs-small) - 1px);
  font-weight: 600;
  color: var(--text-dim);
  cursor: default;
}

.section-header:hover {
  color: var(--text-muted);
}

/* The same colours the graph uses, so a kind looks the same everywhere. */
.ref.branch .ref-icon {
  color: var(--green);
}

.ref.tag .ref-icon {
  color: var(--text-dim);
}

/* Pushed to the right edge, and only there when the pointer is. */
/* The + replaces the count on hover, so every section's count lines up on the right. */
.section-header .add {
  display: none;
  place-items: center;
  width: 22px;
  height: 22px;
  margin-left: auto;
  padding: 0;
  line-height: 1;
  color: var(--text-muted);
}

.section-header:hover .add {
  display: grid;
}

.section-header:has(.add):hover .count {
  display: none;
}

/* Drawn, not a ▾ glyph, which sits on the baseline and never lines up with the label. */
.chevron {
  display: grid;
  place-items: center;
  width: 14px;
  height: 14px;
  flex: none;
  transition: transform 0.12s ease;
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

.chevron.closed {
  transform: rotate(-90deg);
}

.count {
  margin-left: auto;
  font-weight: 500;
}

.ref {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 8px 0 24px;
  height: 28px;
  border-radius: var(--radius-sm);
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
  color: var(--text);
}

/* The checked-out branch: a dot in its colour, where the others have nothing. */
.ref.head {
  background: var(--bg-hover);
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
  font-size: calc(var(--fs-small) - 2px);
  color: var(--text-dim);
}

.empty {
  padding: 2px 8px 2px 24px;
  font-size: var(--fs-small);
  color: var(--text-dim);
}
</style>
