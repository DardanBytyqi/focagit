import { computed, reactive, ref } from 'vue'
import type { BranchRequest, DeleteRequest, MergeDrop } from '@/types'

/**
 * Which windows are open. Kept in one place because they are not independent:
 * only one belongs on screen at a time, and the status bar needs to know that
 * anything is open at all.
 */
const open = reactive({
  clone: false,
  identity: false,
  push: false,
  remote: false,
  settings: false,
  conflicts: false,
})

/** Which pane the settings window lands on. */
const settingsSection = ref('appearance')

/** The ones that carry a subject rather than just a flag. */
const branchRequest = ref<BranchRequest | null>(null)
const deleteRequest = ref<DeleteRequest | null>(null)
const mergeDrop = ref<MergeDrop | null>(null)

/** An error shown inside a window does not need repeating in the status bar. */
const anyOpen = computed(() => Object.values(open).some(Boolean))

function showSettings(section: string) {
  settingsSection.value = section
  open.settings = true
}

/** The push window steps aside rather than stacking a second one on top. */
function showSSH() {
  open.push = false
  showSettings('ssh')
}

function newBranch(start: string, startLabel: string) {
  branchRequest.value = { kind: 'create', start, startLabel }
}

function renameBranch(from: string) {
  branchRequest.value = { kind: 'rename', from }
}

export function useDialogs() {
  return {
    open,
    settingsSection,
    branchRequest,
    deleteRequest,
    mergeDrop,
    anyOpen,
    showSettings,
    showSSH,
    newBranch,
    renameBranch,
  }
}
