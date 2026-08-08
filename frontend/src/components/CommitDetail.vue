<script setup lang="ts">
import { ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { CommitFiles, Diff } from '@wails/go/main/App'
import { formatDate } from '@/utils/format'
import DiffView from '@/components/DiffView.vue'

const props = defineProps<{
  repoId: string
  commit: git.Commit | null
}>()

const files = ref<git.FileChange[]>([])
const selectedPath = ref('')
const diff = ref<git.FileDiff | null>(null)
const loadingFiles = ref(false)
const loadingDiff = ref(false)
const error = ref('')

/** Every fetch is stamped with the request it belongs to. */
let token = 0

async function loadFiles() {
  const mine = ++token
  files.value = []
  selectedPath.value = ''
  diff.value = null
  error.value = ''

  if (!props.commit) return

  loadingFiles.value = true
  try {
    const result = await CommitFiles(props.repoId, props.commit.hash)
    if (mine !== token) return
    files.value = result
    if (result.length) void selectFile(result[0])
  } catch (e) {
    if (mine === token) error.value = String(e)
  } finally {
    if (mine === token) loadingFiles.value = false
  }
}

async function selectFile(f: git.FileChange) {
  if (!props.commit) return
  const mine = token
  selectedPath.value = f.path

  loadingDiff.value = true
  diff.value = null
  try {
    const d = await Diff(props.repoId, {
      commit: props.commit.hash,
      // A deleted file has no path on the new side; diff it by its old one.
      path: f.path || f.oldPath || '',
      staged: false,
    } as git.DiffTarget)
    if (mine !== token) return
    diff.value = d
  } catch (e) {
    if (mine === token) error.value = String(e)
  } finally {
    if (mine === token) loadingDiff.value = false
  }
}

watch(() => props.commit?.hash, loadFiles, { immediate: true })

/** Single-letter status shown in the file list, as git spells it. */
function statusLabel(f: git.FileChange): string {
  return f.status || '?'
}
</script>

<template>
  <div class="detail">
    <div v-if="!commit" class="empty muted">select a commit</div>

    <template v-else>
      <header class="meta">
        <span class="mono hash">{{ commit.hash.slice(0, 10) }}</span>
        <span class="author">{{ commit.author.name }}</span>
        <span class="muted">&lt;{{ commit.author.email }}&gt;</span>
        <span class="muted">{{ formatDate(commit.author.when) }}</span>
        <span v-if="commit.parents.length > 1" class="badge">merge</span>
        <span class="subject">{{ commit.subject }}</span>
      </header>

      <pre v-if="commit.body" class="body">{{ commit.body }}</pre>

      <div v-if="error" class="error">{{ error }}</div>

      <div class="panes">
        <div class="files">
          <div v-if="loadingFiles" class="empty muted">loading…</div>
          <div v-else-if="!files.length" class="empty muted">no files changed</div>
          <div
            v-for="f in files"
            :key="f.path + f.oldPath"
            class="file"
            :class="{ selected: f.path === selectedPath }"
            @click="selectFile(f)"
          >
            <span class="status" :class="'s-' + statusLabel(f)">{{ statusLabel(f) }}</span>
            <span class="path" :title="f.oldPath ? `${f.oldPath} → ${f.path}` : f.path">
              {{ f.path }}
            </span>
            <span v-if="f.oldPath" class="renamed muted">← {{ f.oldPath }}</span>
          </div>
        </div>

        <div class="splitv"></div>

        <div class="diff-pane">
          <DiffView :diff="diff" :loading="loadingDiff" />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.detail {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--bg-raised);
}

.empty {
  padding: 10px 14px;
  font-style: italic;
  font-size: var(--fs-small);
}

.meta {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
  padding: 8px 14px 6px;
  user-select: text;
  flex: none;
}

.hash {
  color: var(--accent);
}

.author {
  font-weight: 600;
}

.subject {
  flex-basis: 100%;
  font-weight: 600;
}

.badge {
  padding: 0 5px;
  border-radius: 3px;
  font-size: 11px;
  color: var(--purple);
  border: 1px solid var(--purple);
}

.body {
  margin: 0 14px 8px;
  padding: 0;
  max-height: 120px;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: var(--fs-small);
  white-space: pre-wrap;
  color: var(--text-muted);
  user-select: text;
  flex: none;
}

.error {
  padding: 4px 14px;
  color: var(--red);
  font-size: var(--fs-small);
}

.panes {
  display: flex;
  flex: 1;
  min-height: 0;
  border-top: 1px solid var(--border);
}

.files {
  width: 280px;
  flex: none;
  overflow-y: auto;
  padding: 4px 0;
}

.file {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 2px 10px;
  height: 21px;
  white-space: nowrap;
  overflow: hidden;
  cursor: default;
  font-size: var(--fs-small);
}

.file:hover {
  background: var(--bg-hover);
}

.file.selected {
  background: var(--bg-selected);
}

.status {
  width: 12px;
  flex: none;
  font-family: var(--font-mono);
  font-weight: 600;
  text-align: center;
}

.s-A {
  color: var(--green);
}

.s-M {
  color: var(--amber);
}

.s-D {
  color: var(--red);
}

.s-R,
.s-C {
  color: var(--accent);
}

.path {
  overflow: hidden;
  text-overflow: ellipsis;
  /* Long paths matter at the end, not the start. */
  direction: rtl;
  text-align: left;
}

.renamed {
  font-size: 11px;
  flex: none;
}

.splitv {
  width: 1px;
  background: var(--border);
  flex: none;
}

.diff-pane {
  flex: 1;
  min-width: 0;
  min-height: 0;
}
</style>
