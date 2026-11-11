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
/* A hairline you can grab: the hit area is wider than the line it draws. */
.splitter {
  position: relative;
  height: 7px;
  margin: -3px 0;
  cursor: row-resize;
  flex: none;
  z-index: var(--z-float);
}

.splitter::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 3px;
  height: 1px;
  background: var(--border);
  transition: background 0.1s ease;
}

.splitter:hover::after {
  height: 2px;
  top: 2.5px;
  background: var(--accent);
}
</style>
