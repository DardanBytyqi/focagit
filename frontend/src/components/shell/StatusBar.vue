<script setup lang="ts">
import { computed } from 'vue'
import type { git, session } from '@wails/go/models'

const props = defineProps<{
  info: session.Info | null
  status: git.Status | null
  count: number
  loading: boolean
  error: string
  /** git's own progress line while a fetch or push runs; empty when idle. */
  network: string
}>()

const changes = computed(() => props.status?.files?.length ?? 0)
</script>

<template>
  <div class="statusbar">
    <template v-if="info">
      <span class="path">{{ info.path }}</span>
      <span>{{ count.toLocaleString() }} commits</span>
      <span v-if="loading" class="loading">loading…</span>
      <span :class="{ dirty: changes > 0 }">
        {{ changes === 0 ? 'clean' : `${changes} change${changes === 1 ? '' : 's'}` }}
      </span>
    </template>
    <span v-else class="muted">no repository open</span>

    <span class="spacer"></span>
    <span v-if="network" class="network mono">{{ network }}</span>
    <span v-else-if="error" class="error" :title="error">{{ error }}</span>
  </div>
</template>

<style scoped>
.statusbar {
  display: flex;
  align-items: center;
  gap: 18px;
  height: var(--statusbar-height);
  padding: 0 12px;
  background: var(--bg-shell);
  border-top: 1px solid var(--border);
  font-size: calc(var(--fs-small) - 1px);
  color: var(--text-dim);
  flex: none;
}

.path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 40%;
}

.loading {
  color: var(--blue);
}

.dirty {
  color: var(--amber);
}

.spacer {
  flex: 1;
}

.error {
  color: var(--red);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 50%;
}

/* Progress outranks the last error: it is what is happening now. */
.network {
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 50%;
}
</style>
