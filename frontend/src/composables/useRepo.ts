import { reactive } from 'vue'
import type { git, session } from '@wails/go/models'
import {
  Checkout,
  CheckoutCommit,
  CreateBranch,
  DeleteBranch,
  GetHistory,
  InitRepo,
  Inspect,
  OpenRepo,
  Refs,
  RenameBranch,
  SelectRepo,
  Status,
} from '@wails/go/main/App'
import { EventsOn } from '@wails/runtime/runtime'
import { errorText } from '@/utils/errors'

/** The frontend's whole picture of the open repository. */
interface State {
  info: session.Info | null
  count: number
  /** Lanes the graph column needs; grows as more of the history arrives. */
  width: number
  /** Changes whenever the history is walked again, invalidating cached pages. */
  gen: number
  loading: boolean
  refs: git.Ref[]
  status: git.Status | null
  error: string
  /** Set when the chosen folder exists but holds no repository yet. */
  initCandidate: string
}

const state = reactive<State>({
  info: null,
  count: 0,
  width: 1,
  gen: 0,
  loading: false,
  refs: [],
  status: null,
  error: '',
  initCandidate: '',
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

async function open(path: string) {
  state.error = ''
  state.initCandidate = ''
  try {
    state.info = await OpenRepo(path)
    state.count = 0
    state.width = 1
    state.loading = true
    state.refs = []
    state.status = null
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
    state.info = await InitRepo(path)
    state.initCandidate = ''
    state.count = 0
    state.width = 1
    state.refs = []
    state.status = null
    await refresh()
  } catch (e) {
    state.error = String(e)
  }
}

async function browse() {
  const path = await SelectRepo()
  if (path) await open(path)
}

/** Re-reads refs and working tree status. */
async function refresh() {
  if (!state.info) return
  try {
    const [refs, status, history] = await Promise.all([
      Refs(state.info.id),
      Status(state.info.id),
      GetHistory(state.info.id),
    ])
    state.refs = refs
    state.status = status
    state.gen = history.gen
    state.count = history.count
    state.width = Math.max(1, history.width)
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

const checkout = (ref: git.Ref) => act((id) => Checkout(id, ref))
const checkoutCommit = (hash: string) => act((id) => CheckoutCommit(id, hash))
const createBranch = (opts: git.CreateBranchOptions) => act((id) => CreateBranch(id, opts))
const deleteBranch = (name: string, force: boolean) => act((id) => DeleteBranch(id, name, force))
const renameBranch = (from: string, to: string) => act((id) => RenameBranch(id, from, to))

export function useRepo() {
  return {
    state,
    open,
    browse,
    refresh,
    initHere,
    checkout,
    checkoutCommit,
    createBranch,
    deleteBranch,
    renameBranch,
  }
}
