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
import { CommitIndex, CommitRange, SearchCommits } from '@wails/go/main/App'
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
  /** The commit HEAD points at, branch or not, which the graph rings. */
  headHash: string
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
  rebase: [hash: string]
}>()

/** Virtualised list. */
const ROW = 30
const OVERSCAN = 12
const PAGE = 200

/** Graph geometry. Lanes are evenly spaced; the dot sits at the row's middle. */
const LANE_W = 15
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

// ---- search ---------------------------------------------------------------

const finding = ref(false)
const findInput = ref<HTMLInputElement | null>(null)
const query = ref('')
const matches = ref<number[]>([])
const current = ref(0)

const matchRows = computed(() => new Set(matches.value))

async function runSearch() {
  const q = query.value
  matches.value = q ? await SearchCommits(props.repoId, q) : []
  current.value = 0
  if (matches.value.length) void goTo(0)
}

let timer: number | undefined
watch(query, () => {
  clearTimeout(timer)
  timer = window.setTimeout(runSearch, 120)
})

/** Scrolls to the nth match and selects it, exactly as revealing a ref does. */
async function goTo(n: number) {
  const i = matches.value[n]
  if (i === undefined) return

  current.value = n
  scrollTo(i)
  await fetchPage(Math.floor(i / PAGE))
  const entry = entryAt(i)
  if (entry) select(entry)
}

/** Steps through the matches, wrapping at either end. */
function step(d: number) {
  const n = matches.value.length
  if (!n) return
  void goTo((current.value + d + n) % n)
}

function openFind() {
  finding.value = true
  void nextTick(() => findInput.value?.select())
}

function closeFind() {
  finding.value = false
  query.value = ''
  matches.value = []
}

function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
    e.preventDefault()
    openFind()
  } else if (e.key === 'Escape' && finding.value) {
    closeFind()
  }
}

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
    void runSearch()
  },
)

watch(
  () => props.repoId,
  () => {
    pages.value = new Map()
    partial.clear()
    selected.value = ''
    closeFind()
    if (viewport.value) viewport.value.scrollTop = 0
    scrollTop.value = 0
    ensureVisiblePages()
  },
)

let observer: ResizeObserver | null = null

onMounted(() => {
  window.addEventListener('keydown', onKey)
  if (!viewport.value) return
  viewportHeight.value = viewport.value.clientHeight
  observer = new ResizeObserver(() => {
    viewportHeight.value = viewport.value?.clientHeight ?? 0
  })
  observer.observe(viewport.value)
  ensureVisiblePages()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  observer?.disconnect()
})
</script>

<template>
  <div class="commit-list" :style="{ '--graph-w': graphWidth + 'px' }">
    <div v-if="finding" class="find">
      <input
        ref="findInput"
        v-model="query"
        class="find-input"
        placeholder="Search message, author or hash"
        @keydown.enter.prevent="step($event.shiftKey ? -1 : 1)"
      />
      <span class="find-count muted">
        {{ matches.length ? `${current + 1}/${matches.length}` : query ? 'none' : '' }}
      </span>
      <button class="find-btn" :disabled="!matches.length" @click="step(-1)">‹</button>
      <button class="find-btn" :disabled="!matches.length" @click="step(1)">›</button>
      <button class="find-btn" @click="closeFind">✕</button>
    </div>

    <div class="header">
      <span></span>
      <span>Description</span>
      <span class="col-author">Author</span>
      <span class="col-date">Date</span>
      <span class="col-hash">Commit</span>
    </div>

    <div ref="viewport" class="viewport" @scroll.passive="onScroll">
      <div class="spacer" :style="{ height: totalHeight + 'px' }">
        <ContextMenuRoot v-for="row in rows" :key="row.index">
          <ContextMenuTrigger as-child :disabled="!row.entry">
            <div
              class="row"
              :class="{
                selected: row.entry && row.entry.commit.hash === selected,
                match: matchRows.has(row.index),
                'match-current': matches[current] === row.index,
              }"
              :style="{
                transform: `translateY(${row.index * ROW}px)`,
                '--lane': row.entry ? laneColor(row.entry.graph.color) : undefined,
              }"
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
                  <!-- Where you are: a ring around the dot, as Fork draws it. -->
                  <circle
                    v-if="row.entry.commit.hash === headHash"
                    class="head-ring"
                    :cx="x(row.entry.graph.lane)"
                    :cy="MID"
                    r="7.5"
                    fill="none"
                    :stroke="laneColor(row.entry.graph.color)"
                    stroke-width="1.5"
                  />
                  <circle
                    :cx="x(row.entry.graph.lane)"
                    :cy="MID"
                    :r="row.entry.graph.merge ? 3.6 : 4.2"
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
                    :class="[r.kind, { current: r.head }]"
                  >
                    <RefIcon
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
              <ContextMenuItem
                class="menu-item"
                :disabled="!head"
                @select="emit('rebase', row.entry!.commit.hash)"
              >
                Rebase {{ head || 'HEAD' }} on this commit
              </ContextMenuItem>
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
  /* Each column a similar margin wider than its value, in em, so they sit evenly spaced. */
  grid-template-columns: var(--graph-w) 1fr 9em 11.5em 6em;
  align-items: center;
  gap: 10px;
  padding: 0 14px;
}

.header {
  height: 32px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
  color: var(--text-dim);
  font-size: calc(var(--fs-small) - 1px);
  font-weight: 500;
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
  height: 30px;
  white-space: nowrap;
  cursor: default;
}

.row:hover {
  background: color-mix(in srgb, var(--bg-hover) 70%, transparent);
}

/* The selected row's edge takes its branch's colour, tying the row to the graph. */
.row.selected {
  background: var(--bg-selected);
  box-shadow: inset 3px 0 0 var(--lane, var(--text-muted));
}

.row.match {
  background: color-mix(in srgb, var(--amber) 14%, transparent);
}

.row.match-current {
  background: color-mix(in srgb, var(--amber) 30%, transparent);
}

.find {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
}

.find-input {
  flex: 1;
  min-width: 0;
  height: 30px;
  padding: 0 10px;
  color: inherit;
  font: inherit;
  background: var(--bg-shell);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  outline: none;
}

.find-input:focus {
  border-color: var(--accent);
}

.find-count {
  min-width: 48px;
  text-align: right;
}

.find-btn {
  padding: 1px 7px;
  color: inherit;
  cursor: default;
  background: none;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
}

.find-btn:hover:not(:disabled) {
  background: var(--bg-hover);
}

.find-btn:disabled {
  opacity: 0.4;
}

/* The lines are drawn open; only the dot is filled. */
/* The ring around HEAD on the first lane reaches past the column's left edge. */
.graph {
  overflow: visible;
}

.graph path {
  fill: none;
  stroke-width: 2;
  stroke-linecap: round;
}

/* Flex, so the badges sit centred against the subject instead of on its baseline. */
.col-subject {
  display: flex;
  align-items: center;
  gap: 7px;
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
  color: var(--text-muted);
}

.col-hash {
  color: var(--text-dim);
}

/* The values read as columns only when they line up in the middle. */
.col-author,
.col-date,
.col-hash {
  text-align: center;
  font-variant-numeric: tabular-nums;
}

/* Refs drawn in their commit's lane colour: solid local, dashed remote, filled when checked out. */
.badge {
  display: inline-flex;
  align-items: center;
  flex: none;
  gap: 5px;
  height: 19px;
  padding: 0 7px 0 6px;
  border-radius: 3px;
  font-size: calc(var(--fs-small) - 1px);
  font-weight: 600;
  line-height: 1;
  color: var(--text);
  border: 1px solid var(--lane);
}

.badge .ref-icon {
  width: 11px;
  height: 11px;
  color: var(--lane);
}

.badge.remote {
  border-style: dashed;
  color: var(--text-muted);
  font-weight: 500;
}

.badge.current {
  background: var(--lane);
  color: var(--bg);
}

.badge.current .ref-icon {
  color: var(--bg);
}

.placeholder {
  height: 8px;
  width: 40%;
  border-radius: 3px;
  background: var(--bg-hover);
}
</style>
