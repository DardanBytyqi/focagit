<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

defineProps<{ title: string; wide?: boolean }>()

const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="overlay" />
      <DialogContent class="dialog" :class="{ wide }">
        <DialogTitle class="title">{{ title }}</DialogTitle>
        <slot />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style>
/* Not scoped: the dialog is teleported out of this component's subtree. */
.overlay {
  position: fixed;
  inset: 0;
  background: rgb(0 0 0 / 0.5);
}

/* inset + auto margins centre on both axes without a transform, which would
   land the box on half pixels and blur the text. */
.dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  width: 380px;
  height: fit-content;
  max-height: 76vh;
  overflow-y: auto;
  padding: 16px;
  background: var(--bg-raised);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  box-shadow: 0 12px 32px rgb(0 0 0 / 0.45);
}

.dialog.wide {
  width: 560px;
}

.dialog .title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}

.dialog label {
  display: block;
  margin-bottom: 4px;
  font-size: var(--fs-small);
  color: var(--text-muted);
}

.dialog input[type='text'] {
  width: 100%;
  padding: 6px 8px;
  font: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  user-select: text;
}

.dialog input[type='text']:focus {
  outline: none;
  border-color: var(--accent);
}

.dialog .actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}

.dialog .actions .primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.dialog .actions .primary:hover:not(:disabled) {
  filter: brightness(1.1);
  background: var(--accent);
  border-color: var(--accent);
}

.dialog .actions .danger {
  background: var(--red);
  border-color: var(--red);
  color: #1e1f22;
}

.dialog .actions .danger:hover:not(:disabled) {
  filter: brightness(1.1);
  background: var(--red);
  border-color: var(--red);
}

.dialog .hint {
  margin: 8px 0 0;
  font-size: var(--fs-small);
  color: var(--text-muted);
}

.dialog .error {
  margin: 10px 0 0;
  font-size: var(--fs-small);
  color: var(--red);
  white-space: pre-wrap;
  user-select: text;
}
</style>
