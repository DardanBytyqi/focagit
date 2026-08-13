<script setup lang="ts">
import { computed } from 'vue'
import { git } from '@wails/go/models'
import CommitDetail from '@/components/history/CommitDetail.vue'

const props = defineProps<{ repoId: string; stash: git.Stash }>()
const emit = defineEmits<{ pop: []; apply: []; drop: [] }>()

/** A stash is a commit, so the detail pane can show it with no special case. */
const commit = computed(() =>
  git.Commit.createFrom({
    hash: props.stash.hash,
    parents: [],
    author: props.stash.author,
    committer: props.stash.author,
    subject: props.stash.message,
    body: '',
  }),
)
</script>

<template>
  <div class="stash-view">
    <!-- The same actions as the sidebar's context menu, for whoever came here first. -->
    <div class="bar">
      <span class="muted">Stash</span>
      <span class="spacer"></span>
      <button @click="emit('pop')">Pop</button>
      <button @click="emit('apply')">Apply and keep</button>
      <button class="drop" @click="emit('drop')">Drop…</button>
    </div>

    <CommitDetail :repo-id="repoId" :commit="commit" />
  </div>
</template>

<style scoped>
.stash-view {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  height: var(--toolbar-height);
  background: var(--bg-toolbar);
  border-bottom: 1px solid var(--border);
  flex: none;
}

.spacer {
  flex: 1;
}

.drop {
  color: var(--red);
}

.drop:hover:not(:disabled) {
  background: var(--red);
  color: var(--bg);
  border-color: var(--red);
}
</style>
