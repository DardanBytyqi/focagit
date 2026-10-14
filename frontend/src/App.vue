<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { git } from '@wails/go/models'
import { InitialRepo, InitialView } from '@wails/go/main/App'
import BranchDialog from '@/components/branch/BranchDialog.vue'
import DeleteBranchDialog from '@/components/branch/DeleteBranchDialog.vue'
import ChangesView from '@/components/changes/ChangesView.vue'
import ApplyCommitDialog from '@/components/history/ApplyCommitDialog.vue'
import HistoryPane from '@/components/history/HistoryPane.vue'
import ResetDialog from '@/components/history/ResetDialog.vue'
import ConflictView from '@/components/merge/ConflictView.vue'
import MergeDropMenu from '@/components/merge/MergeDropMenu.vue'
import OperationBar from '@/components/merge/OperationBar.vue'
import CloneDialog from '@/components/remote/CloneDialog.vue'
import CredentialsDialog from '@/components/remote/CredentialsDialog.vue'
import PushDialog from '@/components/remote/PushDialog.vue'
import RemoteDialog from '@/components/remote/RemoteDialog.vue'
import IdentityDialog from '@/components/settings/IdentityDialog.vue'
import SettingsDialog from '@/components/settings/SettingsDialog.vue'
import Sidebar from '@/components/shell/Sidebar.vue'
import DropStashDialog from '@/components/stash/DropStashDialog.vue'
import StashDialog from '@/components/stash/StashDialog.vue'
import StashView from '@/components/stash/StashView.vue'
import DeleteTagDialog from '@/components/tag/DeleteTagDialog.vue'
import TagDialog from '@/components/tag/TagDialog.vue'
import StatusBar from '@/components/shell/StatusBar.vue'
import TabBar from '@/components/shell/TabBar.vue'
import Toolbar from '@/components/shell/Toolbar.vue'
import Welcome from '@/components/shell/Welcome.vue'
import { useAppearance } from '@/composables/useAppearance'
import { useDialogs } from '@/composables/useDialogs'
import { usePrefs } from '@/composables/usePrefs'
import { useRepo } from '@/composables/useRepo'
import { useShortcuts } from '@/composables/useShortcuts'

const {
  state,
  refsByHash,
  open,
  switchTo,
  close,
  browse,
  refresh,
  checkout,
  checkoutCommit,
  fetchAll,
  push,
  merge,
  rebase,
  pull,
  stash,
  applyStash,
  popStash,
  pushTag,
} = useRepo()
const { state: prefsState, loadSignature } = usePrefs()
const { state: look, loadAppearance, setAppearance } = useAppearance()
const dialogs = useDialogs()
const {
  open: dialog,
  branchRequest,
  deleteRequest,
  mergeDrop,
  dropStashRequest,
  applyRequest,
  resetRequest,
  tagRequest,
  deleteTagRequest,
} = dialogs

/** The main area shows the history, the working tree, or one stash entry. */
const mode = ref<'commit' | 'changes' | 'stash'>('commit')
const changeCount = computed(() => state.status?.files?.length ?? 0)

const shownStash = ref<git.Stash | null>(null)

/** The commit the list should jump to; the nonce lets the same ref be clicked twice. */
const reveal = ref<{ hash: string; nonce: number } | null>(null)
let nonce = 0

// Clicking a ref shows where it points; double-clicking it checks it out.
function revealRef(r: git.Ref) {
  mode.value = 'commit'
  reveal.value = { hash: r.hash, nonce: ++nonce }
}

// The plain button takes everything, untracked included; the arrow is for the rest.
const doStash = () => stash({ message: '', untracked: true, keepIndex: false, paths: [] })

function showStash(s: git.Stash) {
  shownStash.value = s
  mode.value = 'stash'
}

// Dropped, popped, or gone from outside: there is nothing left on screen to act on.
watch(
  () => state.stashes,
  (list) => {
    const shown = shownStash.value
    if (!shown || list.some((s) => s.hash === shown.hash)) return
    shownStash.value = null
    if (mode.value === 'stash') mode.value = 'commit'
  },
)

// Restoring an entry puts the work back in the tree, which is where to look next.
async function restoreStash(s: git.Stash, keep: boolean) {
  const ok = keep ? await applyStash(s.ref, false) : await popStash(s.ref, false)
  if (ok) {
    shownStash.value = null
    mode.value = 'changes'
  }
}

// Who commits here is a property of the repository, so it is re-read on every open.
watch(() => state.info?.id ?? null, loadSignature, { immediate: true })

// Fetch always prunes: a stale sidebar is worse than a slower fetch.
const doFetch = () => fetchAll({ remote: '', prune: true, tags: false })
const doPull = () => pull({ remote: '', prune: true, noFastForward: false })

/** Push says nothing when it works; the window is only for decisions. */
async function doPush() {
  const res = await push({ remote: '', branch: '', setUpstream: false, force: false, tags: false })
  if (!res) dialog.push = true
}

const doMerge = (r: git.Ref) => merge({ ref: r.short, noFastForward: false, message: '' })

/** A drop is two steps when the target is not where HEAD already is. */
async function confirmMerge(noFastForward: boolean) {
  const req = mergeDrop.value
  mergeDrop.value = null
  if (!req) return

  if (!req.target.head && !(await checkout(req.target))) return
  await merge({ ref: req.source.short, noFastForward, message: '' })
}

const doRebase = (onto: string) => rebase({ onto })

/** Dropping a branch on another rebases the one it landed on, as merge does. */
async function confirmRebase() {
  const req = mergeDrop.value
  mergeDrop.value = null
  if (!req) return

  if (!req.target.head && !(await checkout(req.target))) return
  await rebase({ onto: req.source.short })
}

// A tag names a commit, so checking one out can only detach HEAD.
const checkoutTag = (t: git.Tag) => checkoutCommit(t.hash)

function revealTag(t: git.Tag) {
  mode.value = 'commit'
  reveal.value = { hash: t.hash, nonce: ++nonce }
}

// A reset moves the branch HEAD is on, and says which one before it does.
function askReset(commit: git.Commit) {
  resetRequest.value = { commit, branch: state.status?.head ?? '' }
}

// The toolbar branches from wherever HEAD is, named for what the user sees.
function branchFromHead() {
  const st = state.status
  dialogs.newBranch('HEAD', st && !st.detached ? st.head : 'HEAD')
}

/** The tab beside the current one, wrapping round at either end. */
function stepTab(by: number) {
  const tabs = state.tabs
  const i = tabs.findIndex((t) => t.id === state.info?.id)
  if (tabs.length > 1 && i >= 0) switchTo(tabs[(i + by + tabs.length) % tabs.length].id)
}

// Same bounds as the settings stepper and prefs.normalise.
const zoom = (size: number) =>
  setAppearance({ ...look.look, fontSize: Math.min(20, Math.max(10, size)) })

// Network actions wait for the one running, as their disabled buttons do.
const idle = () => !!state.info && !state.busy

useShortcuts({
  fetch: () => idle() && doFetch(),
  pull: () => idle() && doPull(),
  push: () => idle() && doPush(),
  branch: () => state.info && branchFromHead(),
  stash: () => state.info && changeCount.value > 0 && doStash(),
  history: () => state.info && (mode.value = 'commit'),
  changes: () => state.info && (mode.value = 'changes'),
  open: browse,
  closeTab: () => state.info && close(state.info.id),
  nextTab: () => stepTab(1),
  prevTab: () => stepTab(-1),
  settings: () => dialogs.showSettings(''),
  zoomIn: () => zoom(look.look.fontSize + 1),
  zoomOut: () => zoom(look.look.fontSize - 1),
  zoomReset: () => zoom(14),
  refresh: () => state.info && refresh(),
})

// The native window injects the Go bindings before our script runs.
async function bindingsReady(timeoutMs = 5000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (!(window as any).go?.main?.App) {
    if (Date.now() > deadline) return false
    await new Promise((r) => setTimeout(r, 50))
  }
  return true
}

onMounted(async () => {
  if (!(await bindingsReady())) {
    state.error = 'the Go bindings never became available'
    return
  }
  // Before anything is shown, so the window does not flash the wrong theme.
  await loadAppearance()

  const path = await InitialRepo()
  if (path) await open(path)
  // FOCAGIT_OPEN skips the welcome screen during development.
  if ((await InitialView()) === 'changes') mode.value = 'changes'
})
</script>

<template>
  <div class="app">
    <TabBar
      :tabs="state.tabs"
      :active-id="state.info?.id ?? ''"
      :busy="state.busy"
      @select="switchTo"
      @close="close"
      @open="browse"
      @clone="dialog.clone = true"
    />

    <Toolbar
      :info="state.info"
      :status="state.status"
      :signature="prefsState.signature"
      :busy="state.busy"
      @branch="branchFromHead"
      @identity="dialog.identity = true"
      @fetch="doFetch"
      @pull="doPull"
      @push="doPush"
      @push-options="dialog.push = true"
      @stash="doStash"
      @stash-options="dialog.stash = true"
      @settings="dialogs.showSettings"
    />

    <OperationBar v-if="state.status?.operation" @resolve="dialog.conflicts = true" />

    <div class="body">
      <Sidebar
        v-if="state.info"
        :refs="state.refs"
        :tags="state.tags"
        :stashes="state.stashes"
        :change-count="changeCount"
        :mode="mode"
        :head="state.status?.head ?? ''"
        @mode="mode = $event"
        @reveal="revealRef"
        @checkout="checkout"
        @merge="doMerge"
        @rebase="(r) => doRebase(r.short)"
        @merge-drop="(s, t, x, y) => (mergeDrop = { source: s, target: t, x, y })"
        @create="dialogs.newBranch"
        @rename="dialogs.renameBranch"
        @remove="deleteRequest = { name: $event }"
        @add-remote="dialog.remote = true"
        @reveal-tag="revealTag"
        @checkout-tag="checkoutTag"
        @branch-from-tag="(t) => dialogs.newBranch(t.name, t.name)"
        @push-tag="(t) => pushTag(t.name, false)"
        @delete-tag="deleteTagRequest = $event"
        @show-stash="showStash"
        @apply-stash="restoreStash($event, true)"
        @pop-stash="restoreStash($event, false)"
        @drop-stash="dropStashRequest = $event"
      />

      <main class="main">
        <template v-if="state.info">
          <ChangesView
            v-if="mode === 'changes'"
            :repo-id="state.info.id"
            :status="state.status"
            @changed="refresh"
          />
          <StashView
            v-else-if="mode === 'stash' && shownStash"
            :repo-id="state.info.id"
            :stash="shownStash"
            @pop="restoreStash(shownStash, false)"
            @apply="restoreStash(shownStash, true)"
            @drop="dropStashRequest = shownStash"
          />
          <HistoryPane
            v-else
            :repo-id="state.info.id"
            :count="state.count"
            :width="state.width"
            :gen="state.gen"
            :refs-by-hash="refsByHash"
            :head="state.status?.head ?? ''"
            :head-hash="state.status?.oid ?? ''"
            :reveal="reveal"
            @branch="dialogs.newBranch"
            @checkout="checkoutCommit"
            @tag="tagRequest = { commit: $event }"
            @revert="applyRequest = { kind: 'revert', commit: $event }"
            @cherry-pick="applyRequest = { kind: 'cherry-pick', commit: $event }"
            @reset="askReset"
            @rebase="doRebase"
          />
        </template>

        <Welcome v-else />
      </main>
    </div>

    <StatusBar
      :info="state.info"
      :status="state.status"
      :count="state.count"
      :loading="state.loading"
      :error="dialogs.anyOpen.value ? '' : state.error"
      :network="state.network"
    />

    <CloneDialog :open="dialog.clone" @close="dialog.clone = false" />
    <CredentialsDialog />
    <BranchDialog :request="branchRequest" @close="branchRequest = null" />
    <DeleteBranchDialog :request="deleteRequest" @close="deleteRequest = null" />
    <IdentityDialog
      :open="dialog.identity"
      :repo-id="state.info?.id ?? null"
      :repo-name="state.info?.name ?? ''"
      @close="dialog.identity = false"
    />
    <PushDialog :open="dialog.push" @close="dialog.push = false" @ssh="dialogs.showSSH" />
    <SettingsDialog
      :open="dialog.settings"
      :section="dialogs.settingsSection.value"
      @close="dialog.settings = false"
    />
    <RemoteDialog :open="dialog.remote" @close="dialog.remote = false" />
    <StashDialog :open="dialog.stash" @close="dialog.stash = false" />
    <DropStashDialog :stash="dropStashRequest" @close="dropStashRequest = null" />
    <ApplyCommitDialog :request="applyRequest" @close="applyRequest = null" />
    <ResetDialog :request="resetRequest" @close="resetRequest = null" />
    <TagDialog :request="tagRequest" @close="tagRequest = null" />
    <DeleteTagDialog :tag="deleteTagRequest" @close="deleteTagRequest = null" />
    <MergeDropMenu
      :drop="mergeDrop"
      @merge="confirmMerge"
      @rebase="confirmRebase"
      @close="mergeDrop = null"
    />
    <ConflictView :open="dialog.conflicts" @close="dialog.conflicts = false" />
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
</style>
