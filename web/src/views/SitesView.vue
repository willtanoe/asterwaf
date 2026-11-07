<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { api } from "../api/client";
import type { SiteView } from "../api/types";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import Drawer from "../components/Drawer.vue";
import Select from "../components/Select.vue";
import StateMessage from "../components/StateMessage.vue";
import type { SiteDetailResponse, OverviewResponse } from "../api/types";
import { copy } from "../copy";
import { formatClock, formatLatency, formatMib, formatNumber, siteStatusLabel } from "../format";

const modeOptions = [
  { value: "off", label: "off" },
  { value: "shadow", label: "shadow" },
  { value: "enforce", label: "enforce" },
];

const modeRank: Record<string, number> = { off: 0, shadow: 1, enforce: 2 };

const sites = ref<SiteView[]>([]);
const loading = ref(true);
const error = ref(false);
const modes = reactive<Record<string, string>>({});
const saving = ref("");
const message = ref("");
const messageOk = ref(true);

const pendingSite = ref<SiteView | null>(null);
const detail = ref<SiteDetailResponse | null>(null);
const detailHost = ref("");
const detailLoading = ref(false);
const overview = ref<OverviewResponse | null>(null);

const perSite = computed(() => {
  const map = new Map<string, OverviewResponse["per_site"][number]>();
  for (const row of overview.value?.per_site ?? []) {
    map.set(row.site, row);
  }
  return map;
});

const shadowMap = computed(() => {
  const map = new Map<string, number>();
  for (const row of overview.value?.shadow_wouldblock ?? []) {
    map.set(row.site, row.count);
  }
  return map;
});

async function openDetail(host: string): Promise<void> {
  detailHost.value = host;
  detail.value = null;
  detailLoading.value = true;
  try {
    detail.value = await api.siteDetail(host);
  } catch {
    detail.value = null;
  } finally {
    detailLoading.value = false;
  }
}

function closeDetail(): void {
  detailHost.value = "";
}

let timer = 0;

async function load(initial: boolean): Promise<void> {
  if (initial) {
    loading.value = true;
  }
  try {
    const response = await api.sites();
    sites.value = response.sites;
    for (const site of response.sites) {
      if (modes[site.host] === undefined) {
        modes[site.host] = site.mode;
      }
    }
    error.value = false;
    try {
      overview.value = await api.overview("24h", "");
    } catch {
      overview.value = null;
    }
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}

function requestMode(site: SiteView): void {
  // Lowering away from enforce silently stops blocking; require a confirm.
  if (site.mode === "enforce" && modeRank[modes[site.host]] < modeRank.enforce) {
    pendingSite.value = site;
    return;
  }
  void saveMode(site);
}

async function confirmMode(): Promise<void> {
  const site = pendingSite.value;
  pendingSite.value = null;
  if (site !== null) {
    await saveMode(site);
  }
}

async function saveMode(site: SiteView): Promise<void> {
  saving.value = site.host;
  message.value = "";
  try {
    await api.saveSiteMode(site.host, modes[site.host]);
    messageOk.value = true;
    message.value = copy.sites.modeSaved;
    await load(false);
  } catch {
    messageOk.value = false;
    message.value = copy.sites.modeFailed;
  } finally {
    saving.value = "";
  }
}

onMounted(() => {
  void load(true);
  timer = window.setInterval(() => {
    if (!document.hidden) {
      void load(false);
    }
  }, 10000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.sites.title }}</h1>
        <p class="page-subtitle">{{ copy.sites.subtitle }}</p>
      </div>
      <span class="faint">{{ copy.sites.polling }}</span>
    </header>

    <p v-if="message !== ''" :class="messageOk ? 'action-ok' : 'action-error'" role="status">{{ message }}</p>

    <div class="panel">
      <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="error" kind="error" :message="copy.sites.loadFailed" @retry="load(true)" />
      <StateMessage v-else-if="sites.length === 0" kind="empty" :message="copy.sites.empty" />
      <div v-else class="table-scroll">
        <table class="sites-table">
          <thead>
            <tr>
              <th scope="col">{{ copy.sites.table.host }}</th>
              <th scope="col">{{ copy.sites.table.upstream }}</th>
              <th scope="col">{{ copy.sites.table.mode }}</th>
              <th scope="col">{{ copy.sites.table.health }}</th>
              <th scope="col" class="numeric">{{ copy.sites.table.latency }}</th>
              <th scope="col" class="numeric">{{ copy.sites.table.failures }}</th>
              <th scope="col">{{ copy.sites.table.lastCheck }}</th>
              <th scope="col">{{ copy.sites.table.healthPath }}</th>
              <th scope="col" class="numeric">{{ copy.sites.table.maxBody }}</th>
              <th scope="col" class="numeric">{{ copy.overview.perSite.requests }}</th>
              <th scope="col" class="numeric">{{ copy.overview.perSite.blocked }}</th>
              <th scope="col" class="numeric">{{ copy.overview.perSite.blockRate }}</th>
              <th scope="col" class="numeric">{{ copy.overview.perSite.p95 }}</th>
              <th scope="col" class="numeric">{{ copy.overview.sections2.shadow }}</th>
              <th scope="col">{{ copy.overview.perSite.trend }}</th>
              <th scope="col">{{ copy.sites.table.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in sites" :key="site.host">
              <td class="mono"><span class="truncate block-cell" :title="site.host">{{ site.host }}</span></td>
              <td class="mono"><span class="truncate block-cell" :title="site.upstream">{{ site.upstream }}</span></td>
              <td>
                <Select
                  class="mode-select"
                  :model-value="modes[site.host]"
                  :options="modeOptions"
                  :label="`${copy.sites.table.mode}: ${site.host}`"
                  @update:model-value="modes[site.host] = $event"
                />
              </td>
              <td>
                <span class="health-cell">
                  <span
                    class="dot"
                    :class="site.status === 'up' ? 'dot-ok' : site.status === 'down' ? 'dot-down' : 'dot-unknown'"
                    aria-hidden="true"
                  />
                  {{ siteStatusLabel(site.status) }}
                </span>
              </td>
              <td class="numeric mono">{{ site.latency_ms }} ms</td>
              <td class="numeric mono">{{ site.failures }}</td>
              <td class="mono">{{ site.last_check ? formatClock(site.last_check) : "—" }}</td>
              <td class="mono">{{ site.health_path }}</td>
              <td class="numeric mono">{{ formatMib(site.max_body_bytes) }}</td>
              <td class="numeric mono">{{ formatNumber(perSite.get(site.host)?.requests ?? 0) }}</td>
              <td class="numeric mono">{{ formatNumber(perSite.get(site.host)?.blocked ?? 0) }}</td>
              <td class="numeric mono">{{ (perSite.get(site.host)?.block_rate ?? 0).toFixed(1) }}%</td>
              <td class="numeric mono">{{ formatLatency(perSite.get(site.host)?.p95 ?? 0) }}</td>
              <td class="numeric mono">{{ formatNumber(shadowMap.get(site.host) ?? 0) }}</td>
              <td>
                <button type="button" class="btn btn-small" @click="openDetail(site.host)">
                  {{ copy.sites.detail.title }}
                </button>
              </td>
              <td>
                <button
                  type="button"
                  class="btn btn-small"
                  :disabled="saving === site.host || modes[site.host] === site.mode"
                  @click="requestMode(site)"
                >
                  {{ saving === site.host ? copy.sites.saving : copy.sites.saveMode }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <ConfirmDialog
      :open="pendingSite !== null"
      :title="copy.sites.confirmLowerTitle"
      :message="`${copy.sites.confirmLowerMessage} (${pendingSite?.host ?? ''}: enforce -> ${modes[pendingSite?.host ?? '']})`"
      :confirm-label="copy.sites.saveMode"
      danger
      @confirm="confirmMode"
      @cancel="pendingSite = null"
    />

    <Drawer :open="detailHost !== ''" :title="`${copy.sites.detail.title}: ${detailHost}`" @close="closeDetail">
      <StateMessage v-if="detailLoading" kind="loading" :message="copy.common.loading" />
      <template v-else-if="detail !== null">
        <section class="stack">
          <h3 class="panel-title">{{ copy.sites.detail.healthHistory }}</h3>
          <div class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">{{ copy.overview.tables.time }}</th>
                  <th scope="col">{{ copy.sites.table.health }}</th>
                  <th scope="col" class="numeric">{{ copy.sites.table.latency }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="sample in [...detail.health_history].reverse()" :key="sample.ts">
                  <td class="mono">{{ formatClock(sample.ts) }}</td>
                  <td>{{ siteStatusLabel(sample.status) }}</td>
                  <td class="numeric mono">{{ formatLatency(sample.latency_ms) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="stack">
          <h3 class="panel-title">{{ copy.sites.detail.modeHistory }}</h3>
          <p v-if="detail.mode_history.length === 0" class="muted">{{ copy.sites.detail.noModeChanges }}</p>
          <div v-else class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">{{ copy.overview.tables.time }}</th>
                  <th scope="col">{{ copy.rules.versions.actor }}</th>
                  <th scope="col">{{ copy.overview.tables.action }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="change in detail.mode_history" :key="change.ts">
                  <td class="mono">{{ formatClock(change.ts) }}</td>
                  <td class="mono">{{ change.actor }}</td>
                  <td class="mono">{{ change.before }} → {{ change.after }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="stack">
          <h3 class="panel-title">{{ copy.sites.detail.recentBlocked }}</h3>
          <p v-if="detail.recent_blocked.length === 0" class="muted">{{ copy.overview.empty.blocked }}</p>
          <div v-else class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">{{ copy.overview.tables.time }}</th>
                  <th scope="col">{{ copy.overview.tables.rule }}</th>
                  <th scope="col">{{ copy.overview.tables.path }}</th>
                  <th scope="col">{{ copy.overview.tables.ip }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in detail.recent_blocked" :key="row.id">
                  <td class="mono">{{ formatClock(row.ts) }}</td>
                  <td class="mono">{{ row.rule_id }}</td>
                  <td class="mono wrap">{{ row.path }}</td>
                  <td class="mono">{{ row.client_ip }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
      <StateMessage v-else kind="error" :message="copy.common.errorTitle" />
    </Drawer>
  </div>
</template>

<style scoped>
.block-cell {
  display: block;
  max-width: 200px;
}

.mode-select {
  min-width: 120px;
}

.sites-table {
  width: 100%;
}

@media (min-width: 1900px) {
  /* Keep cells tight on very wide displays; the table spans the full width. */
  .sites-table th,
  .sites-table td {
    padding-inline: var(--sp-8);
  }
}

.health-cell {
  display: inline-flex;
  align-items: center;
  gap: var(--sp-8);
  white-space: nowrap;
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

.action-ok {
  margin: 0;
  color: var(--ok);
  font-size: var(--fs-13);
}

.action-error {
  margin: 0;
  color: var(--blocked);
  font-size: var(--fs-13);
}
</style>
