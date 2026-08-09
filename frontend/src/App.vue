<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { InitialRepo, InitialView } from '@wails/go/main/App'
import BranchDialog from '@/components/BranchDialog.vue'
import ChangesView from '@/components/ChangesView.vue'
import CommitDetail from '@/components/CommitDetail.vue'
import CommitList from '@/components/CommitList.vue'
import DeleteBranchDialog from '@/components/DeleteBranchDialog.vue'
import IdentityDialog from '@/components/IdentityDialog.vue'
import PushDialog from '@/components/PushDialog.vue'
import RemoteDialog from '@/components/RemoteDialog.vue'
import RecentList from '@/components/RecentList.vue'
import Sidebar from '@/components/Sidebar.vue'
import StatusBar from '@/components/StatusBar.vue'
import Toolbar from '@/components/Toolbar.vue'
import type { BranchRequest, DeleteRequest } from '@/types'
import { usePrefs } from './composables/usePrefs'
import { useRepo } from './composables/useRepo'

const { state, open, browse, refresh, initHere, checkout, checkoutCommit, fetchAll, push } =
  useRepo()
const { state: prefsState, loadSignature } = usePrefs()

const identityOpen = ref(false)
const pushOpen = ref(false)
const remoteOpen = ref(false)
const fetching = ref(false)
const pushing = ref(false)

/** Push says nothing when it works; the dialog is only for decisions. */
async function doPush() {
  pushing.value = true
  const res = await push({ remote: '', branch: '', setUpstream: false, force: false, tags: false })
  pushing.value = false
  if (!res) pushOpen.value = true
}

// Pruning on every fetch: a stale sidebar is worse than a slower fetch.
async function doFetch() {
  fetching.value = true
  await fetchAll({ remote: '', prune: true, tags: false })
  fetching.value = false
}

// Who commits here is a property of the repository, so it is re-read on every open.
watch(() => state.info?.id ?? null, loadSignature, { immediate: true })

/** A message shown inside a dialog does not need repeating at the bottom. */
const dialogOpen = computed(() => identityOpen.value || pushOpen.value || remoteOpen.value)

// One dialog at a time; null means closed.
const branchRequest = ref<BranchRequest | null>(null)
const deleteRequest = ref<DeleteRequest | null>(null)

function newBranch(start: string, startLabel: string) {
  branchRequest.value = { kind: 'create', start, startLabel }
}

function renameBranch(from: string) {
  branchRequest.value = { kind: 'rename', from }
}

// The toolbar branches from wherever HEAD is, named for what the user sees.
function branchFromHead() {
  const st = state.status
  newBranch('HEAD', st && !st.detached ? st.head : 'HEAD')
}

// Horizontal splitter between the history and the commit detail pane.
const detailHeight = ref(340)
let dragging = false

const selected = ref<git.Commit | null>(null)

/** The lower pane shows either a commit or the working tree. */
const mode = ref<'commit' | 'changes'>('commit')

const changeCount = computed(() => state.status?.files?.length ?? 0)

function showCommit(c: git.Commit) {
  selected.value = c
  mode.value = 'commit'
}

function setMode(m: 'commit' | 'changes') {
  mode.value = m
}

// The native window injects the Go bindings before our script runs.
async function bindingsReady(timeoutMs = 5000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (!(window as any).go?.main?.App) {
    if (Date.now() > deadline) return false
    await new Promise((r) => setTimeout(r, 50))
  }
  return true
}

// FOCAGIT_OPEN skips the welcome screen during development.
onMounted(async () => {
  if (!(await bindingsReady())) {
    state.error = 'the Go bindings never became available'
    return
  }
  const path = await InitialRepo()
  if (path) await open(path)
  if ((await InitialView()) === 'changes') setMode('changes')
})

// Refs indexed by the commit they point at.
const refsByHash = computed(() => {
  const m = new Map<string, git.Ref[]>()
  for (const r of state.refs) {
    const list = m.get(r.hash)
    if (list) list.push(r)
    else m.set(r.hash, [r])
  }
  return m
})


function startDrag(e: MouseEvent) {
  dragging = true
  const startY = e.clientY
  const startHeight = detailHeight.value

  const onMove = (ev: MouseEvent) => {
    if (!dragging) return
    // Clamped so neither pane can be dragged out of existence.
    detailHeight.value = Math.min(600, Math.max(80, startHeight - (ev.clientY - startY)))
  }
  const onUp = () => {
    dragging = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }

  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}
</script>

<template>
  <div class="app">
    <Toolbar
      :info="state.info"
      :status="state.status"
      :signature="prefsState.signature"
      :busy="fetching || pushing"
      @browse="browse"
      @refresh="refresh"
      @branch="branchFromHead"
      @identity="identityOpen = true"
      @fetch="doFetch"
      @push="doPush"
      @push-options="pushOpen = true"
    />

    <div class="body">
      <Sidebar
        v-if="state.info"
        :refs="state.refs"
        :change-count="changeCount"
        :mode="mode"
        @mode="setMode"
        @checkout="checkout"
        @create="newBranch"
        @rename="renameBranch"
        @remove="deleteRequest = { name: $event }"
        @add-remote="remoteOpen = true"
      />

      <main class="main">
        <template v-if="state.info">
          <ChangesView
            v-if="mode === 'changes'"
            :repo-id="state.info.id"
            :status="state.status"
            @changed="refresh"
          />

          <template v-else>
            <div class="history">
              <CommitList
                :repo-id="state.info.id"
                :count="state.count"
                :width="state.width"
                :gen="state.gen"
                :refs-by-hash="refsByHash"
                @select="showCommit"
                @branch="newBranch"
                @checkout="checkoutCommit"
              />
            </div>

            <div class="splitter" @mousedown.prevent="startDrag"></div>

            <section class="detail" :style="{ height: detailHeight + 'px' }">
              <CommitDetail :repo-id="state.info.id" :commit="selected" />
            </section>
          </template>
        </template>

        <div v-else class="welcome">
          <h1>focagit</h1>

          <template v-if="state.initCandidate">
            <p class="muted">
              <span class="mono">{{ state.initCandidate }}</span> is not a repository yet.
            </p>
            <div class="row">
              <button class="primary" @click="initHere">Initialise repository here</button>
              <button @click="browse">Choose another folder…</button>
            </div>
          </template>

          <template v-else>
            <p class="muted">Open a repository to get started.</p>
            <button class="primary" @click="browse">Open repository…</button>
            <RecentList @open="open" />
          </template>

          <p v-if="state.error" class="error">{{ state.error }}</p>
        </div>
      </main>
    </div>

    <StatusBar
      :info="state.info"
      :status="state.status"
      :count="state.count"
      :loading="state.loading"
      :error="dialogOpen ? '' : state.error"
      :network="state.network"
    />

    <BranchDialog :request="branchRequest" @close="branchRequest = null" />
    <DeleteBranchDialog :request="deleteRequest" @close="deleteRequest = null" />
    <IdentityDialog
      :open="identityOpen"
      :repo-id="state.info?.id ?? null"
      :repo-name="state.info?.name ?? ''"
      @close="identityOpen = false"
    />
    <PushDialog :open="pushOpen" @close="pushOpen = false" />
    <RemoteDialog :open="remoteOpen" @close="remoteOpen = false" />
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.history {
  flex: 1;
  min-height: 0;
}

.splitter {
  height: 5px;
  background: var(--border);
  cursor: row-resize;
  flex: none;
}

.splitter:hover {
  background: var(--accent);
}

.detail {
  flex: none;
  min-height: 0;
  overflow: hidden;
}

.welcome {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.welcome .row {
  display: flex;
  gap: 8px;
}

.welcome .mono {
  font-family: var(--font-mono);
  color: var(--text);
}

.welcome h1 {
  margin: 0;
  font-size: 30px;
  font-weight: 300;
  letter-spacing: 0.06em;
}

.primary {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
  padding: 7px 16px;
}

.primary:hover {
  filter: brightness(1.1);
  background: var(--accent);
  border-color: var(--accent);
}

.error {
  color: var(--red);
  max-width: 60ch;
  text-align: center;
}
</style>
