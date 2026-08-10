<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useSSH } from '@/composables/useSSH'

const props = defineProps<{ active: boolean }>()

const { state, loadKeys, generate, addToAgent, copy, openURL } = useSSH()

const creating = ref(false)
const name = ref('id_ed25519')
const comment = ref('')
const copied = ref('')

watch(
  () => props.active,
  (active) => {
    if (!active) return
    state.error = ''
    state.fresh = null
    creating.value = false
    copied.value = ''
    void loadKeys()
  },
  { immediate: true },
)

/** A key the agent holds is one git can use without asking anything. */
const usable = computed(() => state.keys.filter((k) => k.inAgent))

async function create() {
  if (await generate(name.value, comment.value)) creating.value = false
}

async function copyKey(text: string) {
  await copy(text)
  copied.value = text
  window.setTimeout(() => (copied.value === text ? (copied.value = '') : null), 2000)
}
</script>

<template>
  <div class="pane">
    <p class="lead">Keys git authenticates with. Ready means the agent holds it.</p>

    <ul v-if="state.keys.length" class="keys">
      <li v-for="k in state.keys" :key="k.path">
        <span class="who">
          <span class="head">
            {{ k.name }}
            <span class="tag">{{ k.type }}</span>
            <span v-if="k.encrypted" class="tag">passphrase</span>
          </span>
          <span class="mono muted">{{ k.comment || k.fingerprint }}</span>
        </span>

        <span v-if="k.inAgent" class="state ok" title="Loaded in the agent">ready</span>
        <button v-else class="load" title="Load it into the ssh-agent" @click="addToAgent(k.path)">
          load
        </button>

        <button class="copy" title="Copy the public key" @click="copyKey(k.public)">
          {{ copied === k.public ? 'copied' : 'copy' }}
        </button>
      </li>
    </ul>

    <p v-else class="hint">No keys in ~/.ssh yet.</p>

    <!-- A brand new key is useless until it is on GitHub, so it is put in the way. -->
    <section v-if="state.fresh" class="fresh">
      <p><b>{{ state.fresh.name }}</b> is ready. GitHub needs its public half:</p>
      <pre class="pub mono">{{ state.fresh.public }}</pre>
      <div class="actions">
        <button @click="copyKey(state.fresh.public)">
          {{ copied === state.fresh.public ? 'Copied' : 'Copy public key' }}
        </button>
        <button class="primary" @click="openURL('https://github.com/settings/ssh/new')">
          Open GitHub to paste it
        </button>
      </div>
    </section>

    <form v-else-if="creating" class="add" @submit.prevent="create">
      <label for="key-name">File name</label>
      <input id="key-name" v-model="name" type="text" placeholder="id_ed25519" />

      <label for="key-comment">Comment</label>
      <input id="key-comment" v-model="comment" type="text" placeholder="you@example.com" />

      <!-- Why there is no passphrase field is in DESIGN.md, not on screen. -->
      <p class="hint">
        ed25519, no passphrase. Add one with
        <span class="mono">ssh-keygen -p -f ~/.ssh/{{ name || 'id_ed25519' }}</span>
      </p>

      <div class="actions">
        <button type="button" @click="creating = false">Cancel</button>
        <button type="submit" class="primary" :disabled="!name.trim() || state.busy">
          Create key
        </button>
      </div>
    </form>

    <button v-else class="add-link" @click="creating = true">+ Create a new key</button>

    <p v-if="state.error" class="error">{{ state.error }}</p>

    <p v-if="!usable.length && state.keys.length && !state.fresh" class="hint warn">
      No key is loaded, so pushing over SSH will fail.
    </p>

  </div>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

.keys {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 5px;
}

.keys li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 9px;
}

.keys li + li {
  border-top: 1px solid var(--border);
}

.who {
  display: flex;
  flex-direction: column;
  gap: 1px;
  flex: 1 1 auto;
  min-width: 0;
}

.who > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.head {
  font-weight: 600;
  color: var(--text);
}

.tag {
  margin-left: 6px;
  padding: 0 5px;
  border-radius: 3px;
  background: var(--bg);
  font-weight: 400;
  font-size: 11px;
  color: var(--text-dim);
}

.who .mono {
  font-size: var(--fs-small);
}

.state.ok {
  flex: none;
  font-size: var(--fs-small);
  color: var(--green);
}

.load,
.copy {
  flex: none;
  font-size: var(--fs-small);
  color: var(--text-muted);
}

.fresh {
  margin-top: 12px;
  padding: 10px;
  background: var(--bg);
  border: 1px solid var(--green);
  border-radius: 5px;
}

.fresh p {
  margin: 0 0 8px;
  font-size: var(--fs-small);
}

.pub {
  margin: 0;
  padding: 8px;
  max-height: 90px;
  overflow: auto;
  background: var(--bg-raised);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-size: 11px;
  word-break: break-all;
  white-space: pre-wrap;
  user-select: text;
}

.add {
  margin-top: 10px;
  padding: 10px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 5px;
}

.warn {
  color: var(--amber);
}

.add-link {
  margin-top: 8px;
  padding: 4px 0;
  color: var(--accent);
  background: none;
  border: none;
}

.add-link:hover {
  background: none;
  border: none;
  text-decoration: underline;
}
</style>
