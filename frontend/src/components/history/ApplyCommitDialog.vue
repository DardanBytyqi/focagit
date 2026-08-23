<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Modal from '@/components/common/Modal.vue'
import Select from '@/components/common/Select.vue'
import { useRepo } from '@/composables/useRepo'
import type { ApplyRequest } from '@/types'

const props = defineProps<{ request: ApplyRequest | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, revert, cherryPick } = useRepo()

const mainline = ref('1')
const busy = ref(false)

const revertKind = computed(() => props.request?.kind === 'revert')
const verb = computed(() => (revertKind.value ? 'Revert' : 'Cherry-pick'))

const parents = computed(() => props.request?.commit.parents ?? [])
// A merge has two sides, and git will not guess which one the change is against.
const isMerge = computed(() => parents.value.length > 1)

const sides = computed(() =>
  parents.value.map((hash, i) => ({
    value: String(i + 1),
    label: i === 0 ? `Side ${i + 1}, the branch` : `Side ${i + 1}, merged in`,
    hint: hash.slice(0, 7),
  })),
)

watch(
  () => props.request,
  (r) => {
    if (!r) return
    mainline.value = '1'
    state.error = ''
  },
  { immediate: true },
)

const open = computed({
  get: () => !!props.request,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function confirm() {
  const r = props.request
  if (!r || busy.value) return

  const opts = {
    rev: r.commit.hash,
    noCommit: false,
    mainline: isMerge.value ? Number(mainline.value) : 0,
  }
  busy.value = true
  const res = revertKind.value ? await revert(opts) : await cherryPick(opts)
  busy.value = false

  // A conflict is an outcome, not a refusal: the bar takes over from here.
  if (res) emit('close')
}
</script>

<template>
  <Modal v-model:open="open" :title="`${verb} commit`">
    <div class="stack">
      <p class="commit">
        <span class="mono">{{ request?.commit.hash.slice(0, 7) }}</span>
        {{ request?.commit.subject }}
      </p>

      <div v-if="isMerge" class="field">
        <label>Apply against</label>
        <Select v-model="mainline" :options="sides" />
      </div>

      <p v-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button type="button" class="primary" :disabled="busy" @click="confirm">{{ verb }}</button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.commit {
  margin: 0;
  overflow-wrap: anywhere;
}

.commit .mono {
  margin-right: 6px;
  color: var(--text-muted);
}

/* The stack already spaces these, so their own margin would double it. */
.stack > .error {
  margin-top: 0;
}
</style>
