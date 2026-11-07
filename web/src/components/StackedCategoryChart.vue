<script setup lang="ts">
import { Chart } from "chart.js";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { token } from "../charts";
import { formatClock, formatNumber } from "../format";
import { theme } from "../theme";
import type { DetectionSeries } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  series: DetectionSeries[];
  from: number;
  to: number;
  bucketMs: number;
  height?: number;
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"bar", number[], number> | null = null;

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

const start = computed(() => (props.from / props.bucketMs) * props.bucketMs);
const buckets = computed(() => Math.max(1, Math.ceil((props.to - start.value) / props.bucketMs)));

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const grid = token("--border");
  const text = token("--text-dim");
  const labels = Array.from({ length: buckets.value }, (_, index) => start.value + index * props.bucketMs);
  const datasets = props.series.map((item) => {
    const map = new Map(item.points.map((point) => [point.ts, point.count]));
    return {
      label: item.key,
      data: labels.map((ts) => map.get(ts) ?? 0),
      backgroundColor: categoryColor(item.key),
      borderWidth: 0,
      maxBarThickness: 24,
    };
  });
  if (chart !== null) {
    chart.data.labels = labels;
    chart.data.datasets = datasets;
    chart.options.scales!.x!.ticks!.color = text;
    chart.options.scales!.x!.grid!.color = grid;
    chart.options.scales!.y!.ticks!.color = text;
    chart.options.scales!.y!.grid!.color = grid;
    chart.update("none");
    return;
  }
  chart = new Chart<"bar", number[], number>(canvas.value, {
    type: "bar",
    data: { labels, datasets },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: { labels: { color: text, boxWidth: 10, boxHeight: 8 } },
        tooltip: {
          enabled: true,
          callbacks: {
            title: (items) => {
              const index = items[0]?.dataIndex ?? 0;
              return formatClock(labels[index]);
            },
            label: (item) => `${item.dataset.label}: ${formatNumber(item.parsed.y ?? 0)}`,
          },
        },
      },
      scales: {
        x: {
          stacked: true,
          ticks: {
            color: text,
            maxRotation: 0,
            autoSkip: true,
            maxTicksLimit: 8,
            callback: (_value, index) => {
              const ts = labels[index as number];
              return ts === undefined ? "" : formatClock(ts);
            },
          },
          grid: { color: grid },
        },
        y: { stacked: true, beginAtZero: true, ticks: { color: text, precision: 0 }, grid: { color: grid } },
      },
    },
  });
}

onMounted(render);
watch([() => props.series, () => props.from, () => props.to, theme], render, { deep: true, flush: "post" });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});
</script>

<template>
  <div class="stacked-chart" :style="{ height: `${height ?? 220}px` }">
    <canvas ref="canvas" role="img" :aria-label="copy.overview.chart.detectionsAria" />
  </div>
</template>

<style scoped>
.stacked-chart {
  min-width: 0;
  position: relative;
}
</style>
