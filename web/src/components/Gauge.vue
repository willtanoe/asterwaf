<script setup lang="ts">
import { computed } from "vue";

// Speedometer sweep in degrees: 0 percent sits at 150°, 100 percent at 390°,
// so the needle travels clockwise across the top of the dial.
const START = 150;
const SWEEP = 240;
const CX = 120;
const CY = 112;
const RADIUS = 84;

const props = defineProps<{
  // 0-100 scale; null renders an empty dial with an em-dash readout.
  value: number | null;
  display: string;
  label: string;
}>();

function polar(angleDeg: number, radius: number): { x: number; y: number } {
  const rad = (angleDeg * Math.PI) / 180;
  return { x: CX + radius * Math.cos(rad), y: CY + radius * Math.sin(rad) };
}

const dialPath = computed(() => {
  const from = polar(START, RADIUS);
  const to = polar(START + SWEEP, RADIUS);
  return `M ${from.x.toFixed(2)} ${from.y.toFixed(2)} A ${RADIUS} ${RADIUS} 0 1 1 ${to.x.toFixed(2)} ${to.y.toFixed(2)}`;
});

const percent = computed(() => {
  if (props.value === null || Number.isNaN(props.value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, props.value));
});

const needleAngle = computed(() => START + (percent.value / 100) * SWEEP);

const needleTip = polar(START, 64);

type Tick = {
  major: boolean;
  outer: { x: number; y: number };
  inner: { x: number; y: number };
};

const ticks = computed<Tick[]>(() => {
  const list: Tick[] = [];
  for (let unit = 0; unit <= 100; unit += 5) {
    const major = unit % 25 === 0;
    const angle = START + (unit / 100) * SWEEP;
    list.push({ major, outer: polar(angle, 74), inner: polar(angle, major ? 64 : 69) });
  }
  return list;
});

const scaleLabels = computed(() => {
  return [0, 25, 50, 75, 100].map((unit) => {
    const angle = START + (unit / 100) * SWEEP;
    const point = polar(angle, 53);
    return { unit, x: point.x, y: point.y + 3 };
  });
});
</script>

<template>
  <svg class="gauge" viewBox="0 0 240 180" role="img" :aria-label="`${label}: ${display}`">
    <path class="gauge-track" :d="dialPath" />
    <path
      class="gauge-progress"
      :d="dialPath"
      pathLength="100"
      :style="{ strokeDasharray: `${percent} 100` }"
    />
    <g>
      <line
        v-for="(tick, index) in ticks"
        :key="index"
        :class="tick.major ? 'gauge-tick gauge-tick-major' : 'gauge-tick'"
        :x1="tick.outer.x"
        :y1="tick.outer.y"
        :x2="tick.inner.x"
        :y2="tick.inner.y"
      />
    </g>
    <g>
      <text
        v-for="item in scaleLabels"
        :key="item.unit"
        class="gauge-scale"
        :x="item.x"
        :y="item.y"
        text-anchor="middle"
      >
        {{ item.unit }}
      </text>
    </g>
    <g class="gauge-needle" :style="{ transform: `rotate(${needleAngle - START}deg)` }">
      <line :x1="CX" :y1="CY" :x2="needleTip.x" :y2="needleTip.y" />
    </g>
    <circle class="gauge-hub" :cx="CX" :cy="CY" r="7" />
    <text class="gauge-value" :x="CX" :y="156" text-anchor="middle">{{ display }}</text>
    <text class="gauge-caption" :x="CX" :y="172" text-anchor="middle">{{ label }}</text>
  </svg>
</template>

<style scoped>
.gauge {
  width: 100%;
  max-width: 280px;
  height: auto;
  display: block;
}

.gauge-track {
  fill: none;
  stroke: var(--border);
  stroke-width: 10;
  stroke-linecap: round;
}

.gauge-progress {
  fill: none;
  stroke: var(--accent);
  stroke-width: 10;
  stroke-linecap: round;
  transition: stroke-dasharray 0.6s ease;
}

.gauge-tick {
  stroke: var(--border-strong);
  stroke-width: 1.5;
}

.gauge-tick-major {
  stroke: var(--text-faint);
  stroke-width: 2;
}

.gauge-scale {
  fill: var(--text-faint);
  font-size: var(--fs-10);
  font-family: var(--font-mono);
}

.gauge-needle {
  transition: transform 0.6s ease;
}

.gauge-needle line {
  stroke: var(--text);
  stroke-width: 3;
  stroke-linecap: round;
}

.gauge-hub {
  fill: var(--bg-raised);
  stroke: var(--accent);
  stroke-width: 2;
}

.gauge-value {
  fill: var(--text);
  font-size: var(--fs-24);
  font-weight: 600;
  font-family: var(--font-mono);
}

.gauge-caption {
  fill: var(--text-dim);
  font-size: var(--fs-12);
}

@media (prefers-reduced-motion: reduce) {
  .gauge-progress,
  .gauge-needle {
    transition: none;
  }
}
</style>
