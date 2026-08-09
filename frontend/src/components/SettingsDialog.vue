<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppearanceSettings from '@/components/AppearanceSettings.vue'
import Modal from '@/components/Modal.vue'

const props = defineProps<{ open: boolean; section: string }>()
const emit = defineEmits<{ close: [] }>()

/** Add a section here and it appears in the list; nothing else to wire. */
const sections = [
  { key: 'appearance', title: 'Appearance' },
]

const current = ref(props.section || 'appearance')

const isOpen = computed({
  get: () => props.open,
  set: (v) => {
    if (!v) emit('close')
  },
})

// The menu opens the settings straight at the section you asked for.
watch(
  () => [props.open, props.section],
  ([open, section]) => {
    if (open && section) current.value = section as string
  },
)
</script>

<template>
  <Modal v-model:open="isOpen" title="Settings" wide>
    <div class="settings">
      <nav>
        <button
          v-for="s in sections"
          :key="s.key"
          :class="{ on: current === s.key }"
          @click="current = s.key"
        >
          {{ s.title }}
        </button>
      </nav>

      <div class="body">
        <AppearanceSettings v-show="current === 'appearance'" />
      </div>
    </div>

    <div class="actions">
      <button type="button" class="primary" @click="emit('close')">Done</button>
    </div>
  </Modal>
</template>

<style scoped>
.settings {
  display: flex;
  gap: 14px;
  min-height: 260px;
}

nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: none;
  width: 130px;
  border-right: 1px solid var(--border);
  padding-right: 10px;
}

nav button {
  text-align: left;
  padding: 5px 8px;
  color: var(--text-muted);
  font-size: var(--fs-small);
}

nav button.on {
  background: var(--bg-selected);
  color: var(--text);
}

.body {
  flex: 1 1 auto;
  min-width: 0;
}
</style>
