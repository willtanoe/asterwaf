<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { api } from "../api/client";
import type { LogEntry, OverviewResponse, SiteView, StatusResponse } from "../api/types";
import ActionDonut from "../components/ActionDonut.vue";
import BreakdownList from "../components/BreakdownList.vue";
import CopyButton from "../components/CopyButton.vue";
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

const OVERVIEW_INTERVAL = 15000;
const LATEST_INTERVAL = 10000;
const STATUS_INTERVAL = 30000;

const sites = ref<SiteView[]>([]);
const siteOptions = computed(() => [
  { value: "", label: copy.common.allSites },
  ...sites.value.map((item) => ({ value: item.host, label: item.host })),
]);

const detectionsTotal = computed(() =>
  (overview.value?.breakdowns.categories ?? []).reduce((sum, item) => sum + item.count, 0),
);
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
let overviewInFlight = false;
let statusInFlight = false;
let latestInFlight = false;

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

// Definitions live in tooltips so permanent body text stays numeric.
const kpiNote = (key: string): string => {
  if (key === "unique_ips") {
    return overview.value?.range.approximate
      ? copy.overview.kpiContext.uniqueApprox
      : copy.overview.kpiContext.uniqueRaw;
  }
  if (key === "latency_p50_ms" || key === "latency_p95_ms") {
    return copy.overview.kpiContext.latency;
  }
  if (key === "block_rate") {
    return copy.overview.kpiContext.blockRate;
  }
  return "";
};

type KpiKey = keyof typeof copy.overview.kpis;

interface KpiDef {
  key: KpiKey;
  color: string;
  format: (value: number) => string;
  secondary?: () => { label: string; value: string } | undefined;
  flag?: () => string | undefined;
  link?: () => { to: string; label: string } | undefined;
  value?: () => string;
}

const kpiDefs: KpiDef[] = [
  { key: "total_requests", color: "--accent", format: formatNumber },
  {
    key: "blocked_requests",
    color: "--blocked",
    format: formatNumber,
    secondary: () =>
      overview.value?.kpis.shadow_requests === undefined
        ? undefined
        : { label: copy.overview.kpis.shadow_requests, value: kpiValue("shadow_requests", formatNumber) },
  },
  { key: "block_rate", color: "--accent", format: formatPercent },
  {
    key: "ratelimited_requests",
    color: "--ratelimited",
    format: formatNumber,
    value: () => (overview.value?.rate_limit_enabled === false ? "Off" : ""),
    link: () =>
      overview.value?.rate_limit_enabled === false
        ? { to: "/access-lists", label: copy.overview.rateLimitOff }
        : undefined,
  },
  {
    key: "unique_ips",
    color: "--accent",
    format: formatNumber,
    flag: () => (overview.value?.range.approximate === true ? copy.overview.approximate.toLowerCase() : undefined),
  },
  { key: "latency_p50_ms", color: "--text-dim", format: formatLatency },
  { key: "latency_p95_ms", color: "--text-dim", format: formatLatency },
];

type SortKey = "key" | "requests" | "blocked";
interface SortState {
  key: SortKey;
  dir: "asc" | "desc";
}

const ipSort = ref<SortState>({ key: "requests", dir: "desc" });
const pathSort = ref<SortState>({ key: "requests", dir: "desc" });

function toggleSort(state: SortState, key: SortKey): SortState {
  if (state.key === key) {
    return { key, dir: state.dir === "desc" ? "asc" : "desc" };
  }
  return { key, dir: "desc" };
}

function sortRows<T>(
  rows: T[],
  state: SortState,
  value: (row: T) => string,
  numeric: (row: T, key: "requests" | "blocked") => number,
): T[] {
  const sorted = [...rows].sort((left, right) => {
    switch (state.key) {
      case "requests":
        return numeric(left, "requests") - numeric(right, "requests");
      case "blocked":
        return numeric(left, "blocked") - numeric(right, "blocked");
      default:
        return value(left).localeCompare(value(right));
    }
  });
  if (state.dir === "desc") {
    sorted.reverse();
  }
  return sorted;
}

const sortedIps = computed(() =>
  sortRows(
    topIps.value,
    ipSort.value,
    (row) => row.ip,
    (row, key) => row[key],
  ),
);
const sortedPaths = computed(() =>
  sortRows(
    topPaths.value,
    pathSort.value,
    (row) => row.path,
    (row, key) => row[key],
  ),
);

const ariaSort = (state: SortState, key: SortKey): "ascending" | "descending" | "none" => {
  if (state.key !== key) {
    return "none";
  }
  return state.dir === "asc" ? "ascending" : "descending";
};

async function loadOverview(): Promise<void> {
  if (overviewInFlight) {
    return;
  }
  overviewInFlight = true;
  overviewLoading.value = overview.value === null;
  try {
    overview.value = await api.overview(range.value, site.value);
    updatedAt.value = Date.now();
    overviewError.value = false;
  } catch {
    overviewError.value = true;
  } finally {
    overviewInFlight = false;
    overviewLoading.value = false;
  }
}

async function loadStatus(): Promise<void> {
  if (statusInFlight) {
    return;
  }
  statusInFlight = true;
  statusLoading.value = status.value === null;
  try {
    status.value = await api.status();
    statusError.value = false;
  } catch {
    statusError.value = true;
  } finally {
    statusInFlight = false;
    statusLoading.value = false;
  }
}

async function loadLatest(): Promise<void> {
  if (latestInFlight) {
    return;
  }
  latestInFlight = true;
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
    latestInFlight = false;
    latestLoading.value = false;
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
  }, OVERVIEW_INTERVAL);
  latestTimer = window.setInterval(() => {
    if (!document.hidden) {
      void loadLatest();
    }
  }, LATEST_INTERVAL);
  statusTimer = window.setInterval(() => {
    if (!document.hidden) {
      void loadStatus();
    }
  }, STATUS_INTERVAL);
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

async function loadSites(): Promise<void> {
  try {
    const response = await api.sites();
    sites.value = response.sites;
  } catch {
    sites.value = [];
  }
}
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
        <span v-if="updatedAt !== null" class="updated">
          {{ copy.common.updated }} {{ formatClock(updatedAt) }}
        </span>
      </div>
    </header>

    <StatusBar :status="status" :loading="statusLoading" :error="statusError" />

    <p v-if="overviewError && overview !== null" class="stale" role="status">{{ copy.common.stale }}</p>

    <section class="hero-row">
      <div class="panel donut-panel" :aria-label="copy.overview.donut.title">
        <div class="panel-header">
          <h2 class="panel-title">{{ copy.overview.donut.title }}</h2>
        </div>
        <div class="panel-body donut-body">
          <ActionDonut
            :categories="overview?.breakdowns.categories ?? []"
            :actions="overview?.breakdowns.actions ?? []"
            :detections="detectionsTotal"
            :block-rate="kpiValue('block_rate', formatPercent)"
            :state="sectionState"
            @retry="loadOverview"
          />
        </div>
      </div>
      <div class="grid-kpi" aria-label="Key metrics">
        <KpiCard
          v-for="definition in kpiDefs"
          :key="definition.key"
          :label="copy.overview.kpis[definition.key]"
          :value="definition.value !== undefined && definition.value() !== '' ? definition.value() : kpiValue(definition.key, definition.format)"
          :delta="kpiDelta(definition.key)"
          :series="kpiSeries(definition.key)"
          :color="definition.color"
          :note="kpiNote(definition.key)"
          :flag="overview !== null && definition.flag !== undefined ? definition.flag() : undefined"
          :secondary="overview !== null && definition.secondary !== undefined ? definition.secondary() : undefined"
          :is-new="overview !== null && overview.kpis[definition.key] !== undefined && overview.kpis[definition.key].previous === 0 && overview.kpis[definition.key].value > 0"
          :link="definition.link !== undefined ? definition.link() : undefined"
          :state="sectionState"
        />
      </div>
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

    <section class="grid-2">
      <div class="panel">
        <div class="panel-header"><h2 class="panel-title">{{ copy.overview.sections.topIps }}</h2></div>
        <StateMessage v-if="overviewLoading && overview === null" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="overviewError && overview === null" kind="error" :message="copy.common.errorTitle" @retry="loadOverview" />
        <StateMessage v-else-if="topIps.length === 0" kind="empty" :message="copy.overview.empty.ips" />
        <div v-else class="table-scroll">
          <table class="overview-table">
            <thead>
              <tr>
                <th scope="col" :aria-sort="ariaSort(ipSort, 'key')">
                  <button type="button" class="sort-btn" @click="ipSort = toggleSort(ipSort, 'key')">{{ copy.overview.tables.ip }}</button>
                </th>
                <th scope="col" class="numeric" :aria-sort="ariaSort(ipSort, 'requests')">
                  <button type="button" class="sort-btn" @click="ipSort = toggleSort(ipSort, 'requests')">{{ copy.overview.tables.requests }}</button>
                </th>
                <th scope="col" class="numeric" :aria-sort="ariaSort(ipSort, 'blocked')">
                  <button type="button" class="sort-btn" @click="ipSort = toggleSort(ipSort, 'blocked')">{{ copy.overview.tables.blocked }}</button>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in sortedIps" :key="row.ip">
                <td class="mono">
                  <span class="cell-copy">
                    <span class="truncate" :title="row.ip">{{ row.ip }}</span>
                    <CopyButton :value="row.ip" :subject="copy.overview.tables.ip" />
                  </span>
                </td>
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
          <table class="overview-table">
            <thead>
              <tr>
                <th scope="col" :aria-sort="ariaSort(pathSort, 'key')">
                  <button type="button" class="sort-btn" @click="pathSort = toggleSort(pathSort, 'key')">{{ copy.overview.tables.path }}</button>
                </th>
                <th scope="col" class="numeric" :aria-sort="ariaSort(pathSort, 'requests')">
                  <button type="button" class="sort-btn" @click="pathSort = toggleSort(pathSort, 'requests')">{{ copy.overview.tables.requests }}</button>
                </th>
                <th scope="col" class="numeric" :aria-sort="ariaSort(pathSort, 'blocked')">
                  <button type="button" class="sort-btn" @click="pathSort = toggleSort(pathSort, 'blocked')">{{ copy.overview.tables.blocked }}</button>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in sortedPaths" :key="row.path">
                <td class="mono">
                  <span class="cell-copy">
                    <span class="truncate" :title="row.path">{{ row.path }}</span>
                    <CopyButton :value="row.path" :subject="copy.overview.tables.path" />
                  </span>
                </td>
                <td class="numeric mono">{{ formatNumber(row.requests) }}</td>
                <td class="numeric mono">{{ formatNumber(row.blocked) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </section>

    <section class="panel">
      <div class="panel-header"><h2 class="panel-title">{{ copy.overview.sections.latestBlocked }}</h2></div>
        <StateMessage v-if="latestLoading && latest.length === 0" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="latestError && latest.length === 0" kind="error" :message="copy.common.errorTitle" @retry="loadLatest" />
        <StateMessage v-else-if="latest.length === 0" kind="empty" :message="copy.overview.empty.blocked" />
        <div v-else class="table-scroll">
          <table class="overview-table">
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
                <td class="mono"><span class="truncate block-cell" :title="row.rule_id">{{ row.rule_id }}</span></td>
                <td class="mono">
                  <span class="cell-copy">
                    <span class="truncate" :title="row.path">{{ row.path }}</span>
                    <CopyButton :value="row.path" :subject="copy.overview.tables.path" />
                  </span>
                </td>
                <td class="mono">
                  <span class="cell-copy">
                    <span class="truncate" :title="row.client_ip">{{ row.client_ip }}</span>
                    <CopyButton :value="row.client_ip" :subject="copy.overview.tables.ip" />
                  </span>
                </td>
                <td class="numeric mono">{{ row.status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
    </section>
  </div>
</template>

<style scoped>
.overview-table td {
  white-space: nowrap;
}

.hero-row {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: var(--sp-12);
  align-items: start;
}

.donut-panel {
  max-height: 260px;
}

@media (max-width: 767px) {
  .donut-panel {
    max-height: 220px;
  }

  .donut-body {
    max-height: 200px;
  }

  .donut-body :deep(.donut-stage) {
    height: 110px;
  }
}

.grid-kpi .kpi-card {
  align-self: start;
  height: 112px;
}

.donut-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.donut-body {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
}

.donut-body > :deep(.donut) {
  width: 100%;
  height: 100%;
}

@media (max-width: 767px) {
  .hero-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .donut-panel {
    order: -1;
  }

  .donut-body {
    max-height: 200px;
  }
}

.stale {
  margin: 0;
  color: var(--recorded);
  font-size: var(--fs-13);
}

.updated {
  color: var(--text-dim);
  font-size: var(--fs-13);
  font-variant-numeric: tabular-nums;
}

.site-select {
  width: auto;
  min-width: 160px;
}

.breakdown-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: var(--sp-16);
}

.bucket-note {
  margin: var(--sp-8) 0 0;
}

.sort-btn {
  padding: 0;
  background: transparent;
  border: 0;
  color: inherit;
  font: inherit;
  font-weight: inherit;
}

.sort-btn:hover {
  color: var(--text);
}

.cell-copy {
  display: flex;
  align-items: center;
  gap: var(--sp-4);
  min-width: 0;
}

.cell-copy .truncate {
  flex: 1 1 auto;
}

.block-cell {
  display: block;
  max-width: 140px;
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
