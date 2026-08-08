<script setup lang="ts">
import { onMounted } from 'vue'
import { usePrefs } from '@/composables/usePrefs'

const emit = defineEmits<{ open: [path: string] }>()

const { state, loadRecents, forgetRepo } = usePrefs()

onMounted(loadRecents)

/** Shortens the path for display; the full one is in the title attribute. */
function shorten(path: string): string {
  return path.replace(/^\/home\/[^/]+/, '~')
}
</script>

<template>
  <div v-if="state.recents.length" class="recents">
    <h2>Recent</h2>

    <ul>
      <li
        v-for="r in state.recents"
        :key="r.path"
        :class="{ missing: r.missing }"
        :title="r.missing ? `${r.path} — not found` : r.path"
        @click="!r.missing && emit('open', r.path)"
      >
        <span class="name">{{ r.name }}</span>
        <span class="path muted">{{ shorten(r.path) }}</span>
        <button
          class="forget"
          title="Remove from this list"
          @click.stop="forgetRepo(r.path)"
        >
          ×
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.recents {
  width: 460px;
  max-width: 90%;
  margin-top: 18px;
}

h2 {
  margin: 0 0 6px;
  font-size: var(--fs-small);
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

ul {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 220px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 5px;
}

li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 8px;
  cursor: default;
}

li + li {
  border-top: 1px solid var(--border);
}

li:hover {
  background: var(--bg-hover);
}

/* A missing folder stays listed: an unplugged drive is not a reason to forget. */
li.missing {
  color: var(--text-dim);
}

li.missing .name {
  text-decoration: line-through;
}

.name {
  font-weight: 600;
}

.path {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  direction: rtl;
  text-align: left;
  font-size: var(--fs-small);
}

.forget {
  flex: none;
  padding: 0 5px;
  color: var(--text-dim);
  visibility: hidden;
}

li:hover .forget {
  visibility: visible;
}
</style>
