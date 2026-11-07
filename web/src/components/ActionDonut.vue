<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Chart } from "chart.js";
import { token } from "../charts";
import { formatNumber } from "../format";
import { theme } from "../theme";
import type { CountItem } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  /** Detections (non-allowed decisions) grouped by rule category. */
  categories: CountItem[];
  /** Action split for the range, drawn as the thin bar above the legend. */
  actions: CountItem[];
  detections: number;
  blockRate: string;
  state: "ready" | "loading" | "error";
}>();

const emit = defineEmits<{ retry: [] }>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"doughnut"> | null = null;

const categoryColor = (key: string): string => {
  switch (key) {
    case "sqli":
      return token("--accent");
    case "xss":
      return token("--blocked");
    case "traversal":
      return token("--recorded");
    case "rce":
      return token("--ratelimited");
    case "scanner":
      return token("--ok");
    case "accesslist":
      return token("--text-dim");
    case "ratelimit":
      return token("--border-strong");
    default:
      return token("--text-dim");
  }
};

const actionColor = (key: string): string => {
  switch (key) {
    case "block":
      return token("--blocked");
    case "shadow":
      return token("--recorded");
    case "ratelimited":
      return token("--ratelimited");
    default:
      return token("--accent");
  }
};

const segments = computed(() =>
  [...props.categories]
    .sort((left, right) => right.count - left.count)
    .map((item) => ({ ...item, color: categoryColor(item.key) })),
);

const actionSegments = computed(() =>
  ["allow", "shadow", "block", "ratelimited"].map((key) => {
    const item = props.actions.find((action) => action.key === key);
    return { key, count: item?.count ?? 0, color: actionColor(key) };
  }),
);

const actionTotal = computed(() => actionSegments.value.reduce((sum, item) => sum + item.count, 0));

const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const data = segments.value;
  if (chart !== null) {
    chart.data.labels = data.map((segment) => segment.key);
    chart.data.datasets[0].data = data.map((segment) => segment.count);
    chart.data.datasets[0].backgroundColor = data.map((segment) => segment.color);
    chart.update("none");
    return;
  }
  chart = new Chart<"doughnut", number[], string>(canvas.value, {
    type: "doughnut",
    data: {
      labels: data.map((segment) => segment.key),
      datasets: [
        {
          data: data.map((segment) => segment.count),
          backgroundColor: data.map((segment) => segment.color),
          borderColor: token("--bg-panel"),
          borderWidth: 2,
        },
      ],
    },
    options: {
      cutout: "72%",
      animation: reducedMotion ? false : { duration: 400 },
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          enabled: true,
          callbacks: {
            label: (item) => {
              const segment = data[item.dataIndex];
              const share = props.detections > 0 ? (segment.count / props.detections) * 100 : 0;
              return `${segment.key}: ${formatNumber(segment.count)} (${share.toFixed(0)}%)`;
            },
          },
        },
      },
    },
  });
}

onMounted(render);
// flush "post": the canvas only enters the DOM once the loading state resolves.
watch([() => props.categories, () => props.detections, theme], render, { deep: true, flush: "post" });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});

function focusSegment(segment: { key: string; count: number }): void {
  const index = segments.value.findIndex((item) => item.key === segment.key);
  if (chart === null || index < 0) {
    return;
  }
  chart.setActiveElements([{ datasetIndex: 0, index }]);
  chart.tooltip?.setActiveElements([{ datasetIndex: 0, index }], { x: 0, y: 0 });
  chart.update("none");
}

function blurSegment(): void {
  chart?.setActiveElements([]);
  chart?.tooltip?.setActiveElements([], { x: 0, y: 0 });
  chart?.update("none");
}
</script>

<template>
  <div class="donut">
    <div v-if="state === 'loading'" class="donut-skeleton" aria-hidden="true">
      <span class="skeleton donut-skeleton-ring" />
    </div>
    <div v-else-if="state === 'error'" class="donut-state">
      <span>{{ copy.common.errorTitle }}</span>
      <button type="button" class="btn btn-small" @click="emit('retry')">{{ copy.common.retry }}</button>
    </div>
    <div v-else-if="detections === 0" class="donut-state">
      <span class="donut-empty-ring" aria-hidden="true" />
      <span class="muted">{{ copy.overview.donut.empty }}</span>
    </div>
    <template v-else>
      <div class="donut-stage">
        <canvas ref="canvas" role="img" :aria-label="`${copy.overview.donut.aria}: ${formatNumber(detections)}`" />
        <p class="donut-center">
          <span class="donut-total mono">{{ formatNumber(detections) }}</span>
          <span class="donut-caption">{{ copy.overview.donut.detections }}</span>
          <span class="donut-rate mono">{{ blockRate }}</span>
          <span class="donut-caption">{{ copy.overview.donut.blockRate }}</span>
        </p>
      </div>
      <div class="donut-bar" role="img" :aria-label="copy.overview.donut.actionBar">
        <span
          v-for="segment in actionSegments"
          :key="segment.key"
          class="donut-bar-segment"
          :style="{ background: segment.color, width: `${actionTotal > 0 ? (segment.count / actionTotal) * 100 : 0}%` }"
        />
      </div>
      <ul class="donut-legend">
        <li v-for="segment in segments" :key="segment.key">
          <button
            type="button"
            class="donut-legend-item"
            @focus="focusSegment(segment)"
            @blur="blurSegment"
          >
            <span class="donut-swatch" :style="{ background: segment.color }" aria-hidden="true" />
            <span class="donut-legend-label">{{ segment.key }}</span>
            <span class="mono donut-legend-value">
              {{ formatNumber(segment.count) }} ({{ props.detections > 0 ? ((segment.count / props.detections) * 100).toFixed(0) : 0 }}%)
            </span>
          </button>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.donut {
  display: flex;
  flex-direction: column;
  gap: var(--sp-8);
  min-width: 0;
  height: 100%;
}

.donut-stage {
  position: relative;
  flex: none;
  height: 150px;
}

.donut-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin: 0;
  pointer-events: none;
}

.donut-total {
  font-size: var(--fs-24);
  font-weight: 600;
}

.donut-caption {
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.donut-rate {
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.donut-bar {
  display: flex;
  height: 8px;
  border-radius: var(--radius);
  overflow: hidden;
  background: var(--bg-raised);
}

.donut-bar-segment {
  display: block;
  height: 100%;
}

.donut-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--sp-4) var(--sp-8);
}

.donut-legend-entry,
.donut-legend-item {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-4);
  font-size: var(--fs-12);
  color: var(--text-dim);
}

.donut-legend-item {
  padding: var(--sp-4) var(--sp-8);
  background: transparent;
  border: 0;
  border-radius: var(--radius);
  cursor: default;
}

.donut-legend-value {
  font-variant-numeric: tabular-nums;
}

.donut-swatch {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex: none;
}

.donut-skeleton {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
}

.donut-skeleton-ring {
  width: 130px;
  height: 130px;
  border-radius: 50%;
}

.donut-state {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--sp-8);
  color: var(--text-dim);
  font-size: var(--fs-13);
}

.donut-empty-ring {
  width: 100px;
  height: 100px;
  border-radius: 50%;
  border: 10px solid var(--border);
}
</style>
