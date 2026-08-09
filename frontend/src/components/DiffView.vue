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

const totalLines = computed(() =>
  (props.diff?.hunks ?? []).reduce((n, h) => n + h.lines.length, 0),
)
const hiddenLines = computed(() =>
  Math.max(0, totalLines.value - blocks.value.reduce((n, b) => n + b.lines.length, 0)),
)

function changed(l: git.DiffLine): boolean {
  return l.kind === 'add' || l.kind === 'delete'
}

/** Anchor for shift-click ranges, reset whenever the file changes. */
const anchor = ref<{ hunk: number; line: number } | null>(null)
watch(() => props.diff, () => (anchor.value = null))

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

    <template v-else>
      <div v-for="b in blocks" :key="b.index" class="hunk" :class="{ armed: selectable }">
        <div class="hunk-header">
          <span class="range"
            >@@ -{{ b.hunk.oldStart }},{{ b.hunk.oldLines }} +{{ b.hunk.newStart }},{{
              b.hunk.newLines
            }} @@</span
          >
          <span v-if="b.hunk.header" class="heading">{{ b.hunk.header }}</span>
        </div>

        <!-- Floating over the hunk under the cursor rather than sitting in the
             header: the action belongs to the block you are looking at. -->
        <div v-if="selectable" class="hunk-actions">
          <button class="tiny" @click="selectHunk(b)">Select</button>
          <button class="tiny" @click="emit('hunk', b.index)">{{ hunkAction }}</button>
          <button
            v-if="discardable"
            class="tiny danger"
            @click="emit('discardHunk', b.index)"
          >
            Discard
          </button>
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
    </template>
  </div>
</template>

<style scoped>
.diff {
  height: 100%;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: var(--fs-small);
  line-height: 1.45;
  user-select: text;
}

.notice {
  padding: 10px 12px;
  font-family: var(--font);
  font-style: italic;
}

.hunk {
  position: relative;
}

.hunk.armed:hover {
  outline: 1px solid var(--accent);
  outline-offset: -1px;
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
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.4);
}

.hunk-actions .tiny.danger:hover {
  color: var(--red);
  border-color: var(--red);
}

.hunk-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 2px 10px;
  background: var(--bg-toolbar);
  color: var(--text-dim);
  border-top: 1px solid var(--border);
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
}

.heading {
  color: var(--text-muted);
}

.spacer {
  flex: 1;
}

.tiny {
  padding: 0 6px;
  font-family: var(--font);
  font-size: 11px;
  border-color: var(--border-strong);
}

.line {
  display: grid;
  grid-template-columns: 46px 46px 14px 1fr;
  white-space: pre;
}

/* Tinted backgrounds rather than coloured text: the eye finds the changed
   region before it reads any of it. */
.line.add {
  background: color-mix(in srgb, var(--green) 14%, transparent);
}

.line.delete {
  background: color-mix(in srgb, var(--red) 14%, transparent);
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
  box-shadow: inset 3px 0 0 var(--accent);
  background: color-mix(in srgb, var(--accent) 22%, transparent);
}

.ln {
  padding-right: 8px;
  text-align: right;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--bg) 60%, transparent);
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
