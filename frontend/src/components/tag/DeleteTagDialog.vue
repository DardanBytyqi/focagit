<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { git } from '@wails/go/models'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'

const props = defineProps<{ tag: git.Tag | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, deleteTag, pushTag } = useRepo()

const busy = ref(false)

watch(
  () => props.tag,
  (t) => {
    if (t) state.error = ''
  },
  { immediate: true },
)

const open = computed({
  get: () => !!props.tag,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function remove(alsoRemote: boolean) {
  const t = props.tag
  if (!t || busy.value) return

  busy.value = true
  // The remote first: once it is gone here, there is no name left to send.
  const sent = alsoRemote ? !!(await pushTag(t.name, true)) : true
  if (sent) await deleteTag(t.name)
  busy.value = false

  if (sent && !state.error) emit('close')
}
</script>

<template>
  <Modal v-model:open="open" title="Delete tag">
    <div class="stack">
      <p>
        Delete <span class="mono">{{ tag?.name }}</span
        >?
      </p>

      <p v-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button type="button" :disabled="busy" @click="remove(true)">Here and on origin</button>
        <button type="button" class="danger" :disabled="busy" @click="remove(false)">
          Here only
        </button>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
/* The stack already spaces these, so their own margin would double it. */
.stack > .error {
  margin-top: 0;
}
</style>
