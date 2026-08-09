<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import Check from '@/components/Check.vue'
import { useRepo } from '@/composables/useRepo'
import { isUnknownHost, remoteHost } from '@/utils/errors'

const props = defineProps<{ active: boolean }>()
const emit = defineEmits<{ added: [name: string]; cancel: [] }>()

const { state, addRemote } = useRepo()

// origin is what every clone calls its remote, so it is the name to beat.
const name = ref('origin')
const url = ref('')
const fetchNow = ref(true)
const busy = ref(false)
const first = ref<HTMLInputElement | null>(null)

watch(
  () => props.active,
  async (active) => {
    if (!active) return
    name.value = state.remotes.length ? '' : 'origin'
    url.value = ''
    fetchNow.value = true
    await nextTick()
    first.value?.focus()
  },
  { immediate: true },
)

async function submit() {
  if (!name.value.trim() || !url.value.trim() || busy.value) return
  busy.value = true
  const ok = await addRemote(name.value, url.value, fetchNow.value)
  busy.value = false
  if (ok) emit('added', name.value.trim())
}
</script>

<template>
  <form class="stack" @submit.prevent="submit">
    <div class="field">
      <label for="remote-url">Repository URL</label>
      <input
        id="remote-url"
        ref="first"
        v-model="url"
        type="text"
        placeholder="git@github.com:you/project.git"
      />
    </div>

    <!-- The first remote is always called origin; asking would be noise. -->
    <div v-if="state.remotes.length" class="field">
      <label for="remote-name">Short name</label>
      <input id="remote-name" v-model="name" type="text" placeholder="upstream" />
    </div>

    <Check v-model="fetchNow">Fetch from it now</Check>

    <p v-if="state.error" class="error">{{ state.error }}</p>
    <!-- Only the user can check a fingerprint, so this one ends in a terminal. -->
    <p v-if="isUnknownHost(state.error)" class="hint">
      Run <span class="mono">ssh -T git@{{ remoteHost(url) || 'github.com' }}</span> once and
      check the fingerprint.
    </p>

    <div class="actions">
      <button type="button" @click="emit('cancel')">Cancel</button>
      <button type="submit" class="primary" :disabled="!name.trim() || !url.trim() || busy">
        {{ busy && fetchNow ? 'Fetching…' : 'Add remote' }}
      </button>
    </div>
  </form>
</template>

