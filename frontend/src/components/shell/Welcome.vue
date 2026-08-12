<script setup lang="ts">
import RecentList from '@/components/shell/RecentList.vue'
import { useDialogs } from '@/composables/useDialogs'
import { useRepo } from '@/composables/useRepo'

const { state, open, browse, initHere } = useRepo()
const dialogs = useDialogs()
</script>

<template>
  <div class="welcome">
    <h1>focagit</h1>

    <!-- A folder that is not a repository is an offer, not a failure. -->
    <template v-if="state.initCandidate">
      <p class="muted">
        <span class="mono">{{ state.initCandidate }}</span> is not a repository yet.
      </p>
      <div class="row">
        <button class="primary" @click="initHere">Initialise repository here</button>
        <button @click="browse">Choose another folder…</button>
      </div>
    </template>

    <template v-else>
      <p class="muted">Open a repository to get started.</p>
      <div class="row">
        <button class="primary" @click="browse">Open repository…</button>
        <button @click="dialogs.open.clone = true">Clone repository…</button>
      </div>
      <RecentList @open="open" />
    </template>

    <p v-if="state.error" class="error">{{ state.error }}</p>
  </div>
</template>

<style scoped>
.welcome {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.row {
  display: flex;
  gap: 8px;
}

.mono {
  font-family: var(--font-mono);
  color: var(--text);
}

h1 {
  margin: 0;
  font-size: 30px;
  font-weight: 300;
  letter-spacing: 0.06em;
}

.primary {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
  padding: 7px 16px;
}

.primary:hover {
  filter: brightness(1.1);
  background: var(--accent);
  border-color: var(--accent);
}

.error {
  color: var(--red);
  max-width: 60ch;
  text-align: center;
}
</style>
