<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { credentials } from '@wails/go/models'
import { AnswerCredentials, CancelCredentials } from '@wails/go/main/App'
import { EventsOn } from '@wails/runtime/runtime'
import Check from '@/components/common/Check.vue'
import Modal from '@/components/common/Modal.vue'

interface Ask {
  id: string
  url: string
  username: string
}

// Git can ask from several tabs at once; the window answers them one at a time.
const queue = ref<Ask[]>([])
const current = computed(() => queue.value[0] ?? null)

const username = ref('')
const password = ref('')
const remember = ref(true)
const userField = ref<HTMLInputElement | null>(null)
const passField = ref<HTMLInputElement | null>(null)

EventsOn('credentials:ask', (a: Ask) => queue.value.push(a))
EventsOn('credentials:gone', (id: string) => {
  queue.value = queue.value.filter((a) => a.id !== id)
})

watch(current, async (a) => {
  if (!a) return
  username.value = a.username
  password.value = ''
  await nextTick()
  ;(a.username ? passField : userField).value?.focus()
})

const host = computed(() => current.value?.url.replace(/^\w+:\/\//, '') ?? '')

function reply(ok: boolean) {
  const a = current.value
  if (!a) return
  queue.value.shift()
  if (!ok) {
    CancelCredentials(a.id)
    return
  }
  AnswerCredentials(
    a.id,
    credentials.Answer.createFrom({
      username: username.value.trim(),
      password: password.value,
      remember: remember.value,
    }),
  )
  password.value = ''
}

const isOpen = computed({
  get: () => current.value !== null,
  set: (v) => {
    if (!v) reply(false)
  },
})
</script>

<template>
  <Modal v-model:open="isOpen" :title="`Sign in to ${host}`" :width="400">
    <form class="stack" @submit.prevent="reply(true)">
      <div class="field">
        <label for="cred-user">Username</label>
        <input
          id="cred-user"
          ref="userField"
          v-model="username"
          type="text"
          autocomplete="username"
          :readonly="!!current?.username"
        />
      </div>
      <div class="field">
        <label for="cred-pass">Password or token</label>
        <input
          id="cred-pass"
          ref="passField"
          v-model="password"
          type="password"
          autocomplete="current-password"
        />
      </div>
      <Check v-model="remember">Remember</Check>
      <div class="actions">
        <button type="button" @click="reply(false)">Cancel</button>
        <button type="submit" class="primary" :disabled="!username.trim() || !password">
          Sign in
        </button>
      </div>
    </form>
  </Modal>
</template>
