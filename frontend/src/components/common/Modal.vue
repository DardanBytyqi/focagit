<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

const props = withDefaults(
  defineProps<{
    title: string
    /** Width in pixels. Narrower windows win: it never exceeds the viewport. */
    width?: number
    /**
     * Take a set height and scroll inside, instead of growing with the content.
     * A work surface needs its own scrollbar; a form is as tall as its fields.
     */
    fill?: boolean
  }>(),
  { width: 380 },
)

const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="overlay" />
      <DialogContent
        class="dialog"
        :class="{ fill }"
        :style="{ width: `min(${props.width}px, 95vw)` }"
      >
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
  background: var(--scrim);
  z-index: var(--z-modal);
}

/* inset + auto margins centre on both axes without a transform, which would
   land the box on half pixels and blur the text. */
.dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  z-index: calc(var(--z-modal) + 1);
  height: fit-content;
  max-height: 76vh;
  overflow-y: auto;
  padding: 16px;
  background: var(--bg-raised);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  box-shadow: 0 12px 32px var(--shadow);
}

/* A set height, so the panes inside get the same room whatever the content is. */
.dialog.fill {
  display: flex;
  flex-direction: column;
  height: min(900px, 92vh);
  max-height: 92vh;
  overflow: hidden;
}

/* Only the pane that owns the scrollbar should scroll, not the whole dialog. */
.dialog.fill > .title {
  flex: none;
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

/* A label and its control, kept together so the gap between groups can be wide. */
.dialog .field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.dialog .field label {
  margin-bottom: 0;
}

/* The gap between groups. Forms opt in by carrying this class. */
.dialog .stack {
  display: flex;
  flex-direction: column;
  gap: 11px;
}

/* The buttons carry their own margin, which lands on top of the gap. */
.dialog .stack .actions {
  margin-top: 5px;
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
  color: var(--bg);
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
