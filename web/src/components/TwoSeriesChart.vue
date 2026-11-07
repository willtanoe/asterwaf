<script setup lang="ts">
import { Chart } from "chart.js";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { token } from "../charts";
import { formatClock } from "../format";
import { theme } from "../theme";

const props = defineProps<{
  points: { ts: number; a: number; b: number }[];
  labelA: string;
  labelB: string;
  colorA: string;
  colorB: string;
  format: (value: number) => string;
  chartLabel: string;
  height?: number;
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"line", number[], number> | null = null;

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const grid = token("--border");
  const text = token("--text-dim");
  const labels = props.points.map((point) => point.ts);
  const datasets = [
    {
      label: props.labelA,
      data: props.points.map((point) => point.a),
      borderColor: props.colorA,
      backgroundColor: "transparent",
      borderWidth: 2,
      stepped: true,
      pointRadius: 0,
      tension: 0,
    },
    {
      label: props.labelB,
      data: props.points.map((point) => point.b),
      borderColor: props.colorB,
      backgroundColor: "transparent",
      borderWidth: 2,
      stepped: true,
      pointRadius: 0,
      tension: 0,
    },
  ];
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
  chart = new Chart<"line", number[], number>(canvas.value, {
    type: "line",
    data: { labels, datasets },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: { labels: { color: text, boxWidth: 10, boxHeight: 2 } },
        tooltip: {
          enabled: true,
          callbacks: {
            title: (items) => {
              const index = items[0]?.dataIndex ?? 0;
              const point = props.points[index];
              return point === undefined ? "" : formatClock(point.ts);
            },
            label: (item) => `${item.dataset.label}: ${props.format(item.parsed.y ?? 0)}`,
          },
        },
      },
      scales: {
        x: {
          ticks: {
            color: text,
            maxRotation: 0,
            autoSkip: true,
            maxTicksLimit: 8,
            callback: (_value, index) => {
              const point = props.points[index as number];
              return point === undefined ? "" : formatClock(point.ts);
            },
          },
          grid: { color: grid },
        },
        y: { beginAtZero: true, ticks: { color: text }, grid: { color: grid } },
      },
    },
  });
}

onMounted(render);
watch([() => props.points, theme], render, { deep: true, flush: "post" });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});
</script>

<template>
  <div class="two-series" :style="{ height: `${height ?? 220}px` }">
    <canvas ref="canvas" role="img" :aria-label="chartLabel" />
  </div>
</template>

<style scoped>
.two-series {
  min-width: 0;
  position: relative;
}
</style>
