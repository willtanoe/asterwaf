<script setup lang="ts">
import { copy } from "../copy";
import { formatNumber, formatUptime, siteStatusLabel } from "../format";
import type { StatusResponse } from "../api/types";

defineProps<{
  status: StatusResponse | null;
  loading: boolean;
  error: boolean;
}>();
</script>

<template>
  <section class="status-bar" aria-label="Runtime status">
    <div v-if="loading && status === null" class="state">{{ copy.common.loading }}</div>
    <div v-else-if="error && status === null" class="state state-error">
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
      <span class="status-item status-sites">
        <span class="status-label">{{ copy.status.sites }}</span>
        <span v-for="site in status.sites" :key="site.host" class="site-health">
          <span
            class="dot"
            :class="site.status === 'up' ? 'dot-ok' : site.status === 'down' ? 'dot-down' : 'dot-unknown'"
            aria-hidden="true"
          />
          <span class="wrap">{{ site.host }}</span>
          <span class="badge" :class="site.status === 'down' ? 'badge-down' : site.status === 'up' ? 'badge-ok' : 'badge-unknown'">
            {{ siteStatusLabel(site.status) }}
          </span>
          <span class="mono faint">{{ site.latency_ms }} ms</span>
        </span>
      </span>
      <span v-if="status.dropped_logs > 0" class="status-item">
        <span class="status-label">{{ copy.status.dropped }}</span>
        <span class="badge badge-block" :title="copy.status.droppedHint">
          {{ formatNumber(status.dropped_logs) }}
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
  gap: var(--sp-12) var(--sp-24);
  padding: var(--sp-8) var(--sp-12);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-size: var(--fs-12);
  min-width: 0;
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

.status-sites {
  flex-wrap: wrap;
}

.site-health {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-4);
  padding: var(--sp-4) var(--sp-8);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
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
