<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

export interface SelectOption {
  value: string;
  label: string;
}

const props = defineProps<{
  modelValue: string;
  options: SelectOption[];
  label: string;
}>();

const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const open = ref(false);
const activeIndex = ref(0);
const listId = `select-list-${Math.random().toString(36).slice(2, 8)}`;
const typeahead = ref("");
let typeaheadTimer = 0;

const selectedIndex = computed(() =>
  props.options.findIndex((option) => option.value === props.modelValue),
);
const selectedLabel = computed(() =>
  selectedIndex.value >= 0 ? props.options[selectedIndex.value].label : "",
);

function openList(): void {
  open.value = true;
  activeIndex.value = selectedIndex.value >= 0 ? selectedIndex.value : 0;
}

function toggle(): void {
  if (open.value) {
    open.value = false;
  } else {
    openList();
  }
}

function choose(index: number): void {
  const option = props.options[index];
  if (option === undefined) {
    return;
  }
  emit("update:modelValue", option.value);
  open.value = false;
  typeahead.value = "";
}

function move(delta: number): void {
  const count = props.options.length;
  if (count === 0) {
    return;
  }
  activeIndex.value = (activeIndex.value + delta + count) % count;
}

function jump(index: number): void {
  if (props.options.length > 0) {
    activeIndex.value = Math.min(props.options.length - 1, Math.max(0, index));
  }
}

function typeaheadMatch(query: string): number {
  const lower = query.toLowerCase();
  const start = activeIndex.value + 1;
  for (let offset = 0; offset < props.options.length; offset += 1) {
    const index = (start + offset) % props.options.length;
    if (props.options[index].label.toLowerCase().startsWith(lower)) {
      return index;
    }
  }
  return -1;
}

function onKeydown(event: KeyboardEvent): void {
  if (!open.value) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp" || event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openList();
    }
    return;
  }
  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      move(1);
      break;
    case "ArrowUp":
      event.preventDefault();
      move(-1);
      break;
    case "Home":
      event.preventDefault();
      jump(0);
      break;
    case "End":
      event.preventDefault();
      jump(props.options.length - 1);
      break;
    case "Enter":
      event.preventDefault();
      choose(activeIndex.value);
      break;
    case "Escape":
      event.preventDefault();
      open.value = false;
      typeahead.value = "";
      break;
    case "Tab":
      open.value = false;
      typeahead.value = "";
      break;
    default:
      if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
        typeahead.value += event.key;
        const match = typeaheadMatch(typeahead.value);
        if (match >= 0) {
          jump(match);
        }
        window.clearTimeout(typeaheadTimer);
        typeaheadTimer = window.setTimeout(() => {
          typeahead.value = "";
        }, 500);
      }
  }
}

const root = ref<HTMLElement | null>(null);

function onDocumentPointerDown(event: PointerEvent): void {
  if (root.value !== null && !root.value.contains(event.target as Node)) {
    open.value = false;
    typeahead.value = "";
  }
}

onMounted(() => document.addEventListener("pointerdown", onDocumentPointerDown));
onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  window.clearTimeout(typeaheadTimer);
});

watch(
  () => props.modelValue,
  () => {
    typeahead.value = "";
  },
);
</script>

<template>
  <div ref="root" class="select-box">
    <button
      type="button"
      class="select-button"
      :aria-label="label"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-activedescendant="open ? `${listId}-${activeIndex}` : undefined"
      @click="toggle"
      @keydown="onKeydown"
    >
      <span class="select-value truncate">{{ selectedLabel }}</span>
      <span class="select-caret" aria-hidden="true">▾</span>
    </button>
    <ul v-show="open" :id="listId" class="select-list" role="listbox" :aria-label="label">
      <li
        v-for="(option, index) in options"
        :id="`${listId}-${index}`"
        :key="option.value"
        role="option"
        class="select-option"
        :class="{ 'select-option-active': open && index === activeIndex, 'select-option-selected': index === selectedIndex }"
        :aria-selected="index === selectedIndex"
        @pointerdown.prevent="choose(index)"
        @pointermove="jump(index)"
      >
        {{ option.label }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.select-box {
  position: relative;
  min-width: 0;
}

.select-button {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-8);
  width: 100%;
  padding: var(--sp-8) var(--sp-12);
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  color: var(--text);
  font-size: var(--fs-14);
  text-align: left;
}

.select-button:hover {
  border-color: var(--accent);
}

.select-value {
  flex: 1 1 auto;
}

.select-caret {
  color: var(--text-dim);
  flex: none;
}

.select-list {
  position: absolute;
  top: calc(100% + var(--sp-4));
  left: 0;
  right: 0;
  z-index: 60;
  margin: 0;
  padding: var(--sp-4);
  list-style: none;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  max-height: 280px;
  overflow-y: auto;
  box-shadow: none;
}

.select-option {
  padding: var(--sp-8) var(--sp-12);
  border-radius: var(--radius);
  font-size: var(--fs-14);
  color: var(--text);
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.select-option-active {
  background: var(--bg-raised);
}

.select-option-selected {
  color: var(--accent);
  font-weight: 600;
}
</style>
