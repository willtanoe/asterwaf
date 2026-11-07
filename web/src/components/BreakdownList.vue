<script setup lang="ts">
import { computed } from "vue";
import { copy } from "../copy";
import { actionLabel, formatNumber } from "../format";
import type { CountItem } from "../api/types";
import StateMessage from "./StateMessage.vue";

const props = defineProps<{
  title: string;
  items: CountItem[];
  actionStyle?: boolean;
  loading?: boolean;
  error?: boolean;
  emptyMessage: string;
}>();

const emit = defineEmits<{ retry: [] }>();

const max = computed(() => {
  let value = 0;
  for (const item of props.items) {
    value = Math.max(value, item.count);
  }
  return value > 0 ? value : 1;
});

const total = computed(() => {
  let value = 0;
  for (const item of props.items) {
    value += item.count;
  }
  return value;
});

function share(count: number): string {
  if (total.value === 0) {
    return "0%";
  }
  return `${Math.round((count / total.value) * 100)}%`;
}

function label(key: string): string {
  return props.actionStyle ? actionLabel(key) : key;
}

function barClass(key: string): string {
  if (!props.actionStyle) {
    return "bar-fill bar-accent";
  }
  switch (key) {
    case "block":
      return "bar-fill bar-blocked";
    case "shadow":
      return "bar-fill bar-recorded";
    case "ratelimited":
      return "bar-fill bar-ratelimited";
    default:
      return "bar-fill bar-accent";
  }
}
</script>

<template>
  <div class="breakdown">
    <h4 class="breakdown-title">{{ title }}</h4>
    <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
    <StateMessage v-else-if="error" kind="error" :message="copy.common.errorTitle" @retry="emit('retry')" />
    <StateMessage v-else-if="items.length === 0" kind="empty" :message="emptyMessage" />
    <ul v-else class="bars">
      <li v-for="item in items" :key="item.key" class="bar-row">
        <span class="bar-label wrap" :title="item.key">{{ label(item.key) }}</span>
        <span class="bar-track" aria-hidden="true">
          <span :class="barClass(item.key)" :style="{ width: `${Math.max(2, (item.count / max) * 100)}%` }" />
        </span>
        <span class="bar-value mono" :title="`${formatNumber(item.count)} (${share(item.count)})`">{{ formatNumber(item.count) }} <span class="bar-share">({{ share(item.count) }})</span></span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.breakdown {
  display: flex;
  flex-direction: column;
  gap: var(--sp-8);
  min-width: 0;
}

.breakdown-title {
  margin: 0;
  font-size: var(--fs-12);
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.bars {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
}

.bar-row {
  display: grid;
  grid-template-columns: minmax(80px, 1.4fr) minmax(60px, 2fr) auto;
  align-items: center;
  gap: var(--sp-8);
  font-size: var(--fs-12);
  min-width: 0;
}

.bar-label {
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bar-track {
  height: 8px;
  background: var(--bg-raised);
  border-radius: var(--radius);
  overflow: hidden;
}

.bar-fill {
  display: block;
  height: 100%;
}

.bar-accent {
  background: var(--accent);
}

.bar-blocked {
  background: var(--blocked);
}

.bar-recorded {
  background: var(--recorded);
}

.bar-ratelimited {
  background: var(--ratelimited);
}

.bar-value {
  color: var(--text-dim);
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.bar-share {
  color: var(--text-faint);
}
</style>
