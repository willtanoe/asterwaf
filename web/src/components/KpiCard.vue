<script setup lang="ts">
import { computed } from "vue";
import Sparkline from "./Sparkline.vue";
import { copy } from "../copy";

const props = defineProps<{
  label: string;
  value: string;
  delta: number | null;
  series?: number[];
  color: string;
  /** Tooltip on the label; used for definitions instead of permanent body text. */
  note?: string;
  /** Small flag rendered next to the label, e.g. "approximate". */
  flag?: string;
  /** Secondary figure shown under the value, e.g. Recorded on the Blocked card. */
  secondary?: { label: string; value: string };
  /** In-place link instead of the delta line, e.g. the Rate limited card when off. */
  link?: { to: string; label: string };
  /** Previous window counted 0 while the current window has traffic. */
  isNew?: boolean;
  state?: "ready" | "loading" | "error";
}>();

// The formatted value may carry a unit; strip everything after the first space
// so "new" detection works on the raw number.
const numericValue = computed(() => Number.parseFloat(props.value));
const isNewValue = computed(() => props.isNew === true && Number.isFinite(numericValue.value) && numericValue.value > 0);
</script>

<template>
  <article class="kpi-card" :aria-busy="state === 'loading'">
    <h3 class="kpi-label">
      <span class="kpi-label-text" :title="note">{{ label }}</span>
      <span v-if="flag !== undefined" class="kpi-flag">{{ flag }}</span>
    </h3>
    <template v-if="state === 'loading'">
      <span class="skeleton kpi-skeleton-value" aria-hidden="true" />
      <span class="skeleton kpi-skeleton-meta" aria-hidden="true" />
    </template>
    <template v-else>
      <div class="kpi-main">
        <span class="kpi-value kpi-nowrap">{{ state === "error" ? "—" : value }}</span>
        <Sparkline
          v-if="state !== 'error' && series !== undefined && series.length > 1"
          :values="series"
          :color="color"
          :label="label"
          class="kpi-spark"
        />
      </div>
      <p v-if="state === 'error'" class="kpi-meta kpi-error">{{ copy.common.errorTitle }}</p>
      <template v-else-if="link !== undefined">
        <p class="kpi-meta">
          <RouterLink :to="link.to">{{ link.label }}</RouterLink>
        </p>
        <p v-if="delta !== null" class="kpi-meta">
          <span class="kpi-delta">{{ delta >= 0 ? "+" : "−" }}{{ Math.abs(delta).toFixed(1) }}%</span>
          {{ copy.common.vsPrevious }}
        </p>
      </template>
      <p v-else-if="isNewValue" class="kpi-meta">
        <span class="kpi-delta">{{ copy.overview.deltaNew }}</span>
        {{ copy.common.vsPrevious }}
      </p>
      <p v-else-if="delta !== null" class="kpi-meta">
        <span class="kpi-delta" :aria-label="`${delta >= 0 ? copy.overview.deltaUp : copy.overview.deltaDown} ${Math.abs(delta).toFixed(1)}%`">
          {{ delta >= 0 ? "+" : "−" }}{{ Math.abs(delta).toFixed(1) }}%
        </span>
        {{ copy.common.vsPrevious }}
      </p>
      <p v-else class="kpi-meta">{{ copy.overview.deltaNA }}</p>
      <p v-if="secondary !== undefined && state !== 'error'" class="kpi-secondary">
        {{ secondary.label }} <span class="mono">{{ secondary.value }}</span>
      </p>
    </template>
  </article>
</template>

<style scoped>
.kpi-card {
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--sp-12) var(--sp-16);
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
  display: flex;
  align-items: center;
  gap: var(--sp-8);
  min-width: 0;
}

.kpi-label-text {
  cursor: help;
  overflow-wrap: anywhere;
}

.kpi-flag {
  flex: none;
  color: var(--recorded);
  font-weight: 400;
  text-transform: none;
  letter-spacing: 0;
}

.kpi-skeleton-value {
  height: 24px;
  width: 64px;
}

.kpi-skeleton-meta {
  height: 12px;
  width: 110px;
}

.kpi-main {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--sp-8);
  min-width: 0;
}

.kpi-value {
  font-size: var(--fs-20);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.kpi-nowrap {
  white-space: nowrap;
}

.kpi-meta .kpi-delta {
  white-space: nowrap;
}

.kpi-spark {
  width: 45%;
  max-width: 96px;
  flex-shrink: 0;
}

.kpi-meta {
  margin: 0;
  color: var(--text-dim);
  font-size: var(--fs-12);
  font-variant-numeric: tabular-nums;
}

.kpi-delta {
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

.kpi-error {
  color: var(--blocked);
}

.kpi-secondary {
  margin: 0;
  color: var(--text-dim);
  font-size: var(--fs-12);
}
</style>
