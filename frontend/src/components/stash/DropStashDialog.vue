<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'

const props = defineProps<{ stash: git.Stash | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, dropStash } = useRepo()

const busy = ref(false)

watch(
  () => props.stash,
  (s) => {
    if (s) state.error = ''
  },
)

const open = computed({
  get: () => !!props.stash,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function confirm() {
  const s = props.stash
  if (!s || busy.value) return

  busy.value = true
  const ok = await dropStash(s.ref)
  busy.value = false

  if (ok) emit('close')
}
</script>

<template>
  <Modal v-model:open="open" title="Drop stash">
    <p>
      Drop <span class="mono">{{ stash?.message }}</span
      >?
    </p>
    <!-- A dropped entry is unreachable, so this is the one stash action with no way back. -->
    <p class="hint">This cannot be undone.</p>

    <p v-if="state.error" class="error">{{ state.error }}</p>

    <div class="actions">
      <button type="button" @click="emit('close')">Cancel</button>
      <button type="button" class="danger" :disabled="busy" @click="confirm">Drop</button>
    </div>
  </Modal>
</template>
