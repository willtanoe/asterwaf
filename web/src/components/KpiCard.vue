<script setup lang="ts">
import Sparkline from "./Sparkline.vue";
import { copy } from "../copy";

defineProps<{
  label: string;
  value: string;
  delta: number | null;
  series?: number[];
  color: string;
  context: string;
  state?: "ready" | "loading" | "error";
}>();
</script>

<template>
  <article class="kpi-card" :aria-busy="state === 'loading'">
    <h3 class="kpi-label">{{ label }}</h3>
    <div class="kpi-main">
      <span class="kpi-value">{{ state === "error" ? "—" : state === "loading" ? "—" : value }}</span>
      <Sparkline
        v-if="state !== 'error' && series !== undefined && series.length > 1"
        :values="series"
        :color="color"
        :label="label"
        class="kpi-spark"
      />
    </div>
    <p v-if="state === 'loading'" class="kpi-meta">{{ copy.common.loading }}</p>
    <p v-else-if="state === 'error'" class="kpi-meta kpi-error">{{ copy.common.errorTitle }}</p>
    <p v-else-if="delta !== null" class="kpi-meta">
      <span class="kpi-delta">{{ delta >= 0 ? "+" : "" }}{{ delta.toFixed(1) }}%</span>
      {{ copy.common.vsPrevious }}
    </p>
    <p v-else class="kpi-meta">{{ copy.common.noPrevious }}</p>
    <p class="kpi-context">{{ context }}</p>
  </article>
</template>

<style scoped>
.kpi-card {
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--sp-16);
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
  min-width: 0;
}

.kpi-label {
  margin: 0;
  color: var(--text-dim);
  font-size: var(--fs-12);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.kpi-main {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--sp-8);
  min-width: 0;
}

.kpi-value {
  font-size: var(--fs-24);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.kpi-spark {
  width: 45%;
  max-width: 120px;
  flex-shrink: 0;
}

.kpi-meta {
  margin: 0;
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.kpi-delta {
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.kpi-error {
  color: var(--blocked);
}

.kpi-context {
  margin: 0;
  color: var(--text-faint);
  font-size: var(--fs-12);
  overflow-wrap: anywhere;
}
</style>
