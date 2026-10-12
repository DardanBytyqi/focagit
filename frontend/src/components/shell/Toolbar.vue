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
  branch: []
  identity: []
  fetch: []
  pull: []
  push: []
  pushOptions: []
  stash: []
  stashOptions: []
  settings: [section: string]
}>()

// Nothing to put away means the button would open a window that can only refuse.
const dirty = computed(() => !!props.status?.files?.length)

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
    <button
      class="tool"
      :disabled="!info || busy"
      title="Bring down remote branches without touching yours"
      @click="emit('fetch')"
    >
      <svg class="icon" viewBox="0 0 16 16">
        <path d="M13 8a5 5 0 1 1-1.46-3.54" />
        <path d="M13.2 2.8v2.6h-2.6" />
      </svg>
      Fetch
    </button>

    <button
      class="tool"
      :disabled="!info || busy"
      title="Fetch and merge the upstream"
      @click="emit('pull')"
    >
      <svg class="icon" viewBox="0 0 16 16">
        <path d="M8 2.5v8M4.5 7L8 10.5 11.5 7M3 13.5h10" />
      </svg>
      Pull
      <span v-if="behind" class="pill behind">{{ behind }}</span>
    </button>

    <!-- Push does the obvious thing; the arrow is for when it is not obvious. -->
    <span class="split">
      <button
        class="tool split-main"
        :disabled="!info || busy"
        title="Push to the upstream"
        @click="emit('push')"
      >
        <svg class="icon" viewBox="0 0 16 16">
          <path d="M8 13.5v-8M4.5 9L8 5.5 11.5 9M3 2.5h10" />
        </svg>
        Push
        <span v-if="ahead" class="pill">{{ ahead }}</span>
      </button>
      <button
        class="split-more"
        :disabled="!info || busy"
        title="Push somewhere else, or overwrite"
        @click="emit('pushOptions')"
      >
        <svg class="caret" viewBox="0 0 10 10"><path d="M2.5 4l2.5 2.5L7.5 4" /></svg>
      </button>
    </span>

    <span class="sep"></span>

    <button
      class="tool"
      :disabled="!info"
      title="Create a branch from HEAD"
      @click="emit('branch')"
    >
      <svg class="icon" viewBox="0 0 16 16">
        <circle cx="4.5" cy="3.5" r="1.6" />
        <circle cx="4.5" cy="12.5" r="1.6" />
        <circle cx="11.5" cy="3.5" r="1.6" />
        <path d="M4.5 5.1v5.8M11.5 5.1v1.4a2.5 2.5 0 0 1-2.5 2.5H6.4" />
      </svg>
      Branch
    </button>

    <!-- Stash puts everything away; the arrow is for a message and the options. -->
    <span class="split">
      <button
        class="tool split-main"
        :disabled="!info || !dirty"
        title="Put the working tree away and come back to it later"
        @click="emit('stash')"
      >
        <svg class="icon" viewBox="0 0 16 16">
          <path d="M2.5 3h11v3h-11zM3.5 6v6.5h9V6M6.5 8.75h3" />
        </svg>
        Stash
      </button>
      <button
        class="split-more"
        :disabled="!info || !dirty"
        title="Stash with a message, or only part of it"
        @click="emit('stashOptions')"
      >
        <svg class="caret" viewBox="0 0 10 10"><path d="M2.5 4l2.5 2.5L7.5 4" /></svg>
      </button>
    </span>

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

    <div v-if="status" class="branch" :title="branchTitle">
      <svg class="icon" viewBox="0 0 16 16">
        <circle cx="4.5" cy="3.5" r="1.6" />
        <circle cx="4.5" cy="12.5" r="1.6" />
        <circle cx="11.5" cy="3.5" r="1.6" />
        <path d="M4.5 5.1v5.8M11.5 5.1v1.4a2.5 2.5 0 0 1-2.5 2.5H6.4" />
      </svg>
      <span class="branch-name">{{ status.detached ? 'detached HEAD' : status.head }}</span>
      <span v-if="status.upstream" class="tracking">{{ upstreamLabel }}</span>
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
  gap: 2px;
  height: var(--toolbar-height);
  padding: 0 10px;
  background: var(--bg);
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

.icon {
  width: 16px;
  height: 16px;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.tool {
  display: flex;
  align-items: center;
  gap: 7px;
  height: 32px;
  padding: 0 11px;
  color: var(--text-muted);
  border-radius: var(--radius);
}

.tool:hover:not(:disabled) {
  color: var(--text);
}

.tool:disabled .icon {
  opacity: 0.6;
}

.split {
  display: flex;
}

.split-main {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
  padding-right: 7px;
}

.split-more {
  display: grid;
  place-items: center;
  width: 22px;
  height: 32px;
  padding: 0;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  color: var(--text-dim);
}

.caret {
  width: 10px;
  height: 10px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.4;
  stroke-linecap: round;
  stroke-linejoin: round;
}

/* One surface under the pair while either half is under the pointer. */
.split:hover .split-main:not(:disabled),
.split:hover .split-more:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text);
}

.split .split-more:hover:not(:disabled) {
  background: var(--border-strong);
}

.settings {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  padding: 0;
  margin-left: 4px;
  color: var(--text-muted);
  background: none;
  border: 1px solid transparent;
  border-radius: var(--radius);
}

.settings svg {
  width: 16px;
  height: 16px;
}

.settings:hover,
.settings[data-state='open'] {
  color: var(--text);
  background: var(--bg-hover);
}

.sep {
  width: 1px;
  height: 20px;
  margin: 0 8px;
  background: var(--border);
}

.spacer {
  flex: 1;
}

.pill {
  margin-left: 2px;
}

.pill.behind {
  color: var(--amber);
}

.identity {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  max-width: 280px;
  flex: 0 1 auto;
  min-width: 0;
  padding: 0 10px;
  font-size: var(--fs-small);
  color: var(--text-muted);
  border-radius: var(--radius);
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

/* Where HEAD is, as a chip: the one piece of state every action depends on. */
.branch {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 1 auto;
  min-width: 0;
  height: 30px;
  padding: 0 4px 0 6px;
}

.branch .icon {
  width: 14px;
  height: 14px;
  color: var(--green);
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
  font-size: var(--fs-small);
}

.tracking {
  flex: 0 1 auto;
  font-size: var(--fs-small);
  color: var(--text-dim);
}

.ahead,
.behind {
  flex: none;
  font-size: var(--fs-small);
  font-weight: 600;
}

.ahead {
  color: var(--green);
}

.behind {
  color: var(--amber);
}
</style>
