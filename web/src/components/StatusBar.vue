<script setup lang="ts">
import { computed, ref } from "vue";
import { copy } from "../copy";
import { formatNumber, formatUptime, siteStatusLabel } from "../format";
import type { SiteHealth, StatusResponse } from "../api/types";

const props = defineProps<{
  status: StatusResponse | null;
  loading: boolean;
  error: boolean;
}>();

const expanded = ref(false);

const healthyCount = computed(() => {
  if (props.status === null) {
    return 0;
  }
  return props.status.sites.filter((site) => site.status === "up").length;
});

const totalCount = computed(() => props.status?.sites.length ?? 0);

const allHealthy = computed(
  () => props.status !== null && totalCount.value > 0 && healthyCount.value === totalCount.value,
);

/** Down and unknown sites are always visible; healthy ones collapse. */
const problemSites = computed<SiteHealth[]>(() => {
  if (props.status === null) {
    return [];
  }
  return props.status.sites.filter((site) => site.status !== "up");
});

const summaryText = computed(
  () => `${formatNumber(healthyCount.value)}/${formatNumber(totalCount.value)} ${copy.status.healthy}`,
);
</script>

<template>
  <section class="status-bar" aria-label="Runtime status">
    <div v-if="loading && status === null" class="status-skeleton">
      <span class="skeleton status-skeleton-item" aria-hidden="true" />
      <span class="skeleton status-skeleton-item" aria-hidden="true" />
    </div>
    <div v-else-if="error && status === null" class="status-error">
      {{ copy.common.errorTitle }} {{ copy.common.errorHint }}
    </div>
    <template v-else-if="status !== null">
      <span class="status-item">
        <span class="status-label">{{ copy.status.mode }}</span>
        <span class="badge badge-allow">{{ status.mode }}</span>
      </span>
      <span class="status-item">
        <span class="status-label">{{ copy.status.ruleset }}</span>
        <span class="mono">v{{ status.ruleset_version }}</span>
      </span>
      <span class="status-item">
        <span class="status-label">{{ copy.status.uptime }}</span>
        <span class="mono">{{ formatUptime(status.uptime_seconds) }}</span>
      </span>
      <span v-if="status.dropped_logs > 0" class="status-item">
        <span class="status-label">{{ copy.status.dropped }}</span>
        <span class="status-dropped mono" :title="copy.status.droppedHint">
          {{ formatNumber(status.dropped_logs) }}
        </span>
      </span>

      <span class="status-item status-sites">
        <span class="status-label">{{ copy.status.sites }}</span>
        <span class="mono status-summary">{{ summaryText }}</span>
        <button
          v-if="allHealthy"
          type="button"
          class="status-toggle"
          :aria-expanded="expanded"
          @click="expanded = !expanded"
        >
          {{ expanded ? copy.status.hideSites : copy.status.showSites }}
        </button>
        <template v-else>
          <span v-for="site in problemSites" :key="site.host" class="site-health site-problem">
            <span class="dot" :class="site.status === 'down' ? 'dot-down' : 'dot-unknown'" aria-hidden="true" />
            <span class="site-host">{{ site.host }}</span>
            <span class="status-downtext">{{ siteStatusLabel(site.status) }}</span>
            <span class="mono faint">{{ site.latency_ms }} ms</span>
          </span>
        </template>
      </span>

      <span v-if="expanded && allHealthy" class="status-site-list">
        <span v-for="site in status.sites" :key="site.host" class="site-health">
          <span class="dot dot-ok" aria-hidden="true" />
          <span class="site-host">{{ site.host }}</span>
          <span class="mono faint">{{ site.latency_ms }} ms</span>
        </span>
      </span>
    </template>
  </section>
</template>

<style scoped>
.status-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--sp-8) var(--sp-24);
  padding: var(--sp-8) var(--sp-12);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-size: var(--fs-12);
  min-width: 0;
}

.status-skeleton {
  display: flex;
  gap: var(--sp-12);
}

.status-skeleton-item {
  height: 16px;
  width: 220px;
}

.status-error {
  color: var(--blocked);
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-8);
  min-width: 0;
}

.status-label {
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.status-summary {
  color: var(--text-dim);
  font-variant-numeric: tabular-nums;
}

.status-toggle {
  padding: 0 var(--sp-8);
  min-height: 22px;
  background: transparent;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.status-toggle:hover {
  border-color: var(--accent);
  color: var(--text);
}

.status-dropped {
  color: var(--blocked);
  font-variant-numeric: tabular-nums;
}

.status-downtext {
  color: var(--blocked);
}

.status-sites {
  flex-wrap: wrap;
}

.site-health {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-4);
  min-width: 0;
}

.site-host {
  color: var(--text-dim);
}

.site-problem .site-host {
  color: var(--text);
}

.status-site-list {
  flex-basis: 100%;
  display: flex;
  flex-wrap: wrap;
  gap: var(--sp-8) var(--sp-16);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  flex: none;
}

.dot-ok {
  background: var(--ok);
}

.dot-down {
  background: var(--blocked);
}

.dot-unknown {
  background: var(--text-faint);
}
</style>
