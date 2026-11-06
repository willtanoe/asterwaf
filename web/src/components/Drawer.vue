<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
import { copy } from "../copy";

const props = defineProps<{
  open: boolean;
  title: string;
}>();

const emit = defineEmits<{ close: [] }>();

const panel = ref<HTMLElement | null>(null);
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
    emit("close");
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
      panel.value?.focus();
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
    <div v-if="open" class="drawer-backdrop" @click.self="emit('close')">
      <section
        ref="panel"
        class="drawer"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        tabindex="-1"
      >
        <header class="drawer-header">
          <h2 class="panel-title">{{ title }}</h2>
          <button type="button" class="btn btn-small" @click="emit('close')">
            {{ copy.logs.detail.close }}
          </button>
        </header>
        <div class="drawer-body">
          <slot />
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.drawer-backdrop {
  position: fixed;
  inset: 0;
  z-index: 20;
  background: var(--overlay);
  display: flex;
  justify-content: flex-end;
}

.drawer {
  width: min(560px, 100%);
  height: 100%;
  overflow-y: auto;
  background: var(--bg-panel);
  border-left: 1px solid var(--border-strong);
  outline: none;
}

.drawer-header {
  position: sticky;
  top: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-12);
  padding: var(--sp-12) var(--sp-16);
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
}

.drawer-body {
  padding: var(--sp-16);
  display: flex;
  flex-direction: column;
  gap: var(--sp-16);
}
</style>
