<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Chart } from "chart.js";
import { token } from "../charts";
import { actionLabel, formatNumber } from "../format";
import { theme } from "../theme";
import type { CountItem } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  actions: CountItem[];
  total: number;
  blockRate: string;
  state: "ready" | "loading" | "error";
}>();

const emit = defineEmits<{ retry: [] }>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"doughnut"> | null = null;

interface Segment {
  key: string;
  label: string;
  count: number;
  share: number;
  color: string;
}

const colorFor = (key: string): string => {
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

const listed = ["allow", "shadow", "block", "ratelimited"];

const allSegments = computed<Segment[]>(() =>
  listed.map((key) => {
    const item = props.actions.find((action) => action.key === key);
    const count = item?.count ?? 0;
    return {
      key,
      label: actionLabel(key),
      count,
      share: props.total > 0 ? (count / props.total) * 100 : 0,
      color: colorFor(key),
    };
  }),
);

// The donut drops zero segments so no empty arc renders; the legend keeps
// every entry so the meaning of each color is always spelled out.
const segments = computed<Segment[]>(() => allSegments.value.filter((segment) => segment.count > 0));

const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const text = token("--text-dim");
  const data = segments.value;
  if (chart !== null) {
    chart.data.labels = data.map((segment) => segment.label);
    chart.data.datasets[0].data = data.map((segment) => segment.count);
    chart.data.datasets[0].backgroundColor = data.map((segment) => segment.color);
    chart.data.datasets[0].borderColor = token("--bg-panel");
    chart.options.plugins!.legend!.labels!.color = text;
    // Refreshes never animate; only the initial mount may.
    chart.update("none");
    return;
  }
  chart = new Chart<"doughnut", number[], string>(canvas.value, {
    type: "doughnut",
    data: {
      labels: data.map((segment) => segment.label),
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
      // Respect the OS setting; otherwise animate once on mount only.
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
              return `${segment.label}: ${formatNumber(segment.count)} (${segment.share.toFixed(0)}%)`;
            },
          },
        },
      },
    },
  });
}

onMounted(render);
// flush "post": the canvas only enters the DOM once the loading state
// resolves, so the render must run after the template update.
watch([() => props.actions, () => props.total, theme], render, { deep: true, flush: "post" });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});

function focusSegment(segment: Segment): void {
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
    <div v-else-if="total === 0" class="donut-state">
      <span class="donut-empty-ring" aria-hidden="true" />
      <span class="muted">{{ copy.overview.donut.empty }}</span>
    </div>
    <template v-else>
      <div class="donut-stage">
        <canvas ref="canvas" role="img" :aria-label="`${copy.overview.donut.aria}: ${blockRate}`" />
        <p class="donut-center">
          <span class="donut-total mono">{{ formatNumber(total) }}</span>
          <span class="donut-caption">{{ copy.overview.donut.total }}</span>
          <span class="donut-rate mono">{{ blockRate }} {{ copy.overview.donut.blockRate }}</span>
        </p>
      </div>
      <ul class="donut-legend">
        <li v-for="segment in allSegments" :key="segment.key">
          <button
            type="button"
            class="donut-legend-item"
            @focus="focusSegment(segment)"
            @blur="blurSegment"
          >
            <span class="donut-swatch" :style="{ background: segment.color }" aria-hidden="true" />
            <span class="donut-legend-label">{{ segment.label }}</span>
            <span class="mono donut-legend-value">
              {{ formatNumber(segment.count) }} ({{ segment.share.toFixed(0) }}%)
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
  flex: 1 1 auto;
  min-height: 190px;
}

.donut-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0;
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

.donut-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--sp-4) var(--sp-12);
}

.donut-legend-item {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-4);
  padding: var(--sp-4) var(--sp-8);
  background: transparent;
  border: 0;
  border-radius: var(--radius);
  color: var(--text-dim);
  font-size: var(--fs-12);
  cursor: default;
}

.donut-legend-label {
  color: var(--text-dim);
}

.donut-legend-value {
  font-variant-numeric: tabular-nums;
  color: var(--text-dim);
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
  width: 160px;
  height: 160px;
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
  width: 120px;
  height: 120px;
  border-radius: 50%;
  border: 10px solid var(--border);
}
</style>
