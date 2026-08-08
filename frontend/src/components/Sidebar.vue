<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from 'reka-ui'
import type { git } from '@wails/go/models'

const props = defineProps<{
  refs: git.Ref[]
  changeCount: number
  mode: 'commit' | 'changes'
}>()

const emit = defineEmits<{
  mode: [m: 'commit' | 'changes']
  checkout: [r: git.Ref]
  create: [start: string, label: string]
  rename: [name: string]
  remove: [name: string]
}>()

// Only branches can be checked out directly; a tag would detach HEAD.
function canCheckout(r: git.Ref) {
  return r.kind === 'branch' || r.kind === 'remote'
}

/** Sections mirror Fork's sidebar. */
const sections = computed(() => [
  { key: 'branch', title: 'Local', refs: props.refs.filter((r) => r.kind === 'branch') },
  { key: 'remote', title: 'Remotes', refs: props.refs.filter((r) => r.kind === 'remote') },
  { key: 'tag', title: 'Tags', refs: props.refs.filter((r) => r.kind === 'tag') },
])

const collapsed = ref<Record<string, boolean>>({})

function toggle(key: string) {
  collapsed.value[key] = !collapsed.value[key]
}
</script>

<template>
  <aside class="sidebar">
    <!-- The working tree and the history are two views of the repository, not
         a row inside one of them. Fork treats them as peers here. -->
    <nav class="nav">
      <div
        class="nav-item"
        :class="{ active: mode === 'changes' }"
        @click="emit('mode', 'changes')"
      >
        <span class="nav-icon">◧</span>
        <span>Changes</span>
        <span v-if="changeCount" class="badge">{{ changeCount }}</span>
      </div>
      <div class="nav-item" :class="{ active: mode === 'commit' }" @click="emit('mode', 'commit')">
        <span class="nav-icon">≡</span>
        <span>All Commits</span>
      </div>
    </nav>

    <div v-for="s in sections" :key="s.key" class="section">
      <div class="section-header" @click="toggle(s.key)">
        <span class="chevron" :class="{ closed: collapsed[s.key] }">▾</span>
        {{ s.title }}
        <span class="count muted">{{ s.refs.length }}</span>
      </div>

      <div v-show="!collapsed[s.key]">
        <ContextMenuRoot v-for="r in s.refs" :key="r.name">
          <ContextMenuTrigger as-child>
            <div
              class="ref"
              :class="{ head: r.head }"
              @dblclick="canCheckout(r) && emit('checkout', r)"
            >
              <span class="ref-name">{{ r.short }}</span>
              <span v-if="r.upstream" class="upstream muted mono">{{ r.upstream }}</span>
            </div>
          </ContextMenuTrigger>

          <ContextMenuPortal>
            <ContextMenuContent class="menu">
              <ContextMenuItem
                v-if="canCheckout(r)"
                class="menu-item"
                :disabled="r.head"
                @select="emit('checkout', r)"
              >
                Checkout {{ r.short }}
              </ContextMenuItem>

              <ContextMenuItem class="menu-item" @select="emit('create', r.short, r.short)">
                New branch from {{ r.short }}…
              </ContextMenuItem>

              <template v-if="r.kind === 'branch'">
                <ContextMenuSeparator class="menu-sep" />
                <ContextMenuItem class="menu-item" @select="emit('rename', r.short)">
                  Rename…
                </ContextMenuItem>
                <ContextMenuItem
                  class="menu-item danger"
                  :disabled="r.head"
                  @select="emit('remove', r.short)"
                >
                  Delete…
                </ContextMenuItem>
              </template>
            </ContextMenuContent>
          </ContextMenuPortal>
        </ContextMenuRoot>

        <div v-if="!s.refs.length" class="empty muted">none</div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.nav {
  padding: 4px 0 8px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 7px;
  height: 24px;
  padding: 0 10px;
  cursor: default;
}

.nav-item:hover {
  background: var(--bg-hover);
}

.nav-item.active {
  background: var(--bg-selected);
}

.nav-icon {
  width: 12px;
  text-align: center;
  color: var(--text-muted);
}

.badge {
  margin-left: auto;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--amber);
  color: #1e1f22;
  font-size: 11px;
  font-weight: 700;
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-raised);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 6px 0;
  flex: none;
}

.section {
  margin-bottom: 10px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  font-size: var(--fs-small);
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  cursor: default;
}

.chevron {
  display: inline-block;
  transition: transform 0.12s ease;
  font-size: 10px;
}

.chevron.closed {
  transform: rotate(-90deg);
}

.count {
  margin-left: auto;
  font-weight: 400;
}

.ref {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 2px 10px 2px 26px;
  height: 22px;
  cursor: default;
  overflow: hidden;
  white-space: nowrap;
}

.ref:hover {
  background: var(--bg-hover);
}

.ref.head .ref-name {
  font-weight: 600;
  color: var(--green);
}

/* The branch name always wins the available space. A long upstream name would
   otherwise push it out of the row entirely, which is worse than truncating
   the upstream: the name is the thing being identified. */
.ref-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.upstream {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 45%;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11px;
}

.empty {
  padding: 2px 10px 2px 26px;
  font-size: var(--fs-small);
  font-style: italic;
}
</style>
