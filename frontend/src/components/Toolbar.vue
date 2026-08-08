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
}>()

// An identity inherited from the global config is the one that commits under the
// wrong address, so it is worth showing rather than hiding.
const inherited = computed(() => props.signature?.emailScope !== 'local')

const who = computed(() => {
  const sig = props.signature
  if (!sig?.email) return 'no identity'
  return sig.email
})

// Still to come in stage 6.
const pending = ['Pull', 'Merge', 'Stash']

// A branch nobody has pushed yet has nothing to compare against, so "up to date"
// would be a lie; the push dialog is still the way to publish it.
const ahead = computed(() => props.status?.ahead ?? 0)

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
      title="Send your commits to a remote"
      @click="emit('push')"
    >
      Push
      <span v-if="ahead" class="badge">{{ ahead }}</span>
    </button>

    <span class="sep"></span>

    <button :disabled="!info" title="Create a branch from HEAD" @click="emit('branch')">
      Branch…
    </button>

    <button v-for="b in pending" :key="b" disabled :title="`${b} arrives in stage 6`">
      {{ b }}
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
.sep {
  flex: none;
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

/* How many commits are waiting to go out, on the button that sends them. */
.badge {
  margin-left: 5px;
  padding: 0 5px;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
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
