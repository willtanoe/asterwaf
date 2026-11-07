<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { api } from "../api/client";
import type { SiteView } from "../api/types";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import Select from "../components/Select.vue";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { formatClock, formatNumber, siteStatusLabel } from "../format";

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
        <table>
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
              <th scope="col">{{ copy.sites.table.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in sites" :key="site.host">
              <td class="mono wrap">{{ site.host }}</td>
              <td class="mono wrap">{{ site.upstream }}</td>
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
              <td class="numeric mono">{{ formatNumber(site.max_body_bytes) }}</td>
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
  </div>
</template>

<style scoped>
.mode-select {
  min-width: 120px;
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
