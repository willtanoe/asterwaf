<script setup lang="ts">
import { Chart } from "chart.js";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { token } from "../charts";
import { formatDayTime } from "../format";
import { theme } from "../theme";
import type { SeriesPoint } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  series: SeriesPoint[];
  bucket: string;
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"line"> | null = null;

function labels(): string[] {
  return props.series.map((point) => formatDayTime(point.ts));
}

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const requests = props.series.map((point) => point.requests);
  const blocked = props.series.map((point) => point.blocked);
  const accent = token("--accent");
  const blockedColor = token("--blocked");
  const grid = token("--border");
  const text = token("--text-dim");
  if (chart !== null) {
    chart.data.labels = labels();
    chart.data.datasets[0].data = requests;
    chart.data.datasets[1].data = blocked;
    chart.data.datasets[0].borderColor = accent;
    chart.data.datasets[1].borderColor = blockedColor;
    chart.options.scales!.x!.ticks!.color = text;
    chart.options.scales!.y!.ticks!.color = text;
    chart.options.scales!.x!.grid!.color = grid;
    chart.options.scales!.y!.grid!.color = grid;
    chart.update("none");
    return;
  }
  chart = new Chart(canvas.value, {
    type: "line",
    data: {
      labels: labels(),
      datasets: [
        {
          label: copy.overview.chart.requests,
          data: requests,
          borderColor: accent,
          backgroundColor: "transparent",
          borderWidth: 2,
          pointRadius: 0,
          tension: 0.2,
        },
        {
          label: copy.overview.chart.blocked,
          data: blocked,
          borderColor: blockedColor,
          backgroundColor: "transparent",
          borderWidth: 2,
          pointRadius: 0,
          tension: 0.2,
        },
      ],
    },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: {
          labels: { color: text, boxWidth: 12, boxHeight: 2 },
        },
        tooltip: { enabled: true },
      },
      scales: {
        x: {
          ticks: { color: text, maxRotation: 0, autoSkip: true, maxTicksLimit: 8 },
          grid: { color: grid },
        },
        y: {
          beginAtZero: true,
          ticks: { color: text, precision: 0 },
          grid: { color: grid },
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
  height: 260px;
  min-width: 0;
}
</style>
