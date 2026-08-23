<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import { CloneParent, PreviewClone, SelectFolder } from '@wails/go/main/App'
import Check from '@/components/common/Check.vue'
import Modal from '@/components/common/Modal.vue'
import TrustHost from '@/components/remote/TrustHost.vue'
import { useRepo } from '@/composables/useRepo'
import { isUnknownHost, remoteHost } from '@/utils/errors'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { state, clone, cancelClone } = useRepo()

const url = ref('')
const parent = ref('')
const name = ref('')
const submodules = ref(false)
const preview = ref<git.ClonePreview | null>(null)
const first = ref<HTMLInputElement | null>(null)

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

const ready = computed(() => !!url.value.trim() && !!parent.value.trim() && !state.busy)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    state.error = ''
    url.value = ''
    name.value = ''
    submodules.value = false
    parent.value = await CloneParent()
    await nextTick()
    first.value?.focus()
  },
)

/** Answers can come back out of order while typing; only the last one counts. */
let asked = 0

// Where it would land, worked out in Go: the folder name comes from the address.
watch([url, parent, name], async () => {
  const seq = ++asked
  if (!url.value.trim() || !parent.value.trim()) {
    preview.value = null
    return
  }
  const p = await PreviewClone({
    url: url.value,
    parent: parent.value,
    name: name.value,
    recursive: false,
  })
  if (seq === asked) preview.value = p
})

async function pickFolder() {
  const dir = await SelectFolder('Clone into', parent.value)
  if (dir) parent.value = dir
}

async function submit() {
  if (!ready.value) return
  const ok = await clone({
    url: url.value.trim(),
    parent: parent.value.trim(),
    name: name.value.trim(),
    recursive: submodules.value,
  })
  if (ok) emit('close')
}
</script>

<template>
  <Modal v-model:open="isOpen" title="Clone repository" :width="560">
    <form class="stack" @submit.prevent="submit">
      <div class="field">
        <label for="clone-url">Repository URL</label>
        <input
          id="clone-url"
          ref="first"
          v-model="url"
          type="text"
          placeholder="git@github.com:you/project.git"
          :disabled="state.busy"
        />
      </div>

      <div class="field">
        <label for="clone-parent">Clone into</label>
        <div class="row">
          <input id="clone-parent" v-model="parent" type="text" :disabled="state.busy" />
          <button type="button" :disabled="state.busy" @click="pickFolder">Browse…</button>
        </div>
      </div>

      <!-- Only when the name is in the way: otherwise the address decides it. -->
      <div v-if="preview?.taken || name" class="field">
        <label for="clone-name">Folder name</label>
        <input
          id="clone-name"
          v-model="name"
          type="text"
          :placeholder="preview?.name"
          :disabled="state.busy"
        />
      </div>

      <p v-if="preview?.taken" class="warn">
        <span class="mono">{{ preview.path }}</span> already exists.
      </p>
      <p v-else-if="preview?.path" class="hint">
        Into <span class="mono">{{ preview.path }}</span>
      </p>

      <Check v-model="submodules" :disabled="state.busy">Include submodules</Check>

      <p v-if="state.busy" class="hint mono progress">{{ state.network || 'Starting…' }}</p>

      <TrustHost v-if="isUnknownHost(state.error)" :host="remoteHost(url)" @trusted="submit" />
      <p v-else-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button v-if="state.busy" type="button" @click="cancelClone">Stop</button>
        <button v-else type="button" @click="emit('close')">Cancel</button>
        <button type="submit" class="primary" :disabled="!ready || preview?.taken">
          {{ state.busy ? 'Cloning…' : 'Clone' }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.row {
  display: flex;
  gap: 6px;
}

.row input {
  flex: 1;
  min-width: 0;
}

.row button {
  flex: none;
}

.warn {
  margin: 0;
  font-size: var(--fs-small);
  color: var(--amber);
}

/* A fixed height, so the box does not jump as git redraws its counters. */
.progress {
  height: 16px;
  margin: 0;
  overflow: hidden;
  white-space: nowrap;
}
</style>
