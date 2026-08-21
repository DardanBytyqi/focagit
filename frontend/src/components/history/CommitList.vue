<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from 'reka-ui'
import type { git, session } from '@wails/go/models'
import { CommitIndex, CommitRange } from '@wails/go/main/App'
import { formatDate } from '@/utils/format'
import RefIcon from '@/components/shell/RefIcon.vue'

const props = defineProps<{
  repoId: string
  count: number
  width: number
  /** Bumped by Go on every history walk; every cached page is stale after it. */
  gen: number
  refsByHash: Map<string, git.Ref[]>
  /** The branch a reset would move; empty when HEAD is detached. */
  head: string
  /** A commit to scroll to and select; the nonce lets the same one be asked twice. */
  reveal: { hash: string; nonce: number } | null
}>()

const emit = defineEmits<{
  select: [commit: git.Commit]
  branch: [start: string, label: string]
  checkout: [hash: string]
  tag: [commit: git.Commit]
  revert: [commit: git.Commit]
  cherryPick: [commit: git.Commit]
  reset: [commit: git.Commit]
}>()

/** Virtualised list. */
const ROW = 24
const OVERSCAN = 12
const PAGE = 200

/** Graph geometry. Lanes are evenly spaced; the dot sits at the row's middle. */
const LANE_W = 13
const MID = ROW / 2
// The column is sized for the widest point of the whole history.
const MAX_LANES = 10

const viewport = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportHeight = ref(0)
const selected = ref<string>('')

/** Page cache, keyed by page index. */
const pages = ref(new Map<number, session.Entry[]>())
/** Pages fetched before the history had finished loading, so possibly short. */
const partial = new Set<number>()
const inflight = new Set<number>()

const firstIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / ROW) - OVERSCAN))
const lastIndex = computed(() =>
  Math.min(props.count, firstIndex.value + Math.ceil(viewportHeight.value / ROW) + OVERSCAN * 2),
)
const totalHeight = computed(() => props.count * ROW)

/** Width of the graph column in pixels, clamped so it never crowds the text. */
const graphWidth = computed(() => Math.min(Math.max(props.width, 1), MAX_LANES) * LANE_W)

interface Row {
  index: number
  entry: session.Entry | undefined
}

const rows = computed<Row[]>(() => {
  const out: Row[] = []
  for (let i = firstIndex.value; i < lastIndex.value; i++) {
    out.push({ index: i, entry: entryAt(i) })
  }
  return out
})

function entryAt(i: number): session.Entry | undefined {
  return pages.value.get(Math.floor(i / PAGE))?.[i % PAGE]
}

async function fetchPage(page: number) {
  if (inflight.has(page)) return
  if (pages.value.has(page) && !partial.has(page)) return

  inflight.add(page)
  try {
    const entries = await CommitRange(props.repoId, page * PAGE, PAGE)
    pages.value.set(page, entries)
    // Short pages mean the loader had not reached this far yet.
    if (entries.length < PAGE) partial.add(page)
    else partial.delete(page)
    pages.value = new Map(pages.value) // trigger reactivity

    // Land on the newest commit rather than an empty detail pane.
    if (page === 0 && !selected.value && entries.length) select(entries[0])
  } finally {
    inflight.delete(page)
  }
}

function ensureVisiblePages() {
  const from = Math.floor(firstIndex.value / PAGE)
  const to = Math.floor(Math.max(0, lastIndex.value - 1) / PAGE)
  for (let p = from; p <= to; p++) void fetchPage(p)
}

function onScroll() {
  scrollTop.value = viewport.value?.scrollTop ?? 0
}

function select(e: session.Entry) {
  selected.value = e.commit.hash
  emit('select', e.commit)
}

/** Scrolls the row into view, centred, unless it is already on screen. */
function scrollTo(i: number) {
  const vp = viewport.value
  if (!vp) return

  const top = i * ROW
  if (top >= vp.scrollTop && top + ROW <= vp.scrollTop + vp.clientHeight) return
  vp.scrollTop = Math.max(0, top - vp.clientHeight / 2 + ROW / 2)
  scrollTop.value = vp.scrollTop
}

// Clicking a ref lands on its commit, wherever in the history that is.
watch(
  () => props.reveal,
  async (r) => {
    if (!r) return
    // The pane may have just been mounted, so the viewport is not measured yet.
    await nextTick()

    const i = await CommitIndex(props.repoId, r.hash)
    if (i < 0) return

    scrollTo(i)
    await fetchPage(Math.floor(i / PAGE))
    const entry = entryAt(i)
    if (entry) select(entry)
    // The page is still on its way; the row highlights as soon as it lands.
    else selected.value = r.hash
  },
)

// ---- graph geometry -------------------------------------------------------

/** Horizontal centre of a lane. Lanes past the cap pile up on the last one. */
function x(lane: number): number {
  return Math.min(lane, MAX_LANES - 1) * LANE_W + LANE_W / 2
}

function laneColor(color: number): string {
  return `var(--lane-${color})`
}

/** A lane crossing the row untouched: a straight vertical line. */
function passPath(lane: number): string {
  return `M${x(lane)},0 V${ROW}`
}

/** A line arriving from the row above into this commit's dot. */
function inPath(from: number, dot: number): string {
  const a = x(from)
  const b = x(dot)
  if (a === b) return `M${a},0 V${MID}`
  return `M${a},0 C${a},${MID * 0.6} ${b},${MID * 0.4} ${b},${MID}`
}

/** A line leaving this commit's dot towards a parent in the row below. */
function outPath(dot: number, to: number): string {
  const a = x(dot)
  const b = x(to)
  if (a === b) return `M${a},${MID} V${ROW}`
  return `M${a},${MID} C${a},${MID * 1.4} ${b},${MID * 1.6} ${b},${ROW}`
}

// ---- lifecycle ------------------------------------------------------------

watch([firstIndex, lastIndex], ensureVisiblePages)

// While the history streams in, the last page keeps growing and the total changes.
watch(
  () => props.count,
  () => {
    for (const p of partial) pages.value.delete(p)
    partial.clear()
    pages.value = new Map(pages.value)
    ensureVisiblePages()
  },
)

// A new walk can reorder or remove commits, so nothing cached survives it.
watch(
  () => props.gen,
  () => {
    pages.value = new Map()
    partial.clear()
    ensureVisiblePages()
  },
)

watch(
  () => props.repoId,
  () => {
    pages.value = new Map()
    partial.clear()
    selected.value = ''
    if (viewport.value) viewport.value.scrollTop = 0
    scrollTop.value = 0
    ensureVisiblePages()
  },
)

let observer: ResizeObserver | null = null

onMounted(() => {
  if (!viewport.value) return
  viewportHeight.value = viewport.value.clientHeight
  observer = new ResizeObserver(() => {
    viewportHeight.value = viewport.value?.clientHeight ?? 0
  })
  observer.observe(viewport.value)
  ensureVisiblePages()
})

onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div class="commit-list" :style="{ '--graph-w': graphWidth + 'px' }">
    <div class="header">
      <span></span>
      <span>Description</span>
      <span>Author</span>
      <span>Date</span>
      <span>Commit</span>
    </div>

    <div ref="viewport" class="viewport" @scroll.passive="onScroll">
      <div class="spacer" :style="{ height: totalHeight + 'px' }">
        <ContextMenuRoot v-for="row in rows" :key="row.index">
          <ContextMenuTrigger as-child :disabled="!row.entry">
            <div
              class="row"
              :class="{ selected: row.entry && row.entry.commit.hash === selected }"
              :style="{ transform: `translateY(${row.index * ROW}px)` }"
              @click="row.entry && select(row.entry)"
            >
              <template v-if="row.entry">
                <svg class="graph" :width="graphWidth" :height="ROW">
                  <!-- Lanes crossing this row, drawn first so the dot sits on top -->
                  <path
                    v-for="(l, i) in row.entry.graph.pass"
                    :key="`p${i}`"
                    :d="passPath(l.lane)"
                    :stroke="laneColor(l.color)"
                  />
                  <path
                    v-for="(l, i) in row.entry.graph.in"
                    :key="`i${i}`"
                    :d="inPath(l.lane, row.entry.graph.lane)"
                    :stroke="laneColor(l.color)"
                  />
                  <path
                    v-for="(l, i) in row.entry.graph.out"
                    :key="`o${i}`"
                    :d="outPath(row.entry.graph.lane, l.lane)"
                    :stroke="laneColor(l.color)"
                  />
                  <circle
                    :cx="x(row.entry.graph.lane)"
                    :cy="MID"
                    :r="row.entry.graph.merge ? 3.2 : 3.8"
                    :fill="row.entry.graph.merge ? 'var(--bg)' : laneColor(row.entry.graph.color)"
                    :stroke="laneColor(row.entry.graph.color)"
                    stroke-width="2"
                  />
                </svg>

                <span class="col-subject">
                  <span
                    v-for="r in props.refsByHash.get(row.entry.commit.hash) ?? []"
                    :key="r.name"
                    class="badge"
                    :class="r.kind"
                  >
                    <!-- The checked out branch shows a tick where the others show their kind. -->
                    <span v-if="r.head" class="head">✓</span>
                    <RefIcon
                      v-else
                      :kind="r.kind === 'remote' ? 'remote' : r.kind === 'tag' ? 'tag' : 'branch'"
                    />
                    {{ r.short }}
                  </span>
                  <span class="subject-text">{{ row.entry.commit.subject }}</span>
                </span>

                <span class="col-author muted">{{ row.entry.commit.author.name }}</span>
                <span class="col-date muted">{{ formatDate(row.entry.commit.author.when) }}</span>
                <span class="col-hash mono muted">{{ row.entry.commit.hash.slice(0, 7) }}</span>
              </template>

              <!-- The page for this row has not arrived yet. -->
              <template v-else>
                <span></span>
                <span class="placeholder"></span>
              </template>
            </div>
          </ContextMenuTrigger>

          <ContextMenuPortal v-if="row.entry">
            <ContextMenuContent class="menu">
              <ContextMenuItem
                class="menu-item"
                @select="emit('branch', row.entry!.commit.hash, row.entry!.commit.hash.slice(0, 7))"
              >
                New branch here…
              </ContextMenuItem>
              <ContextMenuSeparator class="menu-sep" />
              <ContextMenuItem class="menu-item" @select="emit('checkout', row.entry!.commit.hash)">
                Checkout this commit
              </ContextMenuItem>

              <ContextMenuSeparator class="menu-sep" />
              <ContextMenuItem class="menu-item" @select="emit('tag', row.entry!.commit)">
                Tag this commit…
              </ContextMenuItem>

              <ContextMenuSeparator class="menu-sep" />
              <ContextMenuItem class="menu-item" @select="emit('cherryPick', row.entry!.commit)">
                Cherry-pick this commit…
              </ContextMenuItem>
              <ContextMenuItem class="menu-item" @select="emit('revert', row.entry!.commit)">
                Revert this commit…
              </ContextMenuItem>

              <ContextMenuSeparator class="menu-sep" />
              <!-- A reset moves a branch, so there has to be one to move. -->
              <ContextMenuItem
                class="menu-item danger"
                :disabled="!head"
                @select="emit('reset', row.entry!.commit)"
              >
                Reset {{ head || 'HEAD' }} to here…
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenuPortal>
        </ContextMenuRoot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.commit-list {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.header,
.row {
  display: grid;
  grid-template-columns: var(--graph-w) 1fr 140px 110px 70px;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
}

.header {
  height: 26px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-raised);
  color: var(--text-muted);
  font-size: var(--fs-small);
  flex: none;
}

.viewport {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
}

.spacer {
  position: relative;
}

.row {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 24px;
  white-space: nowrap;
  cursor: default;
}

.row:hover {
  background: var(--bg-hover);
}

.row.selected {
  background: var(--bg-selected);
}

/* The lines are drawn open; only the dot is filled. */
.graph path {
  fill: none;
  stroke-width: 1.6;
}

/* Flex, so the badges sit centred against the subject instead of on its baseline. */
.col-subject {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  overflow: hidden;
}

.subject-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.col-author,
.col-date {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: var(--fs-small);
}

/* The glyph says which kind it is, the colour says it from further away. */
.badge {
  display: inline-flex;
  align-items: center;
  flex: none;
  gap: 5px;
  height: 18px;
  padding: 0 9px;
  border-radius: 3px;
  font-size: 11px;
  line-height: 1;
  background: var(--bg);
  border: 1px solid var(--border-strong);
  color: var(--text);
}

.badge .ref-icon {
  width: 11px;
  height: 11px;
}

/* The colour is on the border and the glyph; the name stays as readable as the subject. */
.badge.branch {
  border-color: var(--green);
}

.badge.branch .ref-icon {
  color: var(--green);
}

.badge.tag {
  border-color: var(--amber);
}

.badge.tag .ref-icon {
  color: var(--amber);
}

.badge.remote .ref-icon {
  color: var(--text-muted);
}

.badge .head {
  color: var(--green);
  font-weight: 700;
}

.placeholder {
  height: 8px;
  width: 40%;
  border-radius: 3px;
  background: var(--bg-hover);
}
</style>
