<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import Modal from '@/components/common/Modal.vue'
import Select from '@/components/common/Select.vue'
import { useRepo } from '@/composables/useRepo'
import type { ResetRequest } from '@/types'

const props = defineProps<{ request: ResetRequest | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, resetBranch, previewReset } = useRepo()

const mode = ref('mixed')
const busy = ref(false)
const preview = ref<git.ResetPreview | null>(null)

// The colour is the warning: how much of your work each one takes with it.
const modes = [
  { value: 'soft', label: 'Soft', dot: 'var(--green)' },
  { value: 'mixed', label: 'Mixed', dot: 'var(--amber)' },
  { value: 'hard', label: 'Hard', dot: 'var(--red)' },
]

const says: Record<string, string> = {
  soft: 'Stay staged, ready to commit again.',
  mixed: 'Stay in your files, but not staged.',
  hard: 'Are deleted, along with anything uncommitted.',
}

const hard = computed(() => mode.value === 'hard')
const here = computed(() => !!preview.value && !preview.value.behind && !preview.value.ahead)
// Only a hard reset touches the working tree, so only it can lose unsaved work.
const losesWork = computed(() => hard.value && !!preview.value?.dirty)

watch(
  () => props.request,
  async (r) => {
    if (!r) return
    mode.value = 'mixed'
    preview.value = null
    state.error = ''
    preview.value = await previewReset(r.commit.hash)
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

  busy.value = true
  const ok = await resetBranch(r.commit.hash, mode.value)
  busy.value = false

  if (ok) emit('close')
}
</script>

<template>
  <Modal v-model:open="open" title="Reset branch">
    <div class="stack">
      <div class="field">
        <label>Move {{ request?.branch || 'HEAD' }} to</label>
        <p class="commit">
          <span class="mono">{{ request?.commit.hash.slice(0, 7) }}</span>
          {{ request?.commit.subject }}
        </p>
      </div>

      <div class="field">
        <label>Your changes</label>
        <Select v-model="mode" :options="modes" />
        <p class="says">{{ says[mode] }}</p>
      </div>

      <!-- Resetting onto HEAD moves nothing; only the changes setting still acts. -->
      <p v-if="here" class="hint">{{ request?.branch || 'HEAD' }} is already on this commit.</p>
      <p v-else-if="preview?.behind" class="hint">
        {{ preview.behind }} commit{{ preview.behind > 1 ? 's' : '' }} left behind.
      </p>
      <p v-else-if="preview?.ahead" class="hint">
        Moves forward by {{ preview.ahead }} commit{{ preview.ahead > 1 ? 's' : '' }}.
      </p>

      <p v-if="losesWork" class="error">Uncommitted changes will be lost.</p>
      <p v-else-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button
          type="button"
          :class="hard ? 'danger' : 'primary'"
          :disabled="busy || (here && mode === 'soft')"
          @click="confirm"
        >
          Reset
        </button>
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

.says {
  margin: 1px 0 0;
  font-size: var(--fs-small);
  color: var(--text-muted);
}

/* The stack already spaces these, so their own margin would double it. */
.stack > .hint,
.stack > .error {
  margin-top: 0;
}
</style>
