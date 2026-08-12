<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { InitialRepo, InitialView } from '@wails/go/main/App'
import BranchDialog from '@/components/branch/BranchDialog.vue'
import DeleteBranchDialog from '@/components/branch/DeleteBranchDialog.vue'
import ChangesView from '@/components/changes/ChangesView.vue'
import HistoryPane from '@/components/history/HistoryPane.vue'
import ConflictView from '@/components/merge/ConflictView.vue'
import MergeBar from '@/components/merge/MergeBar.vue'
import MergeDropMenu from '@/components/merge/MergeDropMenu.vue'
import CloneDialog from '@/components/remote/CloneDialog.vue'
import PushDialog from '@/components/remote/PushDialog.vue'
import RemoteDialog from '@/components/remote/RemoteDialog.vue'
import IdentityDialog from '@/components/settings/IdentityDialog.vue'
import SettingsDialog from '@/components/settings/SettingsDialog.vue'
import Sidebar from '@/components/shell/Sidebar.vue'
import StatusBar from '@/components/shell/StatusBar.vue'
import Toolbar from '@/components/shell/Toolbar.vue'
import Welcome from '@/components/shell/Welcome.vue'
import { useAppearance } from '@/composables/useAppearance'
import { useDialogs } from '@/composables/useDialogs'
import { usePrefs } from '@/composables/usePrefs'
import { useRepo } from '@/composables/useRepo'

const { state, refsByHash, open, browse, refresh, checkout, checkoutCommit, fetchAll, push, merge, pull } =
  useRepo()
const { state: prefsState, loadSignature } = usePrefs()
const { loadAppearance } = useAppearance()
const dialogs = useDialogs()
const { open: dialog, branchRequest, deleteRequest, mergeDrop } = dialogs

/** The lower pane shows either a commit or the working tree. */
const mode = ref<'commit' | 'changes'>('commit')
const changeCount = computed(() => state.status?.files?.length ?? 0)

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

// The toolbar branches from wherever HEAD is, named for what the user sees.
function branchFromHead() {
  const st = state.status
  dialogs.newBranch('HEAD', st && !st.detached ? st.head : 'HEAD')
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
    <Toolbar
      :info="state.info"
      :status="state.status"
      :signature="prefsState.signature"
      :busy="state.busy"
      @browse="browse"
      @clone="dialog.clone = true"
      @branch="branchFromHead"
      @identity="dialog.identity = true"
      @fetch="doFetch"
      @pull="doPull"
      @push="doPush"
      @push-options="dialog.push = true"
      @settings="dialogs.showSettings"
    />

    <MergeBar v-if="state.status?.merging" @resolve="dialog.conflicts = true" />

    <div class="body">
      <Sidebar
        v-if="state.info"
        :refs="state.refs"
        :change-count="changeCount"
        :mode="mode"
        :head="state.status?.head ?? ''"
        @mode="mode = $event"
        @checkout="checkout"
        @merge="doMerge"
        @merge-drop="(s, t, x, y) => (mergeDrop = { source: s, target: t, x, y })"
        @create="dialogs.newBranch"
        @rename="dialogs.renameBranch"
        @remove="deleteRequest = { name: $event }"
        @add-remote="dialog.remote = true"
      />

      <main class="main">
        <template v-if="state.info">
          <ChangesView
            v-if="mode === 'changes'"
            :repo-id="state.info.id"
            :status="state.status"
            @changed="refresh"
          />
          <HistoryPane
            v-else
            :repo-id="state.info.id"
            :count="state.count"
            :width="state.width"
            :gen="state.gen"
            :refs-by-hash="refsByHash"
            @branch="dialogs.newBranch"
            @checkout="checkoutCommit"
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
    <MergeDropMenu :drop="mergeDrop" @merge="confirmMerge" @close="mergeDrop = null" />
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
