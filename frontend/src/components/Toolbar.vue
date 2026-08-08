<script setup lang="ts">
import { computed } from 'vue'
import type { git, session } from '@wails/go/models'

const props = defineProps<{
  info: session.Info | null
  status: git.Status | null
  signature: git.Signature | null
}>()

const emit = defineEmits<{ browse: []; refresh: []; branch: []; identity: [] }>()

// An identity inherited from the global config is the one that commits under the
// wrong address, so it is worth showing rather than hiding.
const inherited = computed(() => props.signature?.emailScope !== 'local')

const who = computed(() => {
  const sig = props.signature
  if (!sig?.email) return 'no identity'
  return sig.email
})

// Still to come in stage 6.
const pending = ['Pull', 'Push', 'Merge', 'Stash']
</script>

<template>
  <div class="toolbar">
    <button @click="emit('browse')">Open…</button>
    <button :disabled="!info" @click="emit('refresh')">Refresh</button>

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

    <div v-if="status" class="branch">
      <span class="branch-name">{{ status.detached ? 'detached HEAD' : status.head }}</span>
      <span v-if="status.upstream" class="tracking muted mono">
        {{ status.upstream }}
        <span v-if="status.ahead" class="ahead">↑{{ status.ahead }}</span>
        <span v-if="status.behind" class="behind">↓{{ status.behind }}</span>
      </span>
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

.sep {
  width: 1px;
  height: 20px;
  margin: 0 6px;
  background: var(--border);
}

.spacer {
  flex: 1;
}

.identity {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 260px;
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

/* Amber, not red: inheriting the global identity is normal, it is just the
   thing you want to notice before committing under the wrong address. */
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
}

.branch-name {
  font-weight: 600;
  color: var(--green);
}

.tracking {
  font-size: var(--fs-small);
}

.ahead {
  color: var(--green);
}

.behind {
  color: var(--amber);
}
</style>
