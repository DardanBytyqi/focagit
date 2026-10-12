<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { lineKey } from '@/utils/lines'

const props = withDefaults(
  defineProps<{
    diff: git.FileDiff | null
    loading: boolean
    /** Changes view only: lets the user pick individual lines. */
    selectable?: boolean
    /** Selected line keys, "hunk:line". */
    selected?: Set<string>
    /** Label for the per-hunk button: "Stage" or "Unstage". */
    hunkAction?: string
    /** Shown next to it, and only for unstaged hunks. */
    discardable?: boolean
  }>(),
  {
    selectable: false,
    selected: () => new Set<string>(),
    hunkAction: 'Stage',
    discardable: false,
  },
)

const emit = defineEmits<{
  select: [keys: string[], on: boolean]
  hunk: [index: number]
  discardHunk: [index: number]
}>()

// A 2 MiB patch is roughly 40k lines.
const MAX_LINES = 2000

interface Block {
  index: number
  hunk: git.Hunk
  lines: git.DiffLine[]
}

const blocks = computed<Block[]>(() => {
  if (!props.diff) return []
  const out: Block[] = []
  let budget = MAX_LINES
  ;(props.diff.hunks ?? []).forEach((hunk, index) => {
    if (budget <= 0) return
    out.push({ index, hunk, lines: hunk.lines.slice(0, budget) })
    budget -= hunk.lines.length
  })
  return out
})

const totalLines = computed(() => (props.diff?.hunks ?? []).reduce((n, h) => n + h.lines.length, 0))
const hiddenLines = computed(() =>
  Math.max(0, totalLines.value - blocks.value.reduce((n, b) => n + b.lines.length, 0)),
)

function changed(l: git.DiffLine): boolean {
  return l.kind === 'add' || l.kind === 'delete'
}

/** Anchor for shift-click ranges, reset whenever the file changes. */
const anchor = ref<{ hunk: number; line: number } | null>(null)
watch(
  () => props.diff,
  () => (anchor.value = null),
)

function onLineClick(hunkIndex: number, lineIndex: number, e: MouseEvent) {
  if (!props.selectable) return
  const hunk = props.diff?.hunks?.[hunkIndex]
  if (!hunk || !changed(hunk.lines[lineIndex])) return

  // Shift-click extends from the last plain click, the way every file list behaves.
  if (e.shiftKey && anchor.value && anchor.value.hunk === hunkIndex) {
    const [from, to] = [anchor.value.line, lineIndex].sort((a, b) => a - b)
    const keys: string[] = []
    for (let i = from; i <= to; i++) {
      if (changed(hunk.lines[i])) keys.push(lineKey(hunkIndex, i))
    }
    emit('select', keys, true)
    return
  }

  anchor.value = { hunk: hunkIndex, line: lineIndex }
  const k = lineKey(hunkIndex, lineIndex)
  emit('select', [k], !props.selected.has(k))
}

function selectHunk(b: Block) {
  const keys = b.lines.map((l, i) => (changed(l) ? lineKey(b.index, i) : '')).filter(Boolean)
  emit('select', keys, true)
}

function sign(kind: string): string {
  return kind === 'add' ? '+' : kind === 'delete' ? '-' : ' '
}
</script>

<template>
  <div class="diff">
    <div v-if="loading" class="notice muted">loading…</div>

    <div v-else-if="!diff" class="notice muted">select a file</div>

    <div v-else-if="diff.binary" class="notice muted">binary file — no textual diff</div>

    <div v-else-if="!diff.hunks?.length" class="notice muted">no changes in this file</div>

    <!-- As wide as the longest line, so backgrounds and headers reach the end when scrolled. -->
    <div v-else class="sheet">
      <div v-for="b in blocks" :key="b.index" class="hunk" :class="{ armed: selectable }">
        <!-- Pinned to the visible edge so the buttons stay in reach when scrolled sideways. -->
        <div v-if="selectable" class="hunk-float">
          <div class="hunk-actions">
            <button class="tiny" @click="selectHunk(b)">Select</button>
            <button class="tiny" @click="emit('hunk', b.index)">{{ hunkAction }}</button>
            <button v-if="discardable" class="tiny danger" @click="emit('discardHunk', b.index)">
              Discard
            </button>
          </div>
        </div>

        <div class="hunk-header">
          <div class="hunk-title">
            <span class="range"
              >@@ -{{ b.hunk.oldStart }},{{ b.hunk.oldLines }} +{{ b.hunk.newStart }},{{
                b.hunk.newLines
              }}
              @@</span
            >
            <span v-if="b.hunk.header" class="heading">{{ b.hunk.header }}</span>
          </div>
        </div>

        <div
          v-for="(l, li) in b.lines"
          :key="li"
          class="line"
          :class="[
            l.kind,
            {
              pick: selectable && changed(l),
              on: selectable && selected.has(lineKey(b.index, li)),
            },
          ]"
          @click="onLineClick(b.index, li, $event)"
        >
          <span class="ln">{{ l.old || '' }}</span>
          <span class="ln">{{ l.new || '' }}</span>
          <span class="sign">{{ sign(l.kind) }}</span>
          <span class="text"
            >{{ l.text }}<span v-if="l.noNewline" class="nonl"> ↵ no newline</span></span
          >
        </div>
      </div>

      <div v-if="hiddenLines" class="notice muted">
        {{ hiddenLines.toLocaleString() }} more lines not shown
      </div>
      <div v-if="diff.truncated" class="notice muted">
        patch truncated — the file is too large to display in full
      </div>
    </div>
  </div>
</template>

<style scoped>
.diff {
  height: 100%;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: calc(var(--fs-small) - 1px);
  line-height: 1.6;
  user-select: text;
  /* Lets pinned parts size themselves to the visible width with cqw. */
  container-type: inline-size;
}

.sheet {
  width: max-content;
  min-width: 100%;
}

.notice {
  padding: 12px 16px;
  font-family: var(--font);
  color: var(--text-dim);
}

.hunk {
  position: relative;
}

.hunk.armed:hover {
  outline: 1px solid var(--accent);
  outline-offset: -1px;
}

.hunk-float {
  position: sticky;
  left: 0;
  width: 100cqw;
  height: 0;
  z-index: var(--z-float);
}

.hunk-actions {
  position: absolute;
  top: 3px;
  right: 14px;
  display: none;
  gap: 4px;
  z-index: var(--z-float);
}

.hunk.armed:hover .hunk-actions {
  display: flex;
}

.hunk-actions .tiny {
  background: var(--bg-raised);
  border-color: var(--border-strong);
  box-shadow: 0 1px 4px var(--shadow);
}

.hunk-actions .tiny.danger:hover {
  color: var(--red);
  border-color: var(--red);
}

.hunk-header {
  background: var(--bg-shell);
  color: var(--text-dim);
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
}

/* The header text stays in view and on one line whatever the horizontal scroll. */
.hunk-title {
  position: sticky;
  left: 0;
  box-sizing: border-box;
  max-width: 100cqw;
  display: flex;
  gap: 10px;
  padding: 4px 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.range {
  flex: none;
}

.heading {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-muted);
}

.spacer {
  flex: 1;
}

.tiny {
  padding: 1px 8px;
  font-family: var(--font);
  font-size: calc(var(--fs-small) - 2px);
  border-color: var(--border-strong);
  border-radius: var(--radius-sm);
}

.line {
  display: grid;
  grid-template-columns: 4.2em 4.2em 1.6em 1fr;
  white-space: pre;
}

/* Tinted backgrounds rather than coloured text: the eye finds the changed region first. */
.line.add {
  background: color-mix(in srgb, var(--green) 11%, transparent);
}

.line.delete {
  background: color-mix(in srgb, var(--red) 11%, transparent);
}

/* The gutter stays put when scrolled sideways, so it needs a solid background under its tint. */
.ln,
.sign {
  position: sticky;
  background: var(--bg);
}

.ln:nth-child(1) {
  left: 0;
}

.ln:nth-child(2) {
  left: 4.2em;
}

.sign {
  left: 8.4em;
}

/* The gutter of a changed line carries a stronger tint of the same colour. */
.line.add .ln {
  background: linear-gradient(color-mix(in srgb, var(--green) 16%, transparent) 0 0), var(--bg);
}

.line.delete .ln {
  background: linear-gradient(color-mix(in srgb, var(--red) 16%, transparent) 0 0), var(--bg);
}

.line.add .sign {
  background: linear-gradient(color-mix(in srgb, var(--green) 11%, transparent) 0 0), var(--bg);
}

.line.delete .sign {
  background: linear-gradient(color-mix(in srgb, var(--red) 11%, transparent) 0 0), var(--bg);
}

.line.add .sign {
  color: var(--green);
}

.line.delete .sign {
  color: var(--red);
}

/* Only changed lines can be picked, and they say so on hover. */
.line.pick {
  cursor: pointer;
}

.line.pick:hover {
  filter: brightness(1.25);
}

.line.on {
  box-shadow: inset 3px 0 0 var(--blue);
  background: color-mix(in srgb, var(--blue) 20%, transparent);
}

.line.on .ln,
.line.on .sign {
  background: linear-gradient(color-mix(in srgb, var(--blue) 20%, transparent) 0 0), var(--bg);
}

.line.on .ln:first-child {
  box-shadow: inset 3px 0 0 var(--blue);
}

.ln {
  padding-right: 10px;
  text-align: right;
  color: var(--text-dim);
  user-select: none;
}

.sign {
  text-align: center;
  user-select: none;
}

.text {
  padding-right: 12px;
  overflow-wrap: normal;
}

.nonl {
  color: var(--text-dim);
  font-style: italic;
}
</style>
