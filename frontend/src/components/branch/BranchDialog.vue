<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Check from '@/components/common/Check.vue'
import Modal from '@/components/common/Modal.vue'
import { useRepo } from '@/composables/useRepo'
import type { BranchRequest } from '@/types'

const props = defineProps<{ request: BranchRequest | null }>()
const emit = defineEmits<{ close: [] }>()

const { state, createBranch, renameBranch } = useRepo()

const name = ref('')
const andCheckout = ref(true)
const busy = ref(false)
const input = ref<HTMLInputElement | null>(null)

// Opening the dialog resets the form; the old error belongs to the old attempt.
watch(
  () => props.request,
  async (r) => {
    if (!r) return
    name.value = r.kind === 'rename' ? r.from : ''
    andCheckout.value = true
    state.error = ''
    await nextTick()
    input.value?.select()
  },
  { immediate: true },
)

// The request is the single source of truth; Escape and the overlay close it.
const open = computed({
  get: () => !!props.request,
  set: (v) => {
    if (!v) emit('close')
  },
})

async function submit() {
  const r = props.request
  if (!r || !name.value.trim() || busy.value) return

  busy.value = true
  const ok =
    r.kind === 'rename'
      ? await renameBranch(r.from, name.value.trim())
      : await createBranch({
          name: name.value.trim(),
          start: r.start,
          checkout: andCheckout.value,
          track: false,
        })
  busy.value = false

  if (ok) emit('close')
}
</script>

<template>
  <Modal v-model:open="open" :title="request?.kind === 'rename' ? 'Rename branch' : 'New branch'">
    <form @submit.prevent="submit">
      <label for="branch-name">Branch name</label>
      <input id="branch-name" ref="input" v-model="name" type="text" autofocus />

      <p v-if="request?.kind === 'create'" class="hint">
        Starting from <span class="mono">{{ request.startLabel }}</span>
      </p>

      <Check v-if="request?.kind === 'create'" v-model="andCheckout" class="check">
        Check out after creating
      </Check>

      <p v-if="state.error" class="error">{{ state.error }}</p>

      <div class="actions">
        <button type="button" @click="emit('close')">Cancel</button>
        <button type="submit" class="primary" :disabled="!name.trim() || busy">
          {{ request?.kind === 'rename' ? 'Rename' : 'Create' }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<style scoped>
.check {
  margin-top: 10px;
}
</style>
