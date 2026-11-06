import type { git } from '@wails/go/models'
import type { TreeNode } from '@/types'

export type { TreeNode }

/** A flat list of paths becomes a directory tree. */

export function buildTree(files: git.FileStatus[]): TreeNode[] {
  const root: TreeNode = { name: '', path: '', dir: true, children: [] }

  for (const file of files) {
    const parts = file.path.split('/')
    let node = root

    parts.forEach((name, i) => {
      const leaf = i === parts.length - 1
      const path = parts.slice(0, i + 1).join('/')

      let next = node.children.find((c) => c.name === name && c.dir === !leaf)
      if (!next) {
        next = { name, path, dir: !leaf, children: [] }
        if (leaf) next.file = file
        node.children.push(next)
      }
      node = next
    })
  }

  sort(root)
  return root.children.map(compact)
}

/** A directory holding only one directory folds into it, so a deep path takes one row. */
function compact(node: TreeNode): TreeNode {
  while (node.dir && node.children.length === 1 && node.children[0].dir) {
    const only = node.children[0]
    node = { ...only, name: `${node.name}/${only.name}` }
  }
  if (node.dir) node.children = node.children.map(compact)
  return node
}

/** Directories before files, then alphabetical — the convention everywhere. */
function sort(node: TreeNode) {
  node.children.sort((a, b) => {
    if (a.dir !== b.dir) return a.dir ? -1 : 1
    return a.name.localeCompare(b.name)
  })
  node.children.forEach(sort)
}

/** Every directory path in a tree, for expanding it all by default. */
export function allDirs(nodes: TreeNode[], out: string[] = []): string[] {
  for (const n of nodes) {
    if (n.dir) {
      out.push(n.path)
      allDirs(n.children, out)
    }
  }
  return out
}
