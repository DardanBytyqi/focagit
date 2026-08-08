<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Check from '@/components/Check.vue'
import Modal from '@/components/Modal.vue'
import { useRepo } from '@/composables/useRepo'
import { isRejected, isStaleInfo } from '@/utils/errors'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { state, loadRemotes, push } = useRepo()

const remote = ref('')
const setUpstream = ref(true)
const force = ref(false)
const busy = ref(false)
/** What the remote said about each ref, shown instead of closing on success. */
const done = ref<string[]>([])

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

/** The branch being pushed, and where it already goes if it has been pushed before. */
const branch = computed(() => (state.status?.detached ? '' : (state.status?.head ?? '')))
const upstream = computed(() => state.status?.upstream ?? '')

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    state.error = ''
    done.value = []
    force.value = false
    await loadRemotes()
    remote.value = state.remotes[0]?.name ?? ''
    setUpstream.value = !upstream.value
  },
)

async function submit() {
  if (!branch.value || busy.value) return
  busy.value = true
  const res = await push({
    remote: remote.value,
    branch: branch.value,
    setUpstream: setUpstream.value,
    force: force.value,
    tags: false,
  })
  busy.value = false
  if (res) done.value = res.map((r) => `${r.ref} ${r.summary}`)
}
</script>

<template>
  <Modal v-model:open="isOpen" title="Push">
    <template v-if="state.status?.detached">
      <p class="hint">HEAD is detached, so there is no branch to push. Create one first.</p>
    </template>

    <template v-else>
      <p class="lead">
        Pushing <b>{{ branch }}</b>
        <template v-if="upstream">
          to <span class="mono">{{ upstream }}</span>
        </template>
      </p>

      <label for="push-remote">Remote</label>
      <select id="push-remote" v-model="remote" class="remote">
        <option v-for="r in state.remotes" :key="r.name" :value="r.name">
          {{ r.name }} — {{ r.push || r.fetch }}
        </option>
      </select>

      <p v-if="!state.remotes.length" class="hint warn">
        This repository has no remote yet. Add one with
        <span class="mono">git remote add origin &lt;url&gt;</span>.
      </p>

      <Check v-model="setUpstream" :disabled="!!upstream && !force">
        Track this remote branch from now on
      </Check>

      <Check v-model="force">Overwrite the remote branch</Check>
      <p v-if="force" class="hint">
        Only if it is still where your last fetch left it, so this cannot silently
        bury someone else's commits.
      </p>

      <ul v-if="done.length" class="results">
        <li v-for="line in done" :key="line" class="mono">{{ line }}</li>
      </ul>

      <p v-if="state.error" class="error">{{ state.error }}</p>
      <p v-if="isRejected(state.error)" class="hint">
        Fetch first and look at what arrived. Overwriting is below, if you are sure.
      </p>
      <p v-else-if="isStaleInfo(state.error)" class="hint">
        The remote moved since your last fetch. Fetch again before overwriting.
      </p>

      <div class="actions">
        <button type="button" @click="emit('close')">{{ done.length ? 'Close' : 'Cancel' }}</button>
        <button
          type="button"
          :class="force ? 'danger' : 'primary'"
          :disabled="!branch || !state.remotes.length || busy"
          @click="submit"
        >
          {{ force ? 'Force push' : 'Push' }}
        </button>
      </div>
    </template>
  </Modal>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

.remote {
  width: 100%;
  padding: 6px 8px;
  font: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  margin-bottom: 4px;
}

.warn {
  color: var(--amber);
}

.results {
  margin: 12px 0 0;
  padding: 8px;
  list-style: none;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-size: var(--fs-small);
  user-select: text;
}
</style>
