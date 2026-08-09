import { reactive } from 'vue'
import type { sshkeys } from '@wails/go/models'
import {
  AddSSHKeyToAgent,
  CopyToClipboard,
  GenerateSSHKey,
  OpenURL,
  SSHKeys,
} from '@wails/go/main/App'
import { errorText } from '@/utils/errors'

interface SSHState {
  keys: sshkeys.Key[]
  /** The key just generated, so the dialog can put its public half in front of you. */
  fresh: sshkeys.Key | null
  error: string
  busy: boolean
}

const state = reactive<SSHState>({ keys: [], fresh: null, error: '', busy: false })

async function loadKeys() {
  state.error = ''
  try {
    state.keys = await SSHKeys()
  } catch (e) {
    state.error = errorText(e)
  }
}

/** Creates a key, loads it into the agent, and leaves it in state.fresh. */
async function generate(name: string, comment: string): Promise<boolean> {
  state.error = ''
  state.busy = true
  try {
    state.fresh = await GenerateSSHKey(name, comment)
    await loadKeys()
    return true
  } catch (e) {
    state.error = errorText(e)
    return false
  } finally {
    state.busy = false
  }
}

async function addToAgent(path: string): Promise<boolean> {
  state.error = ''
  try {
    await AddSSHKeyToAgent(path)
    await loadKeys()
    return true
  } catch (e) {
    state.error = errorText(e)
    return false
  }
}

async function copy(text: string) {
  try {
    await CopyToClipboard(text)
  } catch (e) {
    state.error = errorText(e)
  }
}

export function useSSH() {
  return { state, loadKeys, generate, addToAgent, copy, openURL: OpenURL }
}
