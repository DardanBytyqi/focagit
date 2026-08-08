import { reactive } from 'vue'
import type { git, prefs } from '@wails/go/models'
import {
  AddIdentity,
  ApplyIdentity,
  ClearIdentity,
  ForgetRepo,
  GetIdentityState,
  RecentRepos,
  RemoveIdentity,
  Signature,
} from '@wails/go/main/App'
import { errorText } from '@/utils/errors'

/** What focagit remembers between runs, plus the open repository's identity. */
interface PrefsState {
  recents: prefs.Recent[]
  /** The identities the user has saved, in the order they were added. */
  saved: prefs.Identity[]
  /** What this repository falls back to when it sets nothing of its own. */
  global: git.Signature | null
  /** Who git would attribute the next commit to, null before a repository is open. */
  signature: git.Signature | null
  error: string
}

const state = reactive<PrefsState>({
  recents: [],
  saved: [],
  global: null,
  signature: null,
  error: '',
})

async function loadRecents() {
  state.recents = await RecentRepos()
}

async function forgetRepo(path: string) {
  await ForgetRepo(path)
  await loadRecents()
}

/** Re-reads the identity git would use in the open repository. */
async function loadSignature(repoId: string | null) {
  state.signature = repoId ? await Signature(repoId) : null
}

/** Reads everything the identity dialog shows, in one call. */
async function loadIdentities(repoId: string) {
  const got = await GetIdentityState(repoId)
  state.signature = got.current
  state.global = got.global
  state.saved = got.saved
}

/** Runs one identity change, then re-reads what it changed. */
async function change(repoId: string, fn: () => Promise<void>): Promise<boolean> {
  state.error = ''
  try {
    await fn()
    await loadIdentities(repoId)
    return true
  } catch (e) {
    state.error = errorText(e)
    return false
  }
}

/** Saves an identity and uses it here straight away. */
const addIdentity = (repoId: string, ident: prefs.Identity) =>
  change(repoId, async () => {
    await AddIdentity(ident)
    await ApplyIdentity(repoId, ident)
  })

const removeIdentity = (repoId: string, email: string) =>
  change(repoId, () => RemoveIdentity(email))

const applyIdentity = (repoId: string, ident: prefs.Identity) =>
  change(repoId, () => ApplyIdentity(repoId, ident))

/** Drops the repository's own identity so the global one applies again. */
const useGlobalIdentity = (repoId: string) => change(repoId, () => ClearIdentity(repoId))

export function usePrefs() {
  return {
    state,
    loadRecents,
    loadIdentities,
    loadSignature,
    forgetRepo,
    addIdentity,
    removeIdentity,
    applyIdentity,
    useGlobalIdentity,
  }
}
