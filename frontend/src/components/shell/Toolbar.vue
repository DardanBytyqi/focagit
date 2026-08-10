<script setup lang="ts">
import { computed } from 'vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import type { git, session } from '@wails/go/models'

const props = defineProps<{
  info: session.Info | null
  status: git.Status | null
  signature: git.Signature | null
  /** True while a fetch or push is running, so it cannot be started twice. */
  busy?: boolean
}>()

const emit = defineEmits<{
  browse: []
  refresh: []
  branch: []
  identity: []
  fetch: []
  pull: []
  push: []
  pushOptions: []
  settings: [section: string]
}>()

// An inherited identity is the one that commits under the wrong address.
const inherited = computed(() => props.signature?.emailScope !== 'local')

const who = computed(() => {
  const sig = props.signature
  if (!sig?.email) return 'no identity'
  return sig.email
})

/** Commits waiting to go out; with no upstream, those on no remote. */
const ahead = computed(() => props.status?.ahead || props.status?.unpushed || 0)
const behind = computed(() => props.status?.behind ?? 0)

// An upstream is nearly always "origin/" plus the branch you are already reading
const upstreamLabel = computed(() => {
  const st = props.status
  if (!st?.upstream) return ''
  const suffix = '/' + st.head
  return st.upstream.endsWith(suffix) ? st.upstream.slice(0, -suffix.length) : st.upstream
})

/** The full names, for when the shortened ones are not enough. */
const branchTitle = computed(() => {
  const st = props.status
  if (!st) return ''
  const head = st.detached ? 'detached HEAD' : st.head
  return st.upstream ? `${head} → ${st.upstream}` : head
})
</script>

<template>
  <div class="toolbar">
    <button @click="emit('browse')">Open…</button>
    <button :disabled="!info" @click="emit('refresh')">Refresh</button>

    <span class="sep"></span>

    <button
      :disabled="!info || busy"
      title="Bring down remote branches without touching yours"
      @click="emit('fetch')"
    >
      Fetch
    </button>

    <button
      :disabled="!info || busy"
      title="Fetch and merge the upstream"
      @click="emit('pull')"
    >
      Pull
      <span v-if="behind" class="pill behind">{{ behind }}</span>
    </button>

    <!-- Push does the obvious thing; the arrow is for when it is not obvious. -->
    <span class="split">
      <button
        class="split-main"
        :disabled="!info || busy"
        title="Push to the upstream"
        @click="emit('push')"
      >
        Push
        <span v-if="ahead" class="pill">{{ ahead }}</span>
      </button>
      <button
        class="split-more"
        :disabled="!info || busy"
        title="Push somewhere else, or overwrite"
        @click="emit('pushOptions')"
      >
        ▾
      </button>
    </span>

    <span class="sep"></span>

    <button :disabled="!info" title="Create a branch from HEAD" @click="emit('branch')">
      Branch…
    </button>

    <span class="spacer"></span>

    <button
      v-if="info"
      class="identity"
      :class="{ inherited, unset: !signature?.email }"
      :title="
        signature?.email
          ? `${signature.name} <${signature.email}> — ${inherited ? 'inherited from your global config' : 'set for this repository'}`
          : 'No identity: git will refuse to commit'
      "
      @click="emit('identity')"
    >
      <span class="dot"></span>
      {{ who }}
    </button>

    <span v-if="info" class="sep"></span>

    <div v-if="status" class="branch" :title="branchTitle">
      <span class="branch-name">{{ status.detached ? 'detached HEAD' : status.head }}</span>
      <span v-if="status.upstream" class="tracking muted mono">{{ upstreamLabel }}</span>
      <span v-if="status.ahead" class="ahead">↑{{ status.ahead }}</span>
      <span v-if="status.behind" class="behind">↓{{ status.behind }}</span>
    </div>

    <span v-if="info" class="sep"></span>

    <!-- The machine's settings, past everything the repository owns. -->
    <DropdownMenuRoot>
      <DropdownMenuTrigger class="settings" title="Settings" aria-label="Settings">
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3">
          <circle cx="8" cy="8" r="4.6" />
          <circle cx="8" cy="8" r="1.7" />
          <g stroke-linecap="round">
            <path d="M12.6 8h1.6M11.25 11.25l1.13 1.13M8 12.6v1.6M4.75 11.25l-1.13 1.13" />
            <path d="M3.4 8H1.8M4.75 4.75L3.62 3.62M8 3.4V1.8M11.25 4.75l1.13-1.13" />
          </g>
        </svg>
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent class="menu" align="end" :side-offset="4">
          <DropdownMenuItem class="menu-item" @select="emit('settings', 'appearance')">
            Appearance…
          </DropdownMenuItem>
          <DropdownMenuItem class="menu-item" @select="emit('settings', 'ssh')">
            SSH keys…
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  height: var(--toolbar-height);
  padding: 0 10px;
  background: var(--bg-toolbar);
  border-bottom: 1px solid var(--border);
  flex: none;
}

/* Buttons and separators keep their size; only the names shrink. */
.toolbar > button,
.split,
.settings,
.sep {
  flex: none;
}

.split {
  display: flex;
}

.split-main {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
}

.split-more {
  padding: 4px 5px;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  font-size: 10px;
  color: var(--text-muted);
}

/* One outline around the pair while either half is under the pointer. */
.split:hover .split-main:not(:disabled),
.split:hover .split-more:not(:disabled) {
  background: var(--bg-hover);
  border-color: var(--border);
}

.settings {
  display: flex;
  padding: 4px;
  color: var(--text-muted);
  background: none;
  border: 1px solid transparent;
  border-radius: 4px;
}

.settings svg {
  width: 15px;
  height: 15px;
}

.settings:hover,
.settings[data-state='open'] {
  color: var(--text);
  background: var(--bg-hover);
  border-color: var(--border);
}

.sep {
  width: 1px;
  height: 20px;
  margin: 0 6px;
  background: var(--border);
}

.spacer {
  flex: 1;
}

.pill {
  margin-left: 6px;
}

/* Amber like everywhere else, and legible on a disabled button. */
.pill.behind {
  background: var(--amber);
  color: #1e1f22;
}

.identity {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 260px;
  flex: 0 1 auto;
  min-width: 0;
  font-size: var(--fs-small);
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.identity .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--green);
  flex: none;
}

.identity.inherited .dot {
  background: var(--amber);
}

.identity.unset .dot {
  background: var(--red);
}


.branch {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex: 0 1 auto;
  min-width: 0;
}

.branch-name,
.tracking {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.branch-name {
  flex: 1 1 auto;
  font-weight: 600;
  color: var(--green);
}

.tracking {
  flex: 0 1 auto;
  font-size: var(--fs-small);
}

.ahead,
.behind {
  flex: none;
}

.ahead {
  color: var(--green);
}

.behind {
  color: var(--amber);
}
</style>
