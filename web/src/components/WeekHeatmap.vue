<script setup lang="ts">
import { computed } from "vue";
import { formatNumber } from "../format";
import type { HeatmapPoint } from "../api/types";
import { copy } from "../copy";

const props = defineProps<{
  points: HeatmapPoint[];
}>();

// 行序固定为周一..周日（strftime %w 以周日为 0）。
const DOW_KEYS = ["1", "2", "3", "4", "5", "6", "0"];
const DOW_LABELS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

const lookup = computed(() => {
  const map = new Map<string, number>();
  let max = 0;
  for (const point of props.points) {
    map.set(`${point.dow}-${point.hour}`, point.count);
    if (point.count > max) {
      max = point.count;
    }
  }
  return { map, max };
});

const hours = Array.from({ length: 24 }, (_, index) => index);

function cell(dow: string, hour: number): number {
  return lookup.value.map.get(`${dow}-${hour}`) ?? 0;
}

function shade(count: number): string {
  if (count === 0 || lookup.value.max === 0) {
    return "var(--bg-raised)";
  }
  // 透明度由计数决定，颜色只来自 accent token。
  return `color-mix(in srgb, var(--accent) ${Math.max(20, Math.round((count / lookup.value.max) * 100))}%, var(--bg-raised))`;
}
</script>

<template>
  <div class="heatmap" role="img" :aria-label="copy.overview.heatmap.aria">
    <div class="heatmap-grid">
      <span class="heatmap-label" />
      <span v-for="hour in hours" :key="hour" class="heatmap-hour mono">{{ String(hour).padStart(2, "0") }}</span>
      <template v-for="(dow, index) in DOW_KEYS" :key="dow">
        <span class="heatmap-label">{{ DOW_LABELS[index] }}</span>
        <span
          v-for="hour in hours"
          :key="hour"
          class="heatmap-cell"
          :title="`${DOW_LABELS[index]} ${String(hour).padStart(2, '0')}:00 — ${formatNumber(cell(dow, hour))}`"
          :style="{ background: shade(cell(dow, hour)) }"
        />
      </template>
    </div>
    <div class="heatmap-legend">
      <span class="muted">{{ copy.overview.heatmap.low }}</span>
      <span
        v-for="step in 5"
        :key="step"
        class="heatmap-cell"
        :style="{ background: shade(Math.round((lookup.max * step) / 5)) }"
      />
      <span class="muted">{{ copy.overview.heatmap.high }} ({{ formatNumber(lookup.max) }})</span>
    </div>
  </div>
</template>

<style scoped>
.heatmap {
  display: flex;
  flex-direction: column;
  gap: var(--sp-8);
  min-width: 0;
}

.heatmap-grid {
  display: grid;
  grid-template-columns: 36px repeat(24, minmax(0, 1fr));
  gap: 2px;
  overflow-x: auto;
}

.heatmap-label {
  color: var(--text-faint);
  font-size: var(--fs-10);
  align-self: center;
}

.heatmap-hour {
  color: var(--text-faint);
  font-size: var(--fs-10);
  text-align: center;
}

.heatmap-cell {
  height: 20px;
  border-radius: 2px;
  min-width: 12px;
}

.heatmap-legend {
  display: flex;
  align-items: center;
  gap: var(--sp-4);
  font-size: var(--fs-12);
}

.heatmap-legend .heatmap-cell {
  width: 14px;
  height: 14px;
  aspect-ratio: auto;
}
</style>
