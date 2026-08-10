<script setup lang="ts">
import { ref, watch } from 'vue'
import type { sshkeys } from '@wails/go/models'
import { ScanHostKeys, TrustHostKeys } from '@wails/go/main/App'
import { errorText } from '@/utils/errors'

const props = defineProps<{ host: string }>()
const emit = defineEmits<{ trusted: [] }>()

const keys = ref<sshkeys.HostKey[]>([])
const busy = ref(false)
const error = ref('')
/** Fingerprints are folded away: most people trust and move on. */
const show = ref(false)

/** Where the host publishes its fingerprints, for the ones we know of. */
const docs: Record<string, string> = {
  'github.com': 'https://docs.github.com/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints',
  'gitlab.com': 'https://docs.gitlab.com/ee/user/gitlab_com/#ssh-known_hosts-entries',
}

watch(
  () => props.host,
  async (host) => {
    keys.value = []
    error.value = ''
    show.value = false
    if (!host) return
    busy.value = true
    try {
      keys.value = await ScanHostKeys(host)
    } catch (e) {
      error.value = errorText(e)
    } finally {
      busy.value = false
    }
  },
  { immediate: true },
)

async function trust() {
  busy.value = true
  error.value = ''
  try {
    // The keys shown are the keys stored: never a second scan.
    await TrustHostKeys(props.host, keys.value)
    emit('trusted')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="trust">
    <div class="row">
      <p class="text">First connection to <b>{{ host }}</b>.</p>
      <button type="button" class="primary" :disabled="!keys.length || busy" @click="trust">
        Trust
      </button>
    </div>

    <button type="button" class="link" @click="show = !show">
      {{ show ? 'Hide' : 'Check' }} fingerprints
    </button>

    <template v-if="show">
      <ul v-if="keys.length" class="keys">
        <li v-for="k in keys" :key="k.fingerprint">
          <span class="type">{{ k.type }}</span>
          <span class="mono print">{{ k.fingerprint }}</span>
        </li>
      </ul>
      <p v-else-if="busy" class="hint">Asking {{ host }}…</p>
      <a v-if="docs[host]" :href="docs[host]" target="_blank" class="link">
        What {{ host }} publishes
      </a>
    </template>

    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.trust {
  padding: 8px 10px;
  background: var(--bg);
  border: 1px solid var(--amber);
  border-radius: 5px;
}

.row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.text {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.link {
  display: block;
  margin-top: 4px;
  padding: 0;
  color: var(--accent);
  background: none;
  border: none;
  font-size: var(--fs-small);
}

.link:hover {
  background: none;
  border: none;
  text-decoration: underline;
}

.keys {
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
}

.keys li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 1px 0;
}

.type {
  flex: none;
  width: 62px;
  font-size: 11px;
  color: var(--text-dim);
}

.print {
  overflow: hidden;
  text-overflow: ellipsis;
  user-select: text;
}
</style>
