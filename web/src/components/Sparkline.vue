<script setup lang="ts">
import { Chart } from "chart.js";
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { token } from "../charts";

const props = defineProps<{
  values: number[];
  color: string;
  label: string;
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
let chart: Chart<"line"> | null = null;

function render(): void {
  if (canvas.value === null) {
    return;
  }
  const data = props.values.length > 0 ? props.values : [0];
  const color = token(props.color);
  if (chart !== null) {
    chart.data.datasets[0].data = data;
    chart.data.datasets[0].borderColor = color;
    chart.update("none");
    return;
  }
  chart = new Chart(canvas.value, {
    type: "line",
    data: {
      labels: data.map((_, index) => String(index)),
      datasets: [
        {
          data,
          borderColor: color,
          backgroundColor: "transparent",
          borderWidth: 1.5,
          pointRadius: 0,
          tension: 0,
        },
      ],
    },
    options: {
      animation: false,
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          enabled: true,
          displayColors: false,
          callbacks: {
            title: () => props.label,
            label: (item) => String(item.parsed.y),
          },
        },
      },
      scales: {
        x: { display: false },
        y: { display: false },
      },
    },
  });
}

onMounted(render);
watch(() => [props.values, props.color], render, { deep: true });
onBeforeUnmount(() => {
  chart?.destroy();
  chart = null;
});
</script>

<template>
  <canvas ref="canvas" class="sparkline" role="img" :aria-label="label" />
</template>

<style scoped>
.sparkline {
  display: block;
  width: 100%;
  height: 32px;
}
</style>
