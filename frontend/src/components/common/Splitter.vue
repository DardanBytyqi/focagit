<script setup lang="ts">
/** A draggable line that resizes the pane *below* it. */
const height = defineModel<number>({ required: true })

const props = withDefaults(defineProps<{ min?: number; max?: number }>(), {
  min: 80,
  max: 600,
})

function start(e: MouseEvent) {
  const startY = e.clientY
  const startHeight = height.value

  // Dragging up makes the lower pane taller, hence the subtraction.
  const onMove = (ev: MouseEvent) => {
    const next = startHeight - (ev.clientY - startY)
    height.value = Math.min(props.max, Math.max(props.min, next))
  }
  // Listening on the window, not the line: the pointer outruns a 5px target.
  const onUp = () => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }

  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}
</script>

<template>
  <div class="splitter" @mousedown.prevent="start"></div>
</template>

<style scoped>
.splitter {
  height: 5px;
  background: var(--border);
  cursor: row-resize;
  flex: none;
}

.splitter:hover {
  background: var(--accent);
}
</style>
