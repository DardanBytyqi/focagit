<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Check from '@/components/common/Check.vue'
import Modal from '@/components/common/Modal.vue'
import Select from '@/components/common/Select.vue'
import RemoteForm from '@/components/remote/RemoteForm.vue'
import TrustHost from '@/components/remote/TrustHost.vue'
import { useRepo } from '@/composables/useRepo'
import { isAuthFailure, isRejected, isStaleInfo, isUnknownHost, remoteHost } from '@/utils/errors'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; ssh: [] }>()

const { state, loadRemotes, push } = useRepo()

const remote = ref('')
const setUpstream = ref(true)
const force = ref(false)
const busy = ref(false)
const adding = ref(false)
/** What the remote said about each ref, shown instead of closing on success. */
const done = ref<string[]>([])

/** A freshly added remote is the one you meant to push to. */
function onAdded(name: string) {
  adding.value = false
  remote.value = name
}

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

/** The branch being pushed, and where it already goes if it has been pushed before. */
const branch = computed(() => (state.status?.detached ? '' : (state.status?.head ?? '')))
const upstream = computed(() => state.status?.upstream ?? '')

const remoteOptions = computed(() =>
  state.remotes.map((r) => ({ value: r.name, label: r.name, hint: r.push || r.fetch })),
)

/** The server behind the chosen remote, for talking about it by name. */
const host = computed(() => {
  const r = state.remotes.find((x) => x.name === remote.value)
  return remoteHost(r?.push || r?.fetch || '') || 'the server'
})

/** git's own words, unless we have clearer ones and something to do about it. */
const problem = computed(() => {
  const e = state.error
  if (!e) return null
  if (isRejected(e)) return { text: e, hint: 'Fetch first and see what arrived.' }
  if (isStaleInfo(e)) return { text: e, hint: 'The remote moved. Fetch again.' }
  return { text: e, hint: '' }
})

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    // The error stays: the dialog is often opened because of one.
    done.value = []
    force.value = false
    adding.value = false
    await loadRemotes()
    remote.value = state.remotes[0]?.name ?? ''
    setUpstream.value = !upstream.value
  },
)

/** Trusting the server was the only thing missing, so finish the push. */
async function retryAfterTrust() {
  state.error = ''
  await submit()
}

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
      <p class="hint">HEAD is detached: there is no branch to push.</p>
    </template>

    <!-- Nowhere to push yet: the way out is here, not in a terminal. -->
    <template v-else-if="adding || !state.remotes.length">
      <p class="lead">
        <template v-if="state.remotes.length">Where should this go?</template>
        <template v-else>No remote yet — add one.</template>
      </p>
      <RemoteForm
        :active="isOpen && (adding || !state.remotes.length)"
        @added="onAdded"
        @cancel="adding ? (adding = false) : emit('close')"
      />
    </template>

    <div v-else class="stack">
      <p class="lead">
        Pushing <b>{{ branch }}</b>
        <template v-if="upstream">
          to <span class="mono">{{ upstream }}</span>
        </template>
      </p>

      <div class="field">
        <label>Remote</label>
        <Select v-model="remote" :options="remoteOptions" />
        <button type="button" class="add-link" @click="adding = true">+ Add another remote</button>
      </div>

      <div class="field">
        <Check v-model="setUpstream" :disabled="!!upstream && !force">
          Track this remote branch from now on
        </Check>
        <Check v-model="force">Overwrite the remote branch</Check>
        <p v-if="force" class="hint">Only if it is still where your last fetch left it.</p>
      </div>

      <ul v-if="done.length" class="results">
        <li v-for="line in done" :key="line" class="mono">{{ line }}</li>
      </ul>

      <TrustHost v-if="isUnknownHost(state.error)" :host="host" @trusted="retryAfterTrust" />

      <div v-else-if="problem" class="field">
        <p class="error">{{ problem.text }}</p>
        <p v-if="problem.hint" class="hint">{{ problem.hint }}</p>
        <!-- Over SSH, "who are you" is nearly always a key the agent is not holding. -->
        <button v-if="isAuthFailure(state.error)" type="button" class="link" @click="emit('ssh')">
          Check your SSH keys…
        </button>
      </div>

      <div class="actions">
        <button type="button" class="link ssh" @click="emit('ssh')">SSH keys…</button>
        <span class="spacer"></span>
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
    </div>
  </Modal>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

.add-link,
.link {
  margin-top: 2px;
  padding: 2px 0;
  color: var(--accent);
  background: none;
  border: none;
  font-size: var(--fs-small);
}

.add-link:hover,
.link:hover {
  background: none;
  border: none;
  text-decoration: underline;
}

/* The SSH link sits on the left, away from the button that actually pushes. */
.actions .spacer {
  flex: 1;
}

.actions .ssh {
  align-self: center;
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
