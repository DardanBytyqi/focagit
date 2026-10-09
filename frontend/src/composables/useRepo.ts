import { computed, reactive } from 'vue'
import type { git, session } from '@wails/go/models'
import {
  AbortOperation,
  AddRemote,
  ApplyStash,
  CancelClone,
  Checkout,
  CheckoutCommit,
  CherryPick,
  CloneRepo,
  CloseRepo,
  ConflictText,
  Conflicts,
  CreateBranch,
  CreateTag,
  DeleteBranch,
  DeleteTag,
  DropStash,
  Fetch,
  FinishOperation,
  GetHistory,
  GetRemotes,
  InitRepo,
  Inspect,
  Merge,
  PreviewMerge,
  PreviewReset,
  OpenRepo,
  PopStash,
  Pull,
  Push,
  PushTag,
  Rebase,
  Refs,
  RemoveRemote,
  RenameBranch,
  Reset,
  ResolveConflict,
  ResolveLines,
  Revert,
  SkipOperation,
  SelectRepo,
  StageFiles,
  Stash,
  Stashes,
  Status,
  Tags,
} from '@wails/go/main/App'
import { EventsOn } from '@wails/runtime/runtime'
import { errorText } from '@/utils/errors'

/** The frontend's whole picture of the open repository. */
interface State {
  info: session.Info | null
  /** Every open repository, in the order their tabs were opened. */
  tabs: session.Info[]
  count: number
  /** Lanes the graph column needs; grows as more of the history arrives. */
  width: number
  /** Changes whenever the history is walked again, invalidating cached pages. */
  gen: number
  loading: boolean
  refs: git.Ref[]
  tags: git.Tag[]
  stashes: git.Stash[]
  status: git.Status | null
  error: string
  /** Set when the chosen folder exists but holds no repository yet. */
  initCandidate: string
  remotes: git.Remote[]
  /** git's own progress line while a fetch or push runs; empty when idle. */
  network: string
  /** The unmerged files, refreshed with everything else while a merge is open. */
  conflicts: git.Conflict[]
  /** True while a network or merge operation runs, so it cannot be started twice. */
  busy: boolean
}

const state = reactive<State>({
  info: null,
  tabs: [],
  count: 0,
  width: 1,
  gen: 0,
  loading: false,
  refs: [],
  tags: [],
  stashes: [],
  status: null,
  error: '',
  initCandidate: '',
  remotes: [],
  network: '',
  conflicts: [],
  busy: false,
})

interface Progress {
  id: string
  gen: number
  loaded: number
  width: number
  done: boolean
  error?: string
}

// Go pushes progress as the history streams in.
EventsOn('repo:progress', (p: Progress) => {
  if (!state.info || p.id !== state.info.id) return

  state.gen = p.gen
  state.count = p.loaded
  state.width = Math.max(1, p.width)
  state.loading = !p.done
  if (p.error) state.error = p.error

  // Refs and status are only worth reading once the walk is done.
  if (p.done) void refresh()
})

interface NetworkEvent {
  id: string
  op: string
  line: string
  done: boolean
  error: string
}

// Fetch and push report what they are doing while they do it.
EventsOn('repo:network', (n: NetworkEvent) => {
  // A clone has no repository yet, so it carries no id.
  if (n.id && n.id !== state.info?.id) return
  state.network = n.done ? '' : n.line
})

// Something moved on disk, made by us or by anything else on the machine.
EventsOn('repo:changed', (c: { id: string }) => {
  if (c.id !== state.info?.id) return
  void refresh()
})

/** Clears what belonged to the repository we are leaving. */
function reset() {
  state.count = 0
  state.width = 1
  state.refs = []
  state.tags = []
  state.stashes = []
  state.status = null
  state.conflicts = []
}

/** Makes info the shown repository, adding its tab the first time. */
function adopt(info: session.Info) {
  state.info = info
  if (!state.tabs.some((t) => t.id === info.id)) state.tabs.push(info)
}

/** Shows an already open repository; Go still holds its history. */
async function switchTo(id: string) {
  if (state.info?.id === id) return
  const tab = state.tabs.find((t) => t.id === id)
  if (!tab) return

  state.info = tab
  state.error = ''
  reset()
  await refresh()
}

/** Closes one repository, freeing the cache Go holds for it. */
async function close(id: string) {
  const i = state.tabs.findIndex((t) => t.id === id)
  if (i < 0) return

  state.tabs.splice(i, 1)
  void CloseRepo(id)
  if (state.info?.id !== id) return

  // The neighbour on the right, or the one on the left when this was the last.
  const next = state.tabs[i] ?? state.tabs[i - 1]
  state.info = next ?? null
  reset()
  if (next) await refresh()
}

async function open(path: string) {
  state.error = ''
  state.initCandidate = ''
  try {
    adopt(await OpenRepo(path))
    reset()
    state.loading = true
    // A repository already open returns immediately with its history cached and emits no.
    await refresh()
  } catch (e) {
    state.error = String(e)
    state.info = null
    state.loading = false

    // A plain folder is not a failure, it is an offer.
    const info = await Inspect(path)
    if (info.exists && info.isDir && !info.isRepo) {
      state.initCandidate = path
      state.error = ''
    }
  }
}

/** Runs `git init` in the folder the last open attempt rejected. */
async function initHere() {
  const path = state.initCandidate
  if (!path) return
  state.error = ''
  try {
    adopt(await InitRepo(path))
    state.initCandidate = ''
    reset()
    await refresh()
  } catch (e) {
    state.error = String(e)
  }
}

async function browse() {
  const path = await SelectRepo()
  if (path) await open(path)
}

/** Clones and opens the result; git's refusal is left in state.error. */
async function clone(opts: git.CloneOptions): Promise<boolean> {
  state.error = ''
  state.initCandidate = ''
  return working(async () => {
    try {
      adopt(await CloneRepo(opts))
      reset()
      state.loading = true
      await refresh()
      return true
    } catch (e) {
      state.error = errorText(e)
      return false
    }
  })
}

/** Stops a clone in progress; nothing of it is left on disk. */
const cancelClone = () => CancelClone()

/** Re-reads refs and working tree status. */
async function refresh() {
  if (!state.info) return
  try {
    const [refs, tags, stashes, status, history] = await Promise.all([
      Refs(state.info.id),
      Tags(state.info.id),
      Stashes(state.info.id),
      Status(state.info.id),
      GetHistory(state.info.id),
    ])
    state.refs = refs
    state.tags = tags
    state.stashes = stashes
    state.status = status
    state.gen = history.gen
    state.count = history.count
    state.width = Math.max(1, history.width)
    // Only an operation in progress can have conflicts, so nothing else is asked.
    state.conflicts = status.operation ? await Conflicts(state.info.id) : []
  } catch (e) {
    state.error = String(e)
  }
}

/**
 * Runs a write operation, then re-reads what it may have changed.
 * Returns false and leaves the message in state.error when git refused.
 */
async function act(fn: (id: string) => Promise<void>): Promise<boolean> {
  if (!state.info) return false
  state.error = ''
  try {
    await fn(state.info.id)
    await refresh()
    return true
  } catch (e) {
    state.error = errorText(e)
    return false
  }
}

/** Marks the repository busy for as long as a long operation runs. */
async function working<T>(fn: () => Promise<T>): Promise<T> {
  state.busy = true
  try {
    return await fn()
  } finally {
    state.busy = false
  }
}

/** Refs indexed by the commit they point at, for the graph to label rows. */
const refsByHash = computed(() => {
  const m = new Map<string, git.Ref[]>()
  for (const r of state.refs) {
    const list = m.get(r.hash)
    if (list) list.push(r)
    else m.set(r.hash, [r])
  }
  return m
})

/** Reads the remotes; the push dialog needs them before it can offer a choice. */
async function loadRemotes() {
  if (!state.info) return
  try {
    state.remotes = await GetRemotes(state.info.id)
  } catch (e) {
    state.error = errorText(e)
  }
}

const fetchAll = (opts: git.FetchOptions) => working(() => act((id) => Fetch(id, opts)))

/** Adds a remote, then re-reads the list the push dialog offers. */
async function addRemote(name: string, url: string, fetchNow: boolean): Promise<boolean> {
  const ok = await act((id) => AddRemote(id, name, url, fetchNow))
  await loadRemotes()
  return ok
}

const removeRemote = (name: string) => act((id) => RemoveRemote(id, name))

/**
 * Pushes and returns what the remote made of each ref, or null when it refused.
 * The refusal is left in state.error, as with every other write.
 */
async function push(opts: git.PushOptions): Promise<git.PushResult[] | null> {
  if (!state.info) return null
  state.error = ''
  return working(async () => {
    try {
      const res = await Push(state.info!.id, opts)
      await refresh()
      return res
    } catch (e) {
      state.error = errorText(e)
      return null
    }
  })
}

/** Merges and returns what it turned out to be, or null when git refused. */
async function merge(opts: git.MergeOptions): Promise<git.MergeResult | null> {
  return working(() => run((id) => Merge(id, opts)))
}

/** One conflicted file cut into the blocks that disagree. */
async function conflictText(path: string): Promise<git.ConflictText | null> {
  if (!state.info) return null
  try {
    return await ConflictText(state.info.id, path)
  } catch (e) {
    state.error = errorText(e)
    return null
  }
}

const resolveLines = (path: string, picks: string[]) => act((id) => ResolveLines(id, path, picks))

/** Accepts the file as it stands, for a conflict already settled by hand. */
const markResolved = (path: string) => act((id) => StageFiles(id, [path]))

/** What a merge would do, for a menu that should only offer real choices. */
async function previewMerge(source: string, target: string): Promise<git.MergePreview | null> {
  if (!state.info) return null
  try {
    return await PreviewMerge(state.info.id, source, target)
  } catch (e) {
    state.error = errorText(e)
    return null
  }
}

/** Pull is a fetch and a merge, so it can refuse for either one's reasons. */
async function pull(opts: git.PullOptions): Promise<git.MergeResult | null> {
  return working(() => run((id) => Pull(id, opts)))
}

/** act for calls that return something worth keeping. */
async function run<T>(fn: (id: string) => Promise<T>): Promise<T | null> {
  if (!state.info) return null
  state.error = ''
  try {
    const res = await fn(state.info.id)
    await refresh()
    return res
  } catch (e) {
    state.error = errorText(e)
    return null
  }
}

/** Rebases and returns what it turned out to be, or null when git refused. */
async function rebase(opts: git.RebaseOptions): Promise<git.RebaseResult | null> {
  return working(() => run((id) => Rebase(id, opts)))
}

const abortOperation = () => act((id) => AbortOperation(id))
const finishOperation = (message: string) => act((id) => FinishOperation(id, message))
const skipOperation = () => act((id) => SkipOperation(id))
const resolveConflict = (path: string, side: 'ours' | 'theirs') =>
  act((id) => ResolveConflict(id, path, side))

/** Both replay a commit, so both can end in the same conflicted state. */
const revert = (opts: git.ApplyOptions) => run((id) => Revert(id, opts))
const cherryPick = (opts: git.ApplyOptions) => run((id) => CherryPick(id, opts))

const resetBranch = (rev: string, mode: string) => act((id) => Reset(id, rev, mode))

/** What the branch would give up, for a window that has to warn before it moves. */
async function previewReset(rev: string): Promise<git.ResetPreview | null> {
  if (!state.info) return null
  try {
    return await PreviewReset(state.info.id, rev)
  } catch (e) {
    state.error = errorText(e)
    return null
  }
}

const stash = (opts: git.StashOptions) => act((id) => Stash(id, opts))
/** index restores what was staged as staged; git refuses the whole thing if it cannot. */
const applyStash = (ref: string, index: boolean) => act((id) => ApplyStash(id, ref, index))
const popStash = (ref: string, index: boolean) => act((id) => PopStash(id, ref, index))
const dropStash = (ref: string) => act((id) => DropStash(id, ref))

const createTag = (opts: git.TagOptions) => act((id) => CreateTag(id, opts))
const deleteTag = (name: string) => act((id) => DeleteTag(id, name))

/** Pushing a tag reaches the network, so it reports like every other push. */
async function pushTag(name: string, remove: boolean): Promise<git.PushResult[] | null> {
  if (!state.info) return null
  state.error = ''
  return working(async () => {
    try {
      const res = await PushTag(state.info!.id, '', name, remove)
      await refresh()
      return res
    } catch (e) {
      state.error = errorText(e)
      return null
    }
  })
}

const checkout = (ref: git.Ref) => act((id) => Checkout(id, ref))
const checkoutCommit = (hash: string) => act((id) => CheckoutCommit(id, hash))
const createBranch = (opts: git.CreateBranchOptions) => act((id) => CreateBranch(id, opts))
const deleteBranch = (name: string, force: boolean) => act((id) => DeleteBranch(id, name, force))
const renameBranch = (from: string, to: string) => act((id) => RenameBranch(id, from, to))

export function useRepo() {
  return {
    state,
    refsByHash,
    open,
    switchTo,
    close,
    browse,
    refresh,
    initHere,
    clone,
    cancelClone,
    checkout,
    checkoutCommit,
    createBranch,
    deleteBranch,
    renameBranch,
    createTag,
    deleteTag,
    pushTag,
    stash,
    applyStash,
    popStash,
    dropStash,
    loadRemotes,
    addRemote,
    removeRemote,
    fetchAll,
    push,
    merge,
    rebase,
    previewMerge,
    pull,
    revert,
    cherryPick,
    resetBranch,
    previewReset,
    abortOperation,
    finishOperation,
    skipOperation,
    resolveConflict,
    conflictText,
    resolveLines,
    markResolved,
  }
}
