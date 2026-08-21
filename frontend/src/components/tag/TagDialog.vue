<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'
import { isTagExists } from '@/utils/errors'
import type { TagRequest } from '@/types'

const props = defineProps<{ request: TagRequest | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, createTag } = useRepo()

const name = ref('')
const message = ref('')
const busy = ref(false)
// Set once git has refused the name, which turns the button into a move.
const taken = ref(false)
const input = ref<HTMLInputElement | null>(null)

watch(
  () => props.request,
  async (r) => {
    if (!r) return
    name.value = ''
    message.value = ''
    taken.value = false
    state.error = ''
    await nextTick()
    input.value?.focus()
  },
  { immediate: true },
)

// Editing the name is a fresh attempt, so the warning must not linger.
watch(name, () => {
  taken.value = false
})

const open = computed({
  get: () => !!props.request,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function submit() {
  const r = props.request
  if (!r || busy.value) return

  busy.value = true
  const ok = await createTag({
    name: name.value.trim(),
    rev: r.commit.hash,
    message: message.value.trim(),
    force: taken.value,
  })
  busy.value = false

  if (ok) emit('close')
  else if (isTagExists(state.error)) taken.value = true
}
</script>

<template>
  <Modal v-model:open="open" title="New tag">
    <form class="stack" @submit.prevent="submit">
      <div class="field">
        <label for="tag-name">Name</label>
        <input id="tag-name" ref="input" v-model="name" type="text" placeholder="v1.0.0" />
      </div>

      <div class="field">
        <label for="tag-message">Message</label>
        <input id="tag-message" v-model="message" type="text" />
      </div>

      <p v-if="taken" class="error">That name is taken. Creating it again moves it here.</p>
      <p v-else-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button
          type="submit"
          :class="taken ? 'danger' : 'primary'"
          :disabled="busy || !name.trim()"
        >
          {{ taken ? 'Move it here' : 'Create tag' }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
/* The stack already spaces these, so their own margin would double it. */
.stack > .error {
  margin-top: 0;
}
</style>
