<script setup lang="ts">
import { ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { CommitFiles, Diff } from '@wails/go/main/App'
import { formatDate, reflow } from '@/utils/format'
import AuthorMark from '@/components/common/AuthorMark.vue'
import DiffView from '@/components/common/DiffView.vue'

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
      <!-- Facts and files on the left, the diff on the right at full height. -->
      <div class="side">
        <!-- The subject on top, then every fact in one aligned list of the same size and font. -->
        <header class="meta">
          <div class="subject">
            <span class="subject-text">{{ commit.subject }}</span>
            <span v-if="commit.parents.length > 1" class="badge">merge</span>
          </div>
          <pre v-if="commit.body" class="body">{{ reflow(commit.body) }}</pre>

          <dl class="info">
            <dt>Author</dt>
            <dd class="who">
              <AuthorMark :name="commit.author.name" :email="commit.author.email" :size="16" />
              {{ commit.author.name }} <span class="email">{{ commit.author.email }}</span>
            </dd>
            <dt>Commit</dt>
            <dd>{{ commit.hash.slice(0, 10) }}</dd>
            <dt>Date</dt>
            <dd>{{ formatDate(commit.author.when) }}</dd>
            <dt>{{ commit.parents.length > 1 ? 'Parents' : 'Parent' }}</dt>
            <dd>
              <span v-for="p in commit.parents" :key="p" class="parent">{{ p.slice(0, 7) }}</span>
              <span v-if="!commit.parents.length" class="none">none</span>
            </dd>
          </dl>
        </header>

        <div v-if="error" class="error">{{ error }}</div>

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
              <span dir="ltr">{{ f.path }}</span>
            </span>
            <span v-if="f.oldPath" class="renamed muted">← {{ f.oldPath }}</span>
          </div>
        </div>
      </div>

      <div class="splitv"></div>

      <div class="diff-pane">
        <DiffView :diff="diff" :loading="loadingDiff" />
      </div>
    </template>
  </div>
</template>

<style scoped>
.detail {
  display: flex;
  height: 100%;
  min-height: 0;
  background: var(--bg);
}

.empty {
  padding: 12px 16px;
  font-size: var(--fs-small);
}

.side {
  width: 50%;
  flex: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--bg-shell);
}

.meta {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px 16px;
  user-select: text;
  /* Shrinks and scrolls before the file list drops below a few rows. */
  flex: 0 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.subject {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
  font-size: calc(var(--fs) + 2px);
  font-weight: 600;
  line-height: 1.3;
}

.subject-text {
  min-width: 0;
  overflow-wrap: anywhere;
}

.badge {
  flex: none;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: calc(var(--fs-small) - 1px);
  font-weight: 500;
  color: var(--purple);
  background: color-mix(in srgb, var(--purple) 15%, transparent);
}

.body {
  margin: -6px 0 0;
  padding: 0;
  font-family: var(--font);
  font-size: var(--fs-small);
  line-height: 1.5;
  white-space: pre-wrap;
  color: var(--text-muted);
  user-select: text;
}

/* Label and value in two aligned columns; only the subject is bigger. */
.info {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  column-gap: 20px;
  row-gap: 6px;
  margin: 0;
  padding-top: 14px;
  border-top: 1px solid var(--border);
  font-size: var(--fs-small);
  font-variant-numeric: tabular-nums;
}

.info dt {
  color: var(--text-dim);
}

.info dd {
  margin: 0;
  min-width: 0;
  color: var(--text);
}

.who {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.who .email {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.email {
  margin-left: 6px;
  color: var(--text-dim);
}

.parent + .parent {
  margin-left: 12px;
}

.none {
  color: var(--text-dim);
}

.error {
  padding: 4px 14px;
  color: var(--red);
  font-size: var(--fs-small);
}

.files {
  flex: 1 0 140px;
  overflow-y: auto;
  padding: 6px;
  border-top: 1px solid var(--border);
}

.file {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 8px;
  height: 28px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  overflow: hidden;
  cursor: default;
  font-size: var(--fs-small);
}

.file:hover {
  background: var(--bg-hover);
}

.file.selected {
  background: var(--accent-soft);
}

/* The kind of change as git prints it: one coloured letter. */
.status {
  width: 12px;
  flex: none;
  font-family: var(--font-mono);
  font-size: var(--fs-small);
  font-weight: 700;
  text-align: center;
  color: var(--c, var(--text-muted));
}

.s-A {
  --c: var(--green);
}

.s-M {
  --c: var(--amber);
}

.s-D {
  --c: var(--red);
}

.s-R,
.s-C {
  --c: var(--blue);
}

.path {
  overflow: hidden;
  text-overflow: ellipsis;
  /* Long paths matter at the end, not the start; the inner ltr span keeps a leading dot in place. */
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

.path {
  color: var(--text);
}

.diff-pane {
  flex: 1;
  min-width: 0;
  min-height: 0;
}
</style>
