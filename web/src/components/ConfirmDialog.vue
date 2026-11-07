<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
import { copy } from "../copy";

const props = defineProps<{
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  danger?: boolean;
}>();

const emit = defineEmits<{ confirm: []; cancel: [] }>();

const panel = ref<HTMLElement | null>(null);
const confirmButton = ref<HTMLButtonElement | null>(null);
let previousFocus: HTMLElement | null = null;

function focusable(): HTMLElement[] {
  if (panel.value === null) {
    return [];
  }
  return Array.from(
    panel.value.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => !element.hasAttribute("disabled"));
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") {
    event.preventDefault();
    emit("cancel");
    return;
  }
  if (event.key !== "Tab") {
    return;
  }
  const list = focusable();
  if (list.length === 0) {
    event.preventDefault();
    panel.value?.focus();
    return;
  }
  const first = list[0];
  const last = list[list.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      await nextTick();
      confirmButton.value?.focus();
      document.addEventListener("keydown", onKeydown);
    } else {
      document.removeEventListener("keydown", onKeydown);
      previousFocus?.focus();
      previousFocus = null;
    }
  },
);

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="confirm-backdrop" @click.self="emit('cancel')">
      <section ref="panel" class="confirm" role="alertdialog" aria-modal="true" :aria-label="title" tabindex="-1">
        <h2 class="panel-title">{{ title }}</h2>
        <p class="confirm-message">{{ message }}</p>
        <div class="confirm-actions">
          <button type="button" class="btn" @click="emit('cancel')">{{ copy.common.cancel }}</button>
          <button ref="confirmButton" type="button" :class="danger ? 'btn btn-danger' : 'btn btn-primary'" @click="emit('confirm')">
            {{ confirmLabel }}
          </button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-backdrop {
  position: fixed;
  inset: 0;
  z-index: 40;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--sp-16);
}

.confirm {
  width: min(440px, 100%);
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  padding: var(--sp-16);
  display: flex;
  flex-direction: column;
  gap: var(--sp-12);
  outline: none;
}

.confirm-message {
  margin: 0;
  color: var(--text-dim);
  font-size: var(--fs-14);
  overflow-wrap: anywhere;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--sp-8);
}
</style>
