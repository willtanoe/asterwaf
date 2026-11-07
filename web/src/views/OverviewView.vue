<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { api } from "../api/client";
import type { LogEntry, OverviewResponse, SiteView, StatusResponse } from "../api/types";
import BreakdownList from "../components/BreakdownList.vue";
import Gauge from "../components/Gauge.vue";
import KpiCard from "../components/KpiCard.vue";
import Select from "../components/Select.vue";
import StateMessage from "../components/StateMessage.vue";
import StatusBar from "../components/StatusBar.vue";
import TrafficChart from "../components/TrafficChart.vue";
import { copy } from "../copy";
import {
  actionClass,
  actionLabel,
  formatClock,
  formatDayTime,
  formatLatency,
  formatNumber,
  formatPercent,
  formatRangeTime,
} from "../format";

const ranges = ["15m", "1h", "24h", "7d"] as const;
type RangeKey = (typeof ranges)[number];
const range = ref<RangeKey>("15m");
const site = ref("");

const sites = ref<SiteView[]>([]);
const siteOptions = computed(() => [
  { value: "", label: copy.common.allSites },
  ...sites.value.map((item) => ({ value: item.host, label: item.host })),
]);
const overview = ref<OverviewResponse | null>(null);
const overviewLoading = ref(true);
const overviewError = ref(false);
const status = ref<StatusResponse | null>(null);
const statusLoading = ref(true);
const statusError = ref(false);
const latest = ref<LogEntry[]>([]);
const latestLoading = ref(true);
const latestError = ref(false);
const updatedAt = ref<number | null>(null);

let overviewTimer = 0;
let statusTimer = 0;
let latestTimer = 0;

const kpiDefs = [
  { key: "unique_ips", color: "--accent", format: formatNumber },
  { key: "latency_p50_ms", color: "--text-dim", format: formatLatency },
  { key: "latency_p95_ms", color: "--text-dim", format: formatLatency },
] as const;

const heroCounters = computed(() => [
  { key: "total", tone: "total", label: copy.overview.kpis.total_requests, value: kpiValue("total_requests", formatNumber) },
  { key: "blocked", tone: "blocked", label: copy.overview.kpis.blocked_requests, value: kpiValue("blocked_requests", formatNumber) },
  { key: "shadow", tone: "shadow", label: copy.overview.kpis.shadow_requests, value: kpiValue("shadow_requests", formatNumber) },
  { key: "ratelimited", tone: "ratelimited", label: copy.overview.kpis.ratelimited_requests, value: kpiValue("ratelimited_requests", formatNumber) },
]);

const blockRateValue = computed<number | null>(() => overview.value?.kpis.block_rate?.value ?? null);

const sectionState = computed<"ready" | "loading" | "error">(() => {
  if (overview.value !== null) {
    return "ready";
  }
  if (overviewError.value) {
    return "error";
  }
  return "loading";
});

const seriesData = computed(() => overview.value?.series ?? []);
const rangeInfo = computed(() => overview.value?.range ?? null);
const topIps = computed(() => overview.value?.tables.top_ips ?? []);
const topPaths = computed(() => overview.value?.tables.top_paths ?? []);

const chartEmpty = computed(() => {
  const data = seriesData.value;
  return data.length === 0 || data.every((point) => point.requests === 0);
});

const kpiValue = (key: string, format: (value: number) => string): string => {
  const value = overview.value?.kpis[key];
  return value === undefined ? "—" : format(value.value);
};

const kpiDelta = (key: string): number | null => overview.value?.kpis[key]?.delta_pct ?? null;

const kpiSeries = (key: string): number[] | undefined => overview.value?.kpis[key]?.series;

const kpiContext = (key: string): string => {
  if (key === "unique_ips") {
    return overview.value?.range.approximate ? copy.overview.kpiContext.uniqueApprox : copy.overview.kpiContext.uniqueRaw;
  }
  if (key === "latency_p50_ms" || key === "latency_p95_ms") {
    return copy.overview.kpiContext.latency;
  }
  if (key === "block_rate") {
    return copy.overview.kpiContext.blockRate;
  }
  return "";
};

async function loadOverview(): Promise<void> {
  overviewLoading.value = overview.value === null;
  try {
    overview.value = await api.overview(range.value, site.value);
    updatedAt.value = Date.now();
    overviewError.value = false;
  } catch {
    overviewError.value = true;
  } finally {
    overviewLoading.value = false;
  }
}

async function loadStatus(): Promise<void> {
  statusLoading.value = status.value === null;
  try {
    status.value = await api.status();
    statusError.value = false;
  } catch {
    statusError.value = true;
  } finally {
    statusLoading.value = false;
  }
}

async function loadLatest(): Promise<void> {
  latestLoading.value = latest.value.length === 0;
  try {
    const query = new URLSearchParams({ action: "block", limit: "10" });
    if (site.value !== "") {
      query.set("site", site.value);
    }
    const response = await api.logs(query.toString());
    latest.value = response.items;
    latestError.value = false;
  } catch {
    latestError.value = true;
  } finally {
    latestLoading.value = false;
  }
}

async function loadSites(): Promise<void> {
  try {
    const response = await api.sites();
    sites.value = response.sites;
  } catch {
    sites.value = [];
  }
}

function refreshOverview(): void {
  if (!document.hidden) {
    void loadOverview();
    void loadLatest();
  }
}

function onVisibility(): void {
  if (!document.hidden) {
    refreshOverview();
    void loadStatus();
  }
}

onMounted(() => {
  void loadSites();
  void loadOverview();
  void loadStatus();
  void loadLatest();
  overviewTimer = window.setInterval(() => {
    if (!document.hidden) {
      void loadOverview();
    }
  }, 30000);
  latestTimer = window.setInterval(() => {
    if (!document.hidden) {
      void loadLatest();
    }
  }, 15000);
  statusTimer = window.setInterval(() => {
    if (!document.hidden) {
      void loadStatus();
    }
  }, 10000);
  document.addEventListener("visibilitychange", onVisibility);
});

onBeforeUnmount(() => {
  window.clearInterval(overviewTimer);
  window.clearInterval(latestTimer);
  window.clearInterval(statusTimer);
  document.removeEventListener("visibilitychange", onVisibility);
});

watch(site, () => {
  void loadOverview();
  void loadLatest();
});

watch(range, () => {
  void loadOverview();
});
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.overview.title }}</h1>
        <p class="page-subtitle">{{ copy.overview.subtitle }}</p>
      </div>
      <div class="toolbar">
        <div class="row">
          <span class="muted">{{ copy.common.range }}</span>
          <div class="segmented" role="group" :aria-label="copy.common.range">
            <button
              v-for="item in ranges"
              :key="item"
              type="button"
              :aria-pressed="range === item"
              @click="range = item"
            >
              {{ item }}
            </button>
          </div>
        </div>
        <div class="row">
          <span class="muted">{{ copy.common.site }}</span>
          <Select
            class="site-select"
            :model-value="site"
            :options="siteOptions"
            :label="copy.common.site"
            @update:model-value="site = $event"
          />
        </div>
        <span v-if="updatedAt !== null" class="faint mono">
          {{ copy.common.updated }} {{ formatClock(updatedAt) }}
        </span>
      </div>
    </header>

    <StatusBar :status="status" :loading="statusLoading" :error="statusError" />

    <p v-if="overviewError && overview !== null" class="stale" role="status">{{ copy.common.stale }}</p>

    <section class="panel" aria-label="Block rate">
      <div class="panel-header">
        <h2 class="panel-title">{{ copy.overview.kpis.block_rate }}</h2>
        <span v-if="overview?.range.approximate" class="badge badge-unknown">{{ copy.overview.approximate }}</span>
      </div>
      <div class="panel-body hero-body">
        <StateMessage
          v-if="sectionState === 'loading'"
          kind="loading"
          :message="copy.common.loading"
        />
        <StateMessage
          v-else-if="sectionState === 'error'"
          kind="error"
          :message="copy.common.errorTitle"
          @retry="loadOverview"
        />
        <template v-else>
          <Gauge
            class="hero-gauge"
            :value="blockRateValue"
            :display="kpiValue('block_rate', formatPercent)"
            :label="copy.overview.kpis.block_rate"
          />
          <dl class="hero-counters" :aria-label="copy.overview.gauge.counters">
            <div v-for="counter in heroCounters" :key="counter.key" class="counter" :class="`counter-${counter.tone}`">
              <dt>{{ counter.label }}</dt>
              <dd class="mono">{{ counter.value }}</dd>
            </div>
          </dl>
        </template>
      </div>
    </section>

    <section class="grid-kpi" aria-label="Key metrics">
      <KpiCard
        v-for="definition in kpiDefs"
        :key="definition.key"
        :label="copy.overview.kpis[definition.key]"
        :value="kpiValue(definition.key, definition.format)"
        :delta="kpiDelta(definition.key)"
        :series="kpiSeries(definition.key)"
        :color="definition.color"
        :context="kpiContext(definition.key)"
        :state="sectionState"
      />
    </section>

    <section class="grid-2">
      <div class="panel">
        <div class="panel-header">
          <h2 class="panel-title">{{ copy.overview.sections.traffic }}</h2>
          <span v-if="overview?.range.approximate" class="badge badge-unknown">{{ copy.overview.approximate }}</span>
        </div>
        <div class="panel-body">
          <StateMessage
            v-if="sectionState === 'loading'"
            kind="loading"
            :message="copy.common.loading"
          />
          <StateMessage
            v-else-if="sectionState === 'error'"
            kind="error"
            :message="copy.common.errorTitle"
            @retry="loadOverview"
          />
          <StateMessage
            v-else-if="chartEmpty"
            kind="empty"
            :message="copy.overview.empty.traffic"
          />
          <TrafficChart v-else :series="seriesData" :bucket="rangeInfo?.bucket ?? ''" />
          <p v-if="rangeInfo !== null" class="faint mono bucket-note">
            {{ formatRangeTime(rangeInfo.from) }} - {{ formatRangeTime(rangeInfo.to) }} / {{ rangeInfo.bucket }}
          </p>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <h2 class="panel-title">{{ copy.overview.sections.breakdowns }}</h2>
        </div>
        <div class="panel-body breakdown-grid">
          <BreakdownList
            :title="copy.overview.breakdownLabels.categories"
            :items="overview?.breakdowns.categories ?? []"
            :loading="sectionState === 'loading'"
            :error="sectionState === 'error'"
            :empty-message="copy.overview.empty.breakdown"
            @retry="loadOverview"
          />
          <BreakdownList
            :title="copy.overview.breakdownLabels.rules"
            :items="overview?.breakdowns.rules ?? []"
            :loading="sectionState === 'loading'"
            :error="sectionState === 'error'"
            :empty-message="copy.overview.empty.breakdown"
            @retry="loadOverview"
          />
          <BreakdownList
            :title="copy.overview.breakdownLabels.actions"
            :items="overview?.breakdowns.actions ?? []"
            action-style
            :loading="sectionState === 'loading'"
            :error="sectionState === 'error'"
            :empty-message="copy.overview.empty.breakdown"
            @retry="loadOverview"
          />
          <BreakdownList
            :title="copy.overview.breakdownLabels.sites"
            :items="overview?.breakdowns.sites ?? []"
            :loading="sectionState === 'loading'"
            :error="sectionState === 'error'"
            :empty-message="copy.overview.empty.sites"
            @retry="loadOverview"
          />
        </div>
      </div>
    </section>

    <section class="grid-3">
      <div class="panel">
        <div class="panel-header"><h2 class="panel-title">{{ copy.overview.sections.topIps }}</h2></div>
        <StateMessage v-if="overviewLoading && overview === null" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="overviewError && overview === null" kind="error" :message="copy.common.errorTitle" @retry="loadOverview" />
        <StateMessage v-else-if="topIps.length === 0" kind="empty" :message="copy.overview.empty.ips" />
        <div v-else class="table-scroll">
          <table>
            <thead>
              <tr>
                <th scope="col">{{ copy.overview.tables.ip }}</th>
                <th scope="col" class="numeric" aria-sort="none">{{ copy.overview.tables.requests }}</th>
                <th scope="col" class="numeric" aria-sort="none">{{ copy.overview.tables.blocked }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in topIps" :key="row.ip">
                <td class="mono wrap">{{ row.ip }}</td>
                <td class="numeric mono">{{ formatNumber(row.requests) }}</td>
                <td class="numeric mono">{{ formatNumber(row.blocked) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header"><h2 class="panel-title">{{ copy.overview.sections.topPaths }}</h2></div>
        <StateMessage v-if="overviewLoading && overview === null" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="overviewError && overview === null" kind="error" :message="copy.common.errorTitle" @retry="loadOverview" />
        <StateMessage v-else-if="topPaths.length === 0" kind="empty" :message="copy.overview.empty.paths" />
        <div v-else class="table-scroll">
          <table>
            <thead>
              <tr>
                <th scope="col">{{ copy.overview.tables.path }}</th>
                <th scope="col" class="numeric" aria-sort="none">{{ copy.overview.tables.requests }}</th>
                <th scope="col" class="numeric" aria-sort="none">{{ copy.overview.tables.blocked }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in topPaths" :key="row.path">
                <td class="mono wrap">{{ row.path }}</td>
                <td class="numeric mono">{{ formatNumber(row.requests) }}</td>
                <td class="numeric mono">{{ formatNumber(row.blocked) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header"><h2 class="panel-title">{{ copy.overview.sections.latestBlocked }}</h2></div>
        <StateMessage v-if="latestLoading && latest.length === 0" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="latestError && latest.length === 0" kind="error" :message="copy.common.errorTitle" @retry="loadLatest" />
        <StateMessage v-else-if="latest.length === 0" kind="empty" :message="copy.overview.empty.blocked" />
        <div v-else class="table-scroll">
          <table>
            <thead>
              <tr>
                <th scope="col">{{ copy.overview.tables.time }}</th>
                <th scope="col">{{ copy.overview.tables.action }}</th>
                <th scope="col">{{ copy.overview.tables.rule }}</th>
                <th scope="col">{{ copy.overview.tables.path }}</th>
                <th scope="col">{{ copy.overview.tables.ip }}</th>
                <th scope="col" class="numeric" aria-sort="none">{{ copy.overview.tables.status }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in latest" :key="row.id">
                <td class="mono">{{ formatDayTime(row.ts) }}</td>
                <td><span :class="actionClass(row.action)">{{ actionLabel(row.action) }}</span></td>
                <td class="mono wrap">{{ row.rule_id }}</td>
                <td class="mono wrap">{{ row.path }}</td>
                <td class="mono wrap">{{ row.client_ip }}</td>
                <td class="numeric mono">{{ row.status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.stale {
  margin: 0;
  color: var(--recorded);
  font-size: var(--fs-13);
}

.hero-body {
  display: flex;
  align-items: center;
  gap: var(--sp-24);
  flex-wrap: wrap;
}

.hero-gauge {
  flex: 1 1 240px;
}

.hero-counters {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--sp-12);
  flex: 2 1 300px;
}

.counter {
  border: 1px solid var(--border);
  border-left: 3px solid var(--border-strong);
  border-radius: var(--radius);
  padding: var(--sp-12) var(--sp-16);
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
  min-width: 0;
}

.counter dt {
  color: var(--text-dim);
  font-size: var(--fs-13);
}

.counter dd {
  margin: 0;
  font-size: var(--fs-20);
  overflow-wrap: anywhere;
}

.counter-total {
  border-left-color: var(--accent);
}

.counter-blocked {
  border-left-color: var(--blocked);
}

.counter-shadow {
  border-left-color: var(--recorded);
}

.counter-ratelimited {
  border-left-color: var(--ratelimited);
}

.site-select {
  width: auto;
  min-width: 160px;
}

.breakdown-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--sp-16);
}

.bucket-note {
  margin: var(--sp-8) 0 0;
}

@media (max-width: 640px) {
  .breakdown-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .site-select {
    min-width: 120px;
  }
}
</style>
