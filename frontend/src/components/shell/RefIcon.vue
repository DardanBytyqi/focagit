<script setup lang="ts">
defineProps<{ kind: 'branch' | 'remote' | 'tag' }>()
</script>

<template>
  <svg class="ref-icon" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2">
    <template v-if="kind === 'branch'">
      <circle cx="3.5" cy="2.5" r="1.4" />
      <circle cx="3.5" cy="9.5" r="1.4" />
      <circle cx="8.5" cy="2.5" r="1.4" />
      <path d="M3.5 3.9v4.2M8.5 3.9v1.1a2 2 0 0 1-2 2H5.2" />
    </template>

    <template v-else-if="kind === 'remote'">
      <circle cx="6" cy="6" r="4.2" />
      <path d="M1.8 6h8.4" />
      <path d="M6 1.8c1.9 2.4 1.9 6 0 8.4-1.9-2.4-1.9-6 0-8.4z" />
    </template>

    <template v-else>
      <path
        d="M6.4 1.4H10a.6.6 0 0 1 .6.6v3.6L5.8 10.2a.8.8 0 0 1-1.2 0L1.8 7.4a.8.8 0 0 1 0-1.2z"
      />
      <circle cx="8.4" cy="3.6" r=".8" />
    </template>
  </svg>
</template>

<style scoped>
.ref-icon {
  width: 12px;
  height: 12px;
  flex: none;
  align-self: center;
  color: var(--text-dim);
}
</style>
