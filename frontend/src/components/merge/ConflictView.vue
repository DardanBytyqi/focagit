<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { state, conflictText, resolveLines, resolveConflict, markResolved, finishOperation } =
  useRepo()

type Pick = 'ours' | 'theirs' | 'both' | 'none'

const path = ref('')
const text = ref<git.ConflictText | null>(null)
const picks = ref<Pick[]>([])
const busy = ref(false)

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

const rebase = computed(() => state.status?.rebase ?? null)

const op = computed(() => state.status?.operation ?? '')

// Sides are named after what they are: in a rebase "mine" and "theirs" point the wrong way.
const mine = computed(() => rebase.value?.onto || state.status?.head || 'HEAD')
// Go labels a commit "<short hash> <subject>"; here only the subject means anything.
const theirs = computed(() => {
  const from = state.status?.operationFrom ?? ''
  return op.value === 'merge' ? from : from.replace(/^[0-9a-f]{7,40} /, '')
})

/** What the incoming side is called on the buttons; a merge brings a branch. */
const theirsLabel = computed(() => {
  switch (op.value) {
    case 'rebase':
      return 'Your commit'
    case 'cherry-pick':
      return 'Picked commit'
    case 'revert':
      return 'Revert'
    default:
      return theirs.value || 'Incoming'
  }
})

/** A rebase stops once per conflicting commit, so the title says which one. */
const title = computed(() =>
  rebase.value?.steps
    ? `Resolve conflicts · ${rebase.value.step}/${rebase.value.steps}`
    : 'Resolve conflicts',
)

/** The same name inside a sentence, where only a branch keeps its capital. */
const theirsInText = computed(() =>
  op.value === 'merge' ? theirsLabel.value : theirsLabel.value.toLowerCase(),
)

/** The file being shown, as the conflict list knows it. */
const current = computed(() => state.conflicts.find((c) => c.path === path.value) ?? null)

async function select(next: string) {
  path.value = next
  text.value = null
  picks.value = []
  if (!next) return
  const got = await conflictText(next)
  text.value = got
  picks.value = Array(got?.count ?? 0).fill('none') as Pick[]
}

/** True while a rebase is being moved on to its next commit. */
const continuing = ref(false)

/** Once a commit's conflicts are resolved the rebase carries on, staying open for the next ones. */
async function continueRebase() {
  continuing.value = true
  const ok = await finishOperation('')
  continuing.value = false
  if (!ok || !state.conflicts.length) {
    emit('close')
    return
  }
  // The refresh that brought the next conflicts landed while the watcher stood aside.
  void select(state.conflicts[0].path)
}

// Opening, and every resolved file, lands on whatever is still left to do.
watch(
  [() => props.open, () => state.conflicts.map((c) => c.path).join('\n')],
  () => {
    if (!props.open || continuing.value) return
    const paths = state.conflicts.map((c) => c.path)
    // The last file resolved is the end of the job, not an empty window.
    if (!paths.length) {
      if (state.status?.operation === 'rebase') void continueRebase()
      else emit('close')
      return
    }
    if (paths.includes(path.value)) return
    void select(paths[0])
  },
  { immediate: true },
)

/** Which pick belongs to each segment; -1 for the text both sides agree on. */
const conflictIndex = computed(() => {
  const map: number[] = []
  let n = 0
  for (const s of text.value?.segments ?? []) map.push(s.conflict ? n++ : -1)
  return map
})

/** Both sides still hold a version of the file. */
const both = computed(() => !!current.value?.ours && !!current.value?.theirs)

/** Whether this file can be settled block by block at all. */
const lineByLine = computed(
  () => both.value && !!text.value && !text.value.binary && text.value.count > 0,
)

/** Why it cannot, said in the terms of this file rather than in general. */
const wholeReason = computed(() => {
  const c = current.value
  if (!c || lineByLine.value) return ''
  if (!c.ours) return `${theirs.value} changed this file and ${mine.value} deleted it.`
  if (!c.theirs) return `${mine.value} changed this file and ${theirs.value} deleted it.`
  if (text.value?.binary) return 'This file is binary, so only whole versions can be kept.'
  return 'No conflict markers left: the file has already been settled by hand.'
})

const undecided = computed(() => picks.value.filter((p) => p === 'none').length)
const canApply = computed(() => picks.value.length > 0 && undecided.value < picks.value.length)

function choose(i: number, pick: Pick) {
  picks.value[i] = picks.value[i] === pick ? 'none' : pick
}

function all(pick: Pick) {
  picks.value = picks.value.map(() => pick)
}

async function apply() {
  busy.value = true
  await resolveLines(path.value, picks.value)
  busy.value = false
}

/** For a file one side deleted there is no text to choose through. */
async function keepWhole(side: 'ours' | 'theirs') {
  busy.value = true
  await resolveConflict(path.value, side)
  busy.value = false
}

/** The file on disk is already what it should be; only git needs telling. */
async function keepAsIs() {
  busy.value = true
  await markResolved(path.value)
  busy.value = false
}
</script>

<template>
  <Modal v-model:open="isOpen" :title="title" :width="1320" fill>
    <div class="resolver">
      <nav class="files">
        <button
          v-for="c in state.conflicts"
          :key="c.path"
          type="button"
          :class="{ on: c.path === path }"
          :title="c.path"
          @click="select(c.path)"
        >
          {{ c.path }}
        </button>
      </nav>

      <section class="pane">
        <header>
          <span class="name mono">{{ path }}</span>
          <span class="spacer"></span>
          <template v-if="text?.count">
            <button type="button" class="side-btn mine" @click="all('ours')">
              All from {{ mine }}
            </button>
            <button type="button" class="side-btn theirs" @click="all('theirs')">
              All from {{ theirsInText }}
            </button>
          </template>
        </header>

        <!-- Whenever the block-by-block view cannot apply, the way out is here.
             Never a screen with an explanation and no button on it. -->
        <div v-if="wholeReason" class="whole">
          <p class="hint">{{ wholeReason }}</p>
          <div class="actions">
            <button type="button" class="side-btn mine" :disabled="busy" @click="keepWhole('ours')">
              {{ current?.ours ? `Keep ${mine}` : 'Delete it' }}
            </button>
            <button
              type="button"
              class="side-btn theirs"
              :disabled="busy"
              @click="keepWhole('theirs')"
            >
              {{ current?.theirs ? `Keep ${theirsInText}` : 'Delete it' }}
            </button>
            <!-- Only when the file on disk is a real merged version already. -->
            <button v-if="both && !text?.binary" type="button" :disabled="busy" @click="keepAsIs">
              Keep the file as it is
            </button>
          </div>
        </div>

        <div v-else-if="text" class="file mono">
          <template v-for="(s, si) in text.segments" :key="si">
            <div v-if="!s.conflict" class="common">
              <div v-for="(l, li) in s.lines" :key="li" class="line">{{ l || ' ' }}</div>
            </div>

            <div v-else class="block" :class="{ open: picks[conflictIndex[si]] === 'none' }">
              <div class="bar">
                <span class="label">Conflict {{ conflictIndex[si] + 1 }}</span>
                <span class="spacer"></span>
                <button
                  type="button"
                  class="side-btn mine"
                  :class="{ on: picks[conflictIndex[si]] === 'ours' }"
                  @click="choose(conflictIndex[si], 'ours')"
                >
                  {{ mine }}
                </button>
                <button
                  type="button"
                  class="side-btn theirs"
                  :class="{ on: picks[conflictIndex[si]] === 'theirs' }"
                  @click="choose(conflictIndex[si], 'theirs')"
                >
                  {{ theirsLabel }}
                </button>
                <button
                  type="button"
                  class="side-btn both"
                  :class="{ on: picks[conflictIndex[si]] === 'both' }"
                  @click="choose(conflictIndex[si], 'both')"
                >
                  Both
                </button>
              </div>

              <div class="side mine" :class="{ dropped: picks[conflictIndex[si]] === 'theirs' }">
                <div v-for="(l, li) in s.ours" :key="li" class="line">{{ l || ' ' }}</div>
              </div>
              <div class="side theirs" :class="{ dropped: picks[conflictIndex[si]] === 'ours' }">
                <div v-for="(l, li) in s.theirs" :key="li" class="line">{{ l || ' ' }}</div>
              </div>
            </div>
          </template>
        </div>
      </section>
    </div>

    <div class="actions">
      <!-- The colours are the legend: the same two are on every button. -->
      <span class="legend">
        <span class="swatch mine"></span><b>{{ mine }}</b> <span class="swatch theirs"></span
        >{{ theirsLabel }}
        <b v-if="op !== 'merge'">{{ theirs }}</b>
      </span>
      <span class="spacer"></span>
      <button type="button" @click="emit('close')">Close</button>
      <button
        v-if="text?.count"
        type="button"
        class="primary"
        :disabled="!canApply || busy || continuing"
        @click="apply"
      >
        {{ continuing ? 'Continuing the rebase…' : `Apply to ${path.split('/').pop()}` }}
      </button>
    </div>
  </Modal>
</template>

<style scoped>
.resolver {
  display: flex;
  gap: 12px;
  flex: 1 1 auto;
  min-height: 0;
}

.files {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: none;
  width: 190px;
  padding-right: 10px;
  border-right: 1px solid var(--border);
  overflow-y: auto;
}

.files button {
  padding: 4px 8px;
  text-align: left;
  font-size: var(--fs-small);
  color: var(--text-muted);
  /* Long paths shorten from the left: the file name matters more than the folder. */
  direction: rtl;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.files button.on {
  background: var(--bg-selected);
  color: var(--text);
}

.pane {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  /* Both zero minimums: without them a flex child refuses to shrink below its
     content and the scrollbar lands on the dialog instead of the code. */
  min-width: 0;
  min-height: 0;
}

.pane header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.spacer {
  flex: 1;
}

.file {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  user-select: text;
}

.line {
  padding: 0 8px;
  white-space: pre;
}

.common .line {
  color: var(--text-muted);
}

.block {
  margin: 4px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

/* Until a side is picked, the block is the thing to look at. */
.block.open {
  border-color: var(--amber);
}

.bar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: var(--bg-raised);
  font-family: var(--font);
  font-size: 11px;
}

.label {
  color: var(--text-muted);
}

/* One button style for both sides, so the colour is the only difference and
   therefore the thing that teaches the pairing. */
.side-btn {
  padding: 1px 9px;
  font-size: 11px;
  border: 1px solid currentColor;
  border-radius: 3px;
  background: none;
}

.side-btn.mine {
  color: var(--mine);
}

.side-btn.theirs {
  color: var(--theirs);
}

.side-btn.both {
  color: var(--text-muted);
}

.side-btn:hover:not(:disabled) {
  background: color-mix(in srgb, currentColor 18%, transparent);
  border-color: currentColor;
}

/* The chosen side fills in. The label goes to the page background rather than
   to white, so it stays legible in the light theme too. */
.side-btn.mine.on {
  background: var(--mine);
  border-color: var(--mine);
  color: var(--bg);
}

.side-btn.theirs.on {
  background: var(--theirs);
  border-color: var(--theirs);
  color: var(--bg);
}

.side-btn.both.on {
  background: var(--text-muted);
  border-color: var(--text-muted);
  color: var(--bg);
}

.side.mine {
  background: color-mix(in srgb, var(--mine) 14%, transparent);
  border-left: 3px solid var(--mine);
}

.side.theirs {
  background: color-mix(in srgb, var(--theirs) 14%, transparent);
  border-left: 3px solid var(--theirs);
}

/* A dropped side stays readable, so the choice can be seen and undone. */
.side.dropped {
  opacity: 0.3;
  text-decoration: line-through;
}

.whole {
  padding: 10px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
}

.pad {
  padding: 10px;
}

.legend {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted);
}

.swatch {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.swatch.mine {
  background: var(--mine);
}

.swatch.theirs {
  background: var(--theirs);
  margin-left: 8px;
}

.actions {
  flex: none;
  align-items: center;
}
</style>
