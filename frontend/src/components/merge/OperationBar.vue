<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRepo } from '@/composables/useRepo'

const emit = defineEmits<{ resolve: [] }>()

const { state, abortOperation, finishOperation } = useRepo()

const busy = ref(false)

const op = computed(() => state.status?.operation ?? '')
const left = computed(() => state.conflicts.length)
const into = computed(() => state.status?.head ?? '')
const from = computed(() => state.status?.operationFrom ?? '')

const titles: Record<string, string> = {
  merge: 'Merging',
  revert: 'Reverting',
  'cherry-pick': 'Cherry-picking',
}
const title = computed(() => titles[op.value] ?? 'In progress')

async function finish() {
  busy.value = true
  await finishOperation('')
  busy.value = false
}

async function abort() {
  busy.value = true
  await abortOperation()
  busy.value = false
}
</script>

<template>
  <div class="operation-bar">
    <span class="title">{{ title }}</span>
    <span class="what">
      <b class="theirs">{{ from }}</b>
      <template v-if="op === 'merge'">
        into <b class="mine">{{ into }}</b></template
      >
    </span>

    <span class="count" :class="{ done: !left }">
      {{ left ? `${left} file${left > 1 ? 's' : ''} to resolve` : 'all resolved' }}
    </span>

    <span class="spacer"></span>

    <button type="button" :disabled="busy" @click="abort">Abort</button>
    <button v-if="left" type="button" class="primary" @click="emit('resolve')">
      Resolve conflicts
    </button>
    <button v-else type="button" class="primary" :disabled="busy" @click="finish">
      {{ op === 'merge' ? 'Commit merge' : 'Continue' }}
    </button>
  </div>
</template>

<style scoped>
.operation-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: none;
  padding: 6px 10px;
  background: var(--bg-raised);
  border-bottom: 1px solid var(--amber);
}

.title {
  color: var(--amber);
  font-weight: 600;
}

.what {
  font-size: var(--fs-small);
  color: var(--text-muted);
}

/* The same two colours the resolver uses, so the pairing is learnt here. */
.what .mine {
  color: var(--mine);
}

.what .theirs {
  color: var(--theirs);
}

.count {
  font-size: var(--fs-small);
  color: var(--text-muted);
}

.count.done {
  color: var(--green);
}

.spacer {
  flex: 1;
}
</style>
