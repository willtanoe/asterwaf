<script setup lang="ts">
import { Chart, type ChartDataset } from "chart.js";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { token } from "../charts";
import { formatClock, formatNumber } from "../format";
import { theme } from "../theme";
import type { SeriesPoint } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  series: SeriesPoint[];
  bucket: string;
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"bar" | "line", number[], string> | null = null;

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const accent = token("--accent");
  const blockedColor = token("--blocked");
  const grid = token("--border");
  const text = token("--text-dim");
  const datasets: ChartDataset<"bar" | "line", number[]>[] = [
    {
      // Requests: solid bars per bucket.
      type: "bar",
      label: copy.overview.chart.requests,
      data: props.series.map((point) => point.requests),
      backgroundColor: accent,
      borderWidth: 0,
      maxBarThickness: 24,
      order: 2,
    },
    {
      // Blocked: stepped outline with square markers, never smoothed.
      type: "line",
      label: copy.overview.chart.blocked,
      data: props.series.map((point) => point.blocked),
      borderColor: blockedColor,
      backgroundColor: "transparent",
      borderWidth: 2,
      stepped: true,
      pointStyle: "rect" as const,
      pointRadius: 2.5,
      pointHoverRadius: 3.5,
      pointBackgroundColor: blockedColor,
      tension: 0,
      order: 1,
    },
  ];
  const scaleColor = { ticks: { color: text, precision: 0 }, grid: { color: grid } };
  if (chart !== null) {
    chart.data.labels = props.series.map(() => "");
    chart.data.datasets = datasets;
    chart.options.scales!.x!.ticks!.color = text;
    chart.options.scales!.x!.grid!.color = grid;
    chart.options.scales!.y!.ticks!.color = text;
    chart.options.scales!.y!.grid!.color = grid;
    chart.options.plugins!.legend!.labels!.color = text;
    chart.update("none");
    return;
  }
  chart = new Chart<"bar" | "line", number[], string>(canvas.value, {
    type: "bar",
    data: { labels: props.series.map(() => ""), datasets },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: { labels: { color: text, boxWidth: 12, boxHeight: 8 } },
        tooltip: {
          enabled: true,
          callbacks: {
            title: (items) => {
              const index = items[0]?.dataIndex ?? 0;
              const point = props.series[index];
              return point === undefined ? "" : formatClock(point.ts);
            },
            label: (item) => `${item.dataset.label}: ${formatNumber(item.parsed.y ?? 0)}`,
            footer: (items) => {
              const point = props.series[items[0]?.dataIndex ?? 0];
              if (point === undefined || point.requests === 0) {
                return `${copy.overview.chart.blockRate}: —`;
              }
              const rate = (point.blocked / point.requests) * 100;
              return `${copy.overview.chart.blockRate}: ${rate.toFixed(1)}%`;
            },
          },
          footerColor: text,
        },
      },
      scales: {
        x: {
          ticks: {
            color: text,
            maxRotation: 0,
            autoSkip: true,
            maxTicksLimit: 8,
            // Labels show the bucket start time; the raw ts is only for tooltips.
            callback: (_value, index) => {
              const point = props.series[index as number];
              return point === undefined ? "" : formatClock(point.ts);
            },
          },
          grid: { color: grid },
        },
        y: {
          beginAtZero: true,
          ...scaleColor,
        },
      },
    },
  });
}

onMounted(render);
watch([() => props.series, theme], render, { deep: true });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});
</script>

<template>
  <div class="chart-wrap">
    <canvas ref="canvas" role="img" :aria-label="copy.overview.chart.aria" />
    <p class="sr-only">
      {{ copy.overview.chart.aria }}. Bucket: {{ bucket }}.
    </p>
  </div>
</template>

<style scoped>
.chart-wrap {
  height: 240px;
  min-width: 0;
}
</style>
