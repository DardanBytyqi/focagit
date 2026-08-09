<script setup lang="ts">
import { computed } from 'vue'
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
  push: []
  pushOptions: []
}>()

// An inherited identity is the one that commits under the wrong address.
const inherited = computed(() => props.signature?.emailScope !== 'local')

const who = computed(() => {
  const sig = props.signature
  if (!sig?.email) return 'no identity'
  return sig.email
})

// How many commits are waiting to go out, and to come in.
const ahead = computed(() => props.status?.ahead ?? 0)
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

    <button disabled title="Pull is next">
      Pull
      <span v-if="behind" class="badge behind-badge">{{ behind }}</span>
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
        <span v-if="ahead" class="badge">{{ ahead }}</span>
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

.sep {
  width: 1px;
  height: 20px;
  margin: 0 6px;
  background: var(--border);
}

.spacer {
  flex: 1;
}

/* How many commits are waiting, on the button that would move them. */
.badge {
  margin-left: 5px;
  padding: 0 5px;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
}

/* Amber like everywhere else, and legible on a disabled button. */
.badge.behind-badge {
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
