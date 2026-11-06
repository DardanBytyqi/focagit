<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ name: string; email?: string; size?: number }>()

// FNV-1a, so the same person gets the same mark on every machine.
function hash(s: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  // murmur3's finaliser, so every bit depends on the whole address.
  h = Math.imul(h ^ (h >>> 16), 0x85ebca6b)
  h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35)
  return (h ^ (h >>> 16)) >>> 0
}

// A 5×5 pixel face mirrored around the middle column: 15 bits pick the cells.
const mark = computed(() => {
  const h = hash((props.email || props.name).toLowerCase())
  const cells: { x: number; y: number }[] = []
  for (let y = 0; y < 5; y++) {
    for (let x = 0; x < 3; x++) {
      if (!((h >>> (y * 3 + x)) & 1)) continue
      cells.push({ x, y })
      if (x < 2) cells.push({ x: 4 - x, y })
    }
  }
  // Ten hues 36° apart, from bits the pattern does not use.
  return { cells, hue: ((h >>> 15) % 10) * 36 + 20 }
})
</script>

<template>
  <svg
    class="author-mark"
    viewBox="0 0 15 15"
    :width="size ?? 15"
    :height="size ?? 15"
    :style="{ color: `oklch(var(--mark-l) 0.14 ${mark.hue})` }"
    aria-hidden="true"
  >
    <rect width="15" height="15" rx="3" fill="currentColor" fill-opacity="0.2" />
    <rect
      v-for="c in mark.cells"
      :key="`${c.x}-${c.y}`"
      :x="c.x * 3"
      :y="c.y * 3"
      width="3"
      height="3"
      fill="currentColor"
      shape-rendering="crispEdges"
    />
  </svg>
</template>

<style scoped>
.author-mark {
  flex: none;
}
</style>
