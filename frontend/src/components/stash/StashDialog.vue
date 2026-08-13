<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Check from '@/components/common/Check.vue'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { state, stash } = useRepo()

const message = ref('')
const untracked = ref(true)
const keepIndex = ref(false)
const busy = ref(false)
const input = ref<HTMLInputElement | null>(null)

watch(
  () => props.open,
  async (isOpen) => {
    if (!isOpen) return
    message.value = ''
    untracked.value = true
    keepIndex.value = false
    state.error = ''
    await nextTick()
    input.value?.focus()
  },
)

const model = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function submit() {
  if (busy.value) return

  busy.value = true
  const ok = await stash({
    message: message.value.trim(),
    untracked: untracked.value,
    keepIndex: keepIndex.value,
    paths: [],
  })
  busy.value = false

  if (ok) emit('close')
}
</script>

<template>
  <Modal v-model:open="model" title="Stash changes">
    <form @submit.prevent="submit">
      <label for="stash-message">Message</label>
      <input id="stash-message" ref="input" v-model="message" type="text" />
      <p class="hint">Left empty, git writes its own.</p>

      <Check v-model="untracked" class="check">Include untracked files</Check>
      <Check v-model="keepIndex" class="check">Keep staged changes staged</Check>

      <p v-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button type="submit" class="primary" :disabled="busy">Stash</button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.check {
  margin-top: 10px;
}
</style>
