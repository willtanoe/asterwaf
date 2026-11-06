<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { api } from "../api/client";
import type { SiteView } from "../api/types";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { formatNumber, siteStatusLabel } from "../format";

const sites = ref<SiteView[]>([]);
const loading = ref(true);
const error = ref(false);
const modes = reactive<Record<string, string>>({});
const saving = ref("");
const message = ref("");
const messageOk = ref(true);

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
                <select v-model="modes[site.host]" class="select mode-select">
                  <option value="off">off</option>
                  <option value="shadow">shadow</option>
                  <option value="enforce">enforce</option>
                </select>
              </td>
              <td>
                <span
                  class="badge"
                  :class="site.status === 'up' ? 'badge-ok' : site.status === 'down' ? 'badge-down' : 'badge-unknown'"
                >
                  {{ siteStatusLabel(site.status) }}
                </span>
              </td>
              <td class="numeric mono">{{ site.latency_ms }} ms</td>
              <td class="numeric mono">{{ site.failures }}</td>
              <td class="mono">{{ site.health_path }}</td>
              <td class="numeric mono">{{ formatNumber(site.max_body_bytes) }}</td>
              <td>
                <button
                  type="button"
                  class="btn btn-small"
                  :disabled="saving === site.host || modes[site.host] === site.mode"
                  @click="saveMode(site)"
                >
                  {{ saving === site.host ? copy.sites.saving : copy.sites.saveMode }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mode-select {
  min-width: 120px;
  padding: var(--sp-4) var(--sp-8);
  font-size: var(--fs-13);
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
