<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from 'reka-ui'
import type { git } from '@wails/go/models'
import { useRepo } from '@/composables/useRepo'
import type { MergeDrop } from '@/types'

const props = defineProps<{ drop: MergeDrop | null }>()
const emit = defineEmits<{ merge: [noFastForward: boolean]; close: [] }>()

const { previewMerge } = useRepo()

const preview = ref<git.MergePreview | null>(null)

const isOpen = computed({
  get: () => !!props.drop,
  set: (v) => {
    if (!v) emit('close')
  },
})

// Asked here rather than guessed: only Go knows what the two refs contain.
watch(
  () => props.drop,
  async (drop) => {
    preview.value = null
    if (!drop) return
    preview.value = await previewMerge(drop.source.short, drop.target.short)
  },
  { immediate: true },
)

/** Landing on a branch we are not on means switching to it first. */
const switching = computed(() => props.drop && !props.drop.target.head)
</script>

<template>
  <DropdownMenuRoot v-model:open="isOpen">
    <!-- An anchor with no size, parked where the pointer let go. -->
    <DropdownMenuTrigger
      class="anchor"
      :style="{ left: `${drop?.x ?? 0}px`, top: `${drop?.y ?? 0}px` }"
    />
    <DropdownMenuPortal>
      <DropdownMenuContent class="menu" align="start" :side-offset="0">
        <template v-if="preview?.upToDate">
          <DropdownMenuItem class="menu-item" disabled>
            {{ drop?.target.short }} already has {{ drop?.source.short }}
          </DropdownMenuItem>
        </template>

        <template v-else>
          <DropdownMenuItem class="menu-item" @select="emit('merge', false)">
            Merge {{ drop?.source.short }} into {{ drop?.target.short }}
            <span v-if="switching" class="note">switches to {{ drop?.target.short }}</span>
          </DropdownMenuItem>

          <!-- Only offered when git could otherwise just move the branch. -->
          <DropdownMenuItem
            v-if="preview?.fastForward"
            class="menu-item"
            @select="emit('merge', true)"
          >
            Merge without fast-forward
            <span class="note">keeps the branch visible in the graph</span>
          </DropdownMenuItem>
        </template>

        <DropdownMenuSeparator class="menu-sep" />
        <DropdownMenuItem class="menu-item" @select="emit('close')">Cancel</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

<style scoped>
.anchor {
  position: fixed;
  width: 0;
  height: 0;
  padding: 0;
  border: none;
  background: none;
}

/* The second line explains the item without becoming another item. */
.note {
  display: block;
  font-size: 11px;
  color: var(--text-dim);
}

.menu-item[data-highlighted] .note {
  color: var(--text-muted);
}
</style>
