<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Modal from '@/components/Modal.vue'
import { useRepo } from '@/composables/useRepo'
import { isNotMerged } from '@/utils/errors'
import type { DeleteRequest } from '@/types'

const props = defineProps<{ request: DeleteRequest | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, deleteBranch } = useRepo()

const busy = ref(false)
// Set once git has refused a plain delete, which turns the button into a force.
const unmerged = ref(false)

watch(
  () => props.request,
  (r) => {
    if (!r) return
    unmerged.value = false
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

  busy.value = true
  const ok = await deleteBranch(r.name, unmerged.value)
  busy.value = false

  if (ok) {
    emit('close')
  } else if (isNotMerged(state.error)) {
    // Not a dead end: the same dialog now offers the forced delete.
    unmerged.value = true
  }
}
</script>

<template>
  <Modal v-model:open="open" title="Delete branch">
    <p>
      Delete <span class="mono">{{ request?.name }}</span
      >?
    </p>

    <p v-if="unmerged" class="error">
      This branch has commits that are on no other branch. Deleting it now loses them.
    </p>
    <p v-else-if="state.error" class="error">{{ state.error }}</p>

    <div class="actions">
      <button type="button" @click="emit('close')">Cancel</button>
      <button
        type="button"
        :class="unmerged ? 'danger' : 'primary'"
        :disabled="busy"
        @click="confirm"
      >
        {{ unmerged ? 'Delete anyway' : 'Delete' }}
      </button>
    </div>
  </Modal>
</template>
