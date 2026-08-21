/** Types that exist only in the frontend. */
import type { git } from '@wails/go/models'

/** One node of the staging file tree, built from a flat list of paths. */
export interface TreeNode {
  name: string
  /** Full path: the file's path for leaves, the directory's for branches. */
  path: string
  dir: boolean
  children: TreeNode[]
  file?: git.FileStatus
}

// What is selected in the Changes view, and in which pane.
export interface Selected {
  path: string
  staged: boolean
  untracked: boolean
  dir: boolean
}

/** An open branch dialog: either creating one, or renaming an existing one. */
export type BranchRequest =
  | { kind: 'create'; start: string; startLabel: string }
  | { kind: 'rename'; from: string }

/** An open confirmation for deleting a branch. */
export interface DeleteRequest {
  name: string
}

/** An open confirmation for replaying a commit, one way round or the other. */
export interface ApplyRequest {
  kind: 'revert' | 'cherry-pick'
  commit: git.Commit
}

/** An open confirmation for moving the current branch onto a commit. */
export interface ResetRequest {
  commit: git.Commit
  /** The branch that would move; empty when HEAD is detached. */
  branch: string
}

/** A branch dropped on another branch, and where the pointer let go. */
export interface MergeDrop {
  source: git.Ref
  target: git.Ref
  x: number
  y: number
}
