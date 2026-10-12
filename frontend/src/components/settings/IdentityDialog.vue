<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { prefs } from '@wails/go/models'
import Modal from '@/components/common/Modal.vue'
import { usePrefs } from '@/composables/usePrefs'

const props = defineProps<{ open: boolean; repoId: string | null; repoName: string }>()
const emit = defineEmits<{ close: [] }>()

const { state, loadIdentities, addIdentity, removeIdentity, applyIdentity, useGlobalIdentity } =
  usePrefs()

/** One line of the list: the global identity, a saved one, or this repository's own. */
interface Choice {
  key: string
  /** The bold line. A saved identity is named by its own name; the others need a caption. */
  title: string
  name: string
  email: string
  kind: 'global' | 'saved' | 'own'
}

const adding = ref(false)
const busy = ref(false)
const draft = ref<prefs.Identity>({ name: '', email: '' })
const firstField = ref<HTMLInputElement | null>(null)

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    state.error = ''
    adding.value = false
    if (props.repoId) loadIdentities(props.repoId)
  },
)

/** True while the repository sets its own identity instead of inheriting one. */
const isLocal = computed(() => state.signature?.emailScope === 'local')

const choices = computed<Choice[]>(() => {
  const global = state.global
  const list: Choice[] = [
    {
      key: 'global',
      title: 'My usual identity',
      name: global?.name ?? '',
      email: global?.email ?? '',
      kind: 'global',
    },
  ]

  for (const i of state.saved) {
    list.push({ key: i.email, title: i.name, name: i.name, email: i.email, kind: 'saved' })
  }

  // An identity set by hand, or by another client, would otherwise leave nothing ticked.
  const sig = state.signature
  if (isLocal.value && sig && !state.saved.some((i) => i.email === sig.email)) {
    list.push({
      key: sig.email,
      title: sig.name,
      name: sig.name,
      email: sig.email,
      kind: 'own',
    })
  }
  return list
})

/** The line that matches what this repository actually commits as. */
const current = computed(() => (isLocal.value ? (state.signature?.email ?? '') : 'global'))

async function choose(c: Choice) {
  if (!props.repoId || busy.value || c.key === current.value) return
  busy.value = true
  if (c.kind === 'global') await useGlobalIdentity(props.repoId)
  else await applyIdentity(props.repoId, { name: c.name, email: c.email })
  busy.value = false
}

async function drop(c: Choice) {
  if (!props.repoId || busy.value) return
  busy.value = true
  await removeIdentity(props.repoId, c.email)
  busy.value = false
}

async function startAdding() {
  adding.value = true
  state.error = ''
  draft.value = { name: '', email: '' }
  await nextTick()
  firstField.value?.focus()
}

async function saveDraft() {
  if (!props.repoId || busy.value) return
  busy.value = true
  const ok = await addIdentity(props.repoId, draft.value)
  busy.value = false
  if (ok) adding.value = false
}
</script>

<template>
  <Modal v-model:open="isOpen" title="Who commits here" :width="560">
    <p class="lead">
      Commits made in <b>{{ repoName }}</b> are signed with the identity you pick here.
    </p>

    <ul class="choices">
      <li
        v-for="c in choices"
        :key="c.key"
        :class="{ picked: c.key === current, empty: !c.email }"
        @click="choose(c)"
      >
        <span class="tick">{{ c.key === current ? '✓' : '' }}</span>

        <span class="who">
          <span class="head">
            {{ c.title || 'Nobody' }}
            <span v-if="c.kind === 'global'" class="tag">used everywhere else</span>
            <span v-else-if="c.kind === 'own'" class="tag">set outside focagit</span>
          </span>
          <span v-if="c.email" class="mono muted">{{ c.email }}</span>
          <span v-else class="mono warn">not set — git will refuse to commit</span>
        </span>

        <button
          v-if="c.kind === 'saved'"
          class="drop"
          title="Forget this identity"
          @click.stop="drop(c)"
        >
          ×
        </button>
      </li>
    </ul>

    <form v-if="adding" class="add" @submit.prevent="saveDraft">
      <p class="hint">The name and address that will appear on every commit you sign with it.</p>
      <input ref="firstField" v-model="draft.name" type="text" placeholder="Your name" />
      <input v-model="draft.email" type="text" placeholder="you@example.com" />
      <div class="add-actions">
        <button type="button" @click="adding = false">Cancel</button>
        <button type="submit" class="primary" :disabled="busy">Save and use it here</button>
      </div>
    </form>

    <button v-else class="add-link" @click="startAdding">+ Add an identity</button>

    <p v-if="state.error" class="error">{{ state.error }}</p>

    <div class="actions">
      <button type="button" class="primary" @click="emit('close')">Done</button>
    </div>
  </Modal>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

.choices {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 5px;
}

.choices li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 9px;
  cursor: pointer;
}

.choices li + li {
  border-top: 1px solid var(--border);
}

.choices li:hover {
  background: var(--bg-hover);
}

.choices li.picked {
  background: color-mix(in srgb, var(--accent) 14%, transparent);
}

.tick {
  flex: none;
  width: 12px;
  color: var(--blue);
  font-weight: 700;
}

.who {
  display: flex;
  flex-direction: column;
  gap: 1px;
  flex: 1 1 auto;
  min-width: 0;
}

.who > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.who .head {
  font-weight: 600;
  color: var(--text);
}

/* The two lines that are not a saved identity need saying which they are. */
.tag {
  margin-left: 6px;
  font-weight: 400;
  font-size: var(--fs-small);
  color: var(--text-dim);
}

.who .mono {
  font-size: var(--fs-small);
}

.warn {
  color: var(--amber);
}

.drop {
  flex: none;
  padding: 0 6px;
  color: var(--text-dim);
  visibility: hidden;
}

.choices li:hover .drop {
  visibility: visible;
}

.add {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 10px;
  padding: 10px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 5px;
}

.add .hint {
  margin: 0 0 2px;
}

.add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 2px;
}

.add-link {
  margin-top: 8px;
  padding: 4px 0;
  color: var(--blue);
  background: none;
  border: none;
}

.add-link:hover {
  background: none;
  border: none;
  text-decoration: underline;
}
</style>
