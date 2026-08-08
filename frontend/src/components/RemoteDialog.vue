<script setup lang="ts">
import { computed, watch } from 'vue'
import Modal from '@/components/Modal.vue'
import RemoteForm from '@/components/RemoteForm.vue'
import { useRepo } from '@/composables/useRepo'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { state, loadRemotes } = useRepo()

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

// The form needs the current list to know whether "origin" is still free.
watch(
  () => props.open,
  (open) => {
    if (!open) return
    state.error = ''
    void loadRemotes()
  },
)
</script>

<template>
  <Modal v-model:open="isOpen" title="Add remote">
    <RemoteForm :active="open" @added="emit('close')" @cancel="emit('close')" />
    <p v-if="state.remotes.length" class="hint">
      Already configured: <span class="mono">{{ state.remotes.map((r) => r.name).join(', ') }}</span>
    </p>
  </Modal>
</template>
