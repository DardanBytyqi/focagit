<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git, patch } from '@wails/go/models'
import {
  Commit,
  DiscardFiles,
  DiscardLines,
  StageFiles,
  StageLines,
  UnstageFiles,
  WorkingDiff,
} from '@wails/go/main/App'
import type { Selected, TreeNode } from '@/types'
import { lineRefs } from '@/utils/lines'
import { allDirs, buildTree } from '@/utils/tree'
import Check from '@/components/common/Check.vue'
import DiffView from '@/components/common/DiffView.vue'
import FileTree from '@/components/changes/FileTree.vue'

const props = defineProps<{
  repoId: string
  status: git.Status | null
}>()

const emit = defineEmits<{ changed: [] }>()

const picked = ref<Selected | null>(null)
const diff = ref<git.FileDiff | null>(null)
const loadingDiff = ref(false)
const selection = ref(new Set<string>())
const busy = ref(false)
const error = ref('')

const summary = ref('')
const description = ref('')
const amend = ref(false)

// A file edited after being staged appears in both lists, with different content in each.
const unstaged = computed(() =>
  (props.status?.files ?? []).filter((f) => f.untracked || f.unstaged !== ''),
)
const staged = computed(() => (props.status?.files ?? []).filter((f) => f.staged !== ''))

const unstagedTree = computed(() => buildTree(unstaged.value))
const stagedTree = computed(() => buildTree(staged.value))

/** Directories the user closed by hand, so re-expanding does not undo it. */
const collapsed = ref(new Set<string>())

// Directories start open: a collapsed tree hides the very thing the user came to look at.
const expanded = ref(new Set<string>())
watch(
  [unstagedTree, stagedTree],
  ([u, s]) => {
    const next = new Set(expanded.value)
    for (const d of [...allDirs(u), ...allDirs(s)]) {
      if (!collapsed.value.has(d)) next.add(d)
    }
    expanded.value = next
  },
  { immediate: true },
)

function toggleDir(path: string) {
  const next = new Set(expanded.value)
  const shut = new Set(collapsed.value)
  if (next.has(path)) {
    next.delete(path)
    shut.add(path)
  } else {
    next.add(path)
    shut.delete(path)
  }
  expanded.value = next
  collapsed.value = shut
}

const canCommit = computed(
  () => staged.value.length > 0 && summary.value.trim() !== '' && !busy.value,
)

// 72 is where git's own tooling starts wrapping.
const summaryTooLong = computed(() => summary.value.length > 72)

async function pick(node: TreeNode, isStaged: boolean) {
  picked.value = {
    path: node.path,
    staged: isStaged,
    untracked: !!node.file?.untracked,
    dir: node.dir,
  }
  selection.value = new Set()
  await loadDiff()
}

// The paths a node stands for.
function pathsOf(path: string, dir: boolean, isStaged: boolean): string[] {
  if (!dir) return [path]
  const list = isStaged ? staged.value : unstaged.value
  const prefix = path + '/'
  return list.filter((f) => f.path.startsWith(prefix)).map((f) => f.path)
}

/** Paths the current selection covers, empty when it is in the other pane. */
function selectedPaths(isStaged: boolean): string[] {
  const p = picked.value
  if (!p || p.staged !== isStaged) return []
  return pathsOf(p.path, p.dir, isStaged)
}

let token = 0

async function loadDiff() {
  const p = picked.value
  // A directory has no diff of its own.
  if (!p || p.dir) {
    diff.value = null
    return
  }
  const mine = ++token
  loadingDiff.value = true
  try {
    const d = await WorkingDiff(props.repoId, p.path, p.staged, p.untracked)
    if (mine !== token) return
    diff.value = d
  } catch (e) {
    if (mine === token) error.value = String(e)
  } finally {
    if (mine === token) loadingDiff.value = false
  }
}

/** Runs a repository-changing call, then refreshes everything downstream. */
async function act(fn: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await fn()
    emit('changed')
    selection.value = new Set()
    await loadDiff()
  } catch (e) {
    error.value = String(e)
  } finally {
    busy.value = false
  }
}

const stageAll = () =>
  act(() =>
    StageFiles(
      props.repoId,
      unstaged.value.map((f) => f.path),
    ),
  )
const unstageAll = () =>
  act(() =>
    UnstageFiles(
      props.repoId,
      staged.value.map((f) => f.path),
    ),
  )

function apply(paths: string[], isStaged: boolean) {
  if (!paths.length) return
  return act(() => (isStaged ? UnstageFiles(props.repoId, paths) : StageFiles(props.repoId, paths)))
}

/** The pane header button: acts on what is selected, not on everything. */
function applySelected(isStaged: boolean) {
  return apply(selectedPaths(isStaged), isStaged)
}

function nodeAction(node: TreeNode, isStaged: boolean) {
  return apply(pathsOf(node.path, node.dir, isStaged), isStaged)
}

function discardNode(node: TreeNode) {
  const paths = pathsOf(node.path, node.dir, false)
  if (!paths.length) return
  const what = node.dir ? `${paths.length} files under ${node.path}` : node.path
  // Discarded working tree content is not recoverable from git.
  if (!window.confirm(`Discard all changes to ${what}?\n\nThis cannot be undone.`)) return
  return act(() => DiscardFiles(props.repoId, paths))
}

function hunkRefs(index: number): patch.LineRef[] {
  const hunk = diff.value?.hunks?.[index]
  if (!hunk) return []
  const refs: patch.LineRef[] = []
  hunk.lines.forEach((l, i) => {
    if (l.kind === 'add' || l.kind === 'delete') refs.push({ hunk: index, line: i })
  })
  return refs
}

function applySelection() {
  const p = picked.value
  if (!p || !selection.value.size) return
  // Which pane the file was picked in decides the direction.
  return act(() => StageLines(props.repoId, p.path, lineRefs(selection.value), p.staged))
}

function applyHunk(index: number) {
  const p = picked.value
  const refs = hunkRefs(index)
  if (!p || !refs.length) return
  return act(() => StageLines(props.repoId, p.path, refs, p.staged))
}

function discardHunk(index: number) {
  const p = picked.value
  const refs = hunkRefs(index)
  if (!p || !refs.length) return
  if (!window.confirm('Discard this hunk?\n\nThis cannot be undone.')) return
  return act(() => DiscardLines(props.repoId, p.path, refs))
}

function onSelect(keys: string[], on: boolean) {
  const next = new Set(selection.value)
  for (const k of keys) {
    if (on) next.add(k)
    else next.delete(k)
  }
  selection.value = next
}

// Split a pasted multi-line message the way git reads it: first paragraph is the subject.
function onSummaryPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text') ?? ''
  if (!text.includes('\n')) return

  e.preventDefault()
  const [head, ...rest] = text.split(/\n\s*\n/)
  summary.value = head.replace(/\n/g, ' ').trim()
  const body = rest.join('\n\n').trim()
  if (body) description.value = description.value ? description.value + '\n\n' + body : body
}

/** Depth-first search for the first actual file in a tree. */
function firstFile(nodes: TreeNode[]): TreeNode | null {
  for (const n of nodes) {
    if (!n.dir) return n
    const found = firstFile(n.children)
    if (found) return found
  }
  return null
}

function commit() {
  // Git stores one blob.
  const message = description.value.trim()
    ? `${summary.value.trim()}\n\n${description.value.trim()}\n`
    : `${summary.value.trim()}\n`

  return act(async () => {
    await Commit(props.repoId, message, amend.value)
    summary.value = ''
    description.value = ''
    amend.value = false
    picked.value = null
    diff.value = null
  })
}

watch(
  () => props.status,
  () => {
    const p = picked.value

    // Land on something rather than an empty pane, the way Fork opens with the first.
    if (!p) {
      const fromUnstaged = firstFile(unstagedTree.value)
      const node = fromUnstaged ?? firstFile(stagedTree.value)
      if (node) void pick(node, !fromUnstaged)
      return
    }

    // Drop the selection when its file leaves the pane it was picked in.
    const list = p.staged ? staged.value : unstaged.value
    if (!list.some((f) => f.path === p.path)) {
      picked.value = null
      diff.value = null
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="changes">
    <div class="files">
      <div class="pane">
        <div class="pane-header">
          <span class="title">Unstaged</span>
          <span class="count muted">{{ unstaged.length }}</span>
          <span class="spacer"></span>
          <button
            class="tiny"
            :disabled="!selectedPaths(false).length || busy"
            @click="applySelected(false)"
          >
            Stage
          </button>
          <button class="tiny ghost" :disabled="!unstaged.length || busy" @click="stageAll">
            All
          </button>
        </div>
        <div class="list">
          <FileTree
            :nodes="unstagedTree"
            :depth="0"
            :expanded="expanded"
            :selected-path="picked && !picked.staged ? picked.path : ''"
            :staged="false"
            :busy="busy"
            @pick="pick($event, false)"
            @toggle-dir="toggleDir"
            @action="nodeAction($event, false)"
            @discard="discardNode"
          />
          <div v-if="!unstaged.length" class="empty muted">nothing to stage</div>
        </div>
      </div>

      <div class="pane">
        <div class="pane-header">
          <span class="title">Staged</span>
          <span class="count muted">{{ staged.length }}</span>
          <span class="spacer"></span>
          <button
            class="tiny"
            :disabled="!selectedPaths(true).length || busy"
            @click="applySelected(true)"
          >
            Unstage
          </button>
          <button class="tiny ghost" :disabled="!staged.length || busy" @click="unstageAll">
            All
          </button>
        </div>
        <div class="list">
          <FileTree
            :nodes="stagedTree"
            :depth="0"
            :expanded="expanded"
            :selected-path="picked && picked.staged ? picked.path : ''"
            :staged="true"
            :busy="busy"
            @pick="pick($event, true)"
            @toggle-dir="toggleDir"
            @action="nodeAction($event, true)"
          />
          <div v-if="!staged.length" class="empty muted">nothing staged</div>
        </div>
      </div>
    </div>

    <div class="splitv"></div>

    <div class="work">
      <div class="diff-header">
        <span v-if="picked" class="mono file-name">{{ picked.path }}</span>
        <span class="spacer"></span>
        <template v-if="picked && selection.size">
          <span class="muted">{{ selection.size }} lines</span>
          <button class="tiny" :disabled="busy" @click="applySelection">
            {{ picked.staged ? 'Unstage' : 'Stage' }} selected
          </button>
        </template>
      </div>

      <div v-if="error" class="error">{{ error }}</div>

      <div class="diff-pane">
        <DiffView
          :diff="diff"
          :loading="loadingDiff"
          :selectable="!!picked"
          :selected="selection"
          :hunk-action="picked?.staged ? 'Unstage' : 'Stage'"
          :discardable="!!picked && !picked.staged"
          @select="onSelect"
          @hunk="applyHunk"
          @discard-hunk="discardHunk"
        />
      </div>

      <!-- Fork keeps summary and body in one bordered block, split by a rule,
           so it reads as the single message git will actually store. -->
      <div class="commit-box">
        <div class="message">
          <div class="summary-row">
            <input
              v-model="summary"
              class="summary"
              placeholder="Commit summary"
              maxlength="200"
              @paste="onSummaryPaste"
            />
            <span class="counter" :class="{ warn: summaryTooLong }">{{ summary.length }}</span>
          </div>
          <textarea v-model="description" class="description" placeholder="Description"></textarea>
        </div>

        <div class="commit-row">
          <Check v-model="amend" class="amend">Amend</Check>
          <span class="spacer"></span>
          <button class="primary" :disabled="!canCommit" @click="commit">
            {{
              amend
                ? 'Amend Commit'
                : `Commit ${staged.length} File${staged.length === 1 ? '' : 's'}`
            }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.changes {
  display: flex;
  flex: 1;
  height: 100%;
  min-height: 0;
  background: var(--bg-raised);
}

.files {
  width: 300px;
  flex: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 60px;
  border-bottom: 1px solid var(--border);
}

.pane-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: var(--bg-toolbar);
  border-bottom: 1px solid var(--border);
  flex: none;
}

.title {
  font-weight: 600;
}

.count {
  font-size: var(--fs-small);
}

.spacer {
  flex: 1;
}

.list {
  flex: 1;
  overflow: auto;
  min-height: 0;
  padding: 2px 0;
}

.empty {
  padding: 4px 12px;
  font-size: var(--fs-small);
  font-style: italic;
}

.tiny {
  padding: 1px 10px;
  font-size: var(--fs-small);
  border-color: var(--border-strong);
}

/* "All" is the blunt instrument; it should not look like the default. */
.tiny.ghost {
  padding: 1px 7px;
  color: var(--text-muted);
  border-color: transparent;
}

.splitv {
  width: 1px;
  background: var(--border);
  flex: none;
}

.work {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.diff-header {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 26px;
  padding: 0 10px;
  background: var(--bg-toolbar);
  border-bottom: 1px solid var(--border);
  font-size: var(--fs-small);
  flex: none;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.error {
  padding: 4px 10px;
  color: var(--red);
  font-size: var(--fs-small);
  flex: none;
}

.diff-pane {
  flex: 1;
  min-height: 0;
}

.commit-box {
  flex: none;
  padding: 8px 10px;
  border-top: 1px solid var(--border);
  background: var(--bg-raised);
}

.message {
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
  overflow: hidden;
}

.message:focus-within {
  border-color: var(--accent);
}

.summary-row {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.summary,
.description {
  background: transparent;
  border: none;
  color: var(--text);
  font: inherit;
  padding: 5px 8px;
  outline: none;
  width: 100%;
}

.counter {
  padding-right: 8px;
  font-size: 11px;
  color: var(--text-dim);
}

.counter.warn {
  color: var(--amber);
}

.description {
  height: 52px;
  resize: none;
  display: block;
  font-family: var(--font-mono);
  font-size: var(--fs-small);
}

.commit-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.amend {
  color: var(--text-muted);
}

.primary {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
  padding: 5px 14px;
}

.primary:disabled {
  background: var(--bg-hover);
  border-color: var(--border);
  color: var(--text-dim);
}
</style>
