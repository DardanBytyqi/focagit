<script setup lang="ts">
import { ref } from 'vue'
import type { git } from '@wails/go/models'
import Splitter from '@/components/common/Splitter.vue'
import CommitDetail from '@/components/history/CommitDetail.vue'
import CommitList from '@/components/history/CommitList.vue'

defineProps<{
  repoId: string
  count: number
  width: number
  gen: number
  refsByHash: Map<string, git.Ref[]>
  head: string
}>()

const emit = defineEmits<{
  branch: [start: string, label: string]
  checkout: [hash: string]
  revert: [commit: git.Commit]
  cherryPick: [commit: git.Commit]
  reset: [commit: git.Commit]
}>()

/** Which commit the lower pane is showing; nothing else needs to know. */
const selected = ref<git.Commit | null>(null)

const detailHeight = ref(340)
</script>

<template>
  <div class="history">
    <CommitList
      :repo-id="repoId"
      :count="count"
      :width="width"
      :gen="gen"
      :refs-by-hash="refsByHash"
      :head="head"
      @select="selected = $event"
      @branch="(start, label) => emit('branch', start, label)"
      @checkout="emit('checkout', $event)"
      @revert="emit('revert', $event)"
      @cherry-pick="emit('cherryPick', $event)"
      @reset="emit('reset', $event)"
    />
  </div>

  <Splitter v-model="detailHeight" />

  <section class="detail" :style="{ height: detailHeight + 'px' }">
    <CommitDetail :repo-id="repoId" :commit="selected" />
  </section>
</template>

<style scoped>
.history {
  flex: 1;
  min-height: 0;
}

.detail {
  flex: none;
  min-height: 0;
  overflow: hidden;
}
</style>
