<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { api } from "../api/client";
import type { LogEntry, SiteView } from "../api/types";
import Drawer from "../components/Drawer.vue";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { actionClass, actionLabel, formatDayTime } from "../format";

type SortKey = "ts" | "status" | "duration_ms";
type SortDirection = "asc" | "desc";

const filters = reactive({
  from: "",
  to: "",
  site: "",
  action: "",
  category: "",
  rule: "",
  ip: "",
  path: "",
  status: "",
});

const sites = ref<SiteView[]>([]);
const items = ref<LogEntry[]>([]);
const cursor = ref("");
const loading = ref(true);
const loadingMore = ref(false);
const error = ref(false);
const sortKey = ref<SortKey>("ts");
const sortDirection = ref<SortDirection>("desc");

const selected = ref<LogEntry | null>(null);
const detail = ref<LogEntry | null>(null);
const detailLoading = ref(false);
const detailError = ref(false);
const actionMessage = ref("");
const actionError = ref(false);
const acting = ref(false);

const sorted = computed(() => {
  const list = [...items.value];
  const factor = sortDirection.value === "asc" ? 1 : -1;
  list.sort((left, right) => {
    const a = left[sortKey.value];
    const b = right[sortKey.value];
    return (a - b) * factor;
  });
  return list;
});

function sortState(key: SortKey): "ascending" | "descending" | "none" {
  return sortKey.value === key ? (sortDirection.value === "asc" ? "ascending" : "descending") : "none";
}

function toggleSort(key: SortKey): void {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === "asc" ? "desc" : "asc";
  } else {
    sortKey.value = key;
    sortDirection.value = key === "ts" ? "desc" : "asc";
  }
}

function buildQuery(extra: Record<string, string> = {}): string {
  const params = new URLSearchParams();
  const add = (key: string, value: string): void => {
    if (value !== "") {
      params.set(key, value);
    }
  };
  add("site", filters.site);
  add("action", filters.action);
  add("category", filters.category);
  add("rule_id", filters.rule);
  add("ip", filters.ip);
  add("path", filters.path);
  if (filters.status !== "") {
    add("status", filters.status);
  }
  if (filters.from !== "") {
    add("from", new Date(filters.from).toISOString());
  }
  if (filters.to !== "") {
    add("to", new Date(filters.to).toISOString());
  }
  for (const [key, value] of Object.entries(extra)) {
    add(key, value);
  }
  return params.toString();
}

async function load(reset: boolean): Promise<void> {
  if (reset) {
    loading.value = true;
    error.value = false;
    items.value = [];
    cursor.value = "";
  } else {
    loadingMore.value = true;
  }
  try {
    const response = await api.logs(buildQuery(cursor.value !== "" ? { cursor: cursor.value } : {}));
    items.value = reset ? response.items : [...items.value, ...response.items];
    cursor.value = response.next_cursor ?? "";
    error.value = false;
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

async function loadSites(): Promise<void> {
  try {
    sites.value = (await api.sites()).sites;
  } catch {
    sites.value = [];
  }
}

function resetFilters(): void {
  filters.from = "";
  filters.to = "";
  filters.site = "";
  filters.action = "";
  filters.category = "";
  filters.rule = "";
  filters.ip = "";
  filters.path = "";
  filters.status = "";
  void load(true);
}

async function openDetail(row: LogEntry): Promise<void> {
  selected.value = row;
  detail.value = null;
  detailError.value = false;
  detailLoading.value = true;
  actionMessage.value = "";
  actionError.value = false;
  try {
    detail.value = await api.log(row.id);
  } catch {
    detailError.value = true;
  } finally {
    detailLoading.value = false;
  }
}

function closeDetail(): void {
  selected.value = null;
}

async function createEntry(action: "allow" | "deny"): Promise<void> {
  const row = detail.value ?? selected.value;
  if (row === null) {
    return;
  }
  acting.value = true;
  actionMessage.value = "";
  actionError.value = false;
  try {
    await api.createAccessEntry({
      action,
      address: row.client_ip,
      note: `Created from log ${row.id}`,
      enabled: true,
      source_log_id: row.id,
    });
    actionMessage.value = copy.logs.detail.created;
  } catch {
    actionError.value = true;
    actionMessage.value = copy.logs.detail.actionFailed;
  } finally {
    acting.value = false;
  }
}

const canDisableRule = computed(() => {
  const id = (detail.value ?? selected.value)?.rule_id ?? "";
  return id !== "" && !id.startsWith("acl:") && !id.startsWith("body-") && id !== "ratelimit";
});

async function disableRule(): Promise<void> {
  const row = detail.value ?? selected.value;
  if (row === null || row.rule_id === "") {
    return;
  }
  acting.value = true;
  actionMessage.value = "";
  actionError.value = false;
  try {
    await api.toggleRule(row.rule_id);
    actionMessage.value = copy.logs.detail.ruleDisabled;
  } catch {
    actionError.value = true;
    actionMessage.value = copy.logs.detail.actionFailed;
  } finally {
    acting.value = false;
  }
}

function rowKey(event: KeyboardEvent): void {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    const target = event.currentTarget as HTMLElement | null;
    const id = target?.dataset.rowId;
    const row = items.value.find((item) => String(item.id) === id);
    if (row !== undefined) {
      void openDetail(row);
    }
  }
}

onMounted(() => {
  void loadSites();
  void load(true);
});
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.logs.title }}</h1>
        <p class="page-subtitle">{{ copy.logs.subtitle }}</p>
      </div>
    </header>

    <form class="panel filter-panel" @submit.prevent="load(true)">
      <div class="filter-grid">
        <label class="field">
          <span>{{ copy.logs.filters.from }}</span>
          <input v-model="filters.from" class="input" type="datetime-local" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.to }}</span>
          <input v-model="filters.to" class="input" type="datetime-local" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.site }}</span>
          <select v-model="filters.site" class="select">
            <option value="">{{ copy.logs.filters.any }}</option>
            <option v-for="site in sites" :key="site.host" :value="site.host">{{ site.host }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.action }}</span>
          <select v-model="filters.action" class="select">
            <option value="">{{ copy.logs.filters.any }}</option>
            <option value="allow">{{ copy.actions.allow }}</option>
            <option value="block">{{ copy.actions.block }}</option>
            <option value="shadow">{{ copy.actions.shadow }}</option>
            <option value="ratelimited">{{ copy.actions.ratelimited }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.category }}</span>
          <input v-model="filters.category" class="input" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.rule }}</span>
          <input v-model="filters.rule" class="input" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.ip }}</span>
          <input v-model="filters.ip" class="input" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.path }}</span>
          <input v-model="filters.path" class="input" />
        </label>
        <label class="field">
          <span>{{ copy.logs.filters.status }}</span>
          <input v-model="filters.status" class="input" inputmode="numeric" />
        </label>
      </div>
      <div class="row">
        <button type="submit" class="btn btn-primary">{{ copy.logs.filters.apply }}</button>
        <button type="button" class="btn" @click="resetFilters">{{ copy.logs.filters.reset }}</button>
      </div>
    </form>

    <div class="panel">
      <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="error" kind="error" :message="copy.logs.loadFailed" @retry="load(true)" />
      <StateMessage v-else-if="items.length === 0" kind="empty" :message="copy.logs.empty" />
      <div v-else class="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col" :aria-sort="sortState('ts')">
                <button type="button" class="sort" @click="toggleSort('ts')">
                  {{ copy.logs.table.time }}
                </button>
              </th>
              <th scope="col">{{ copy.logs.table.action }}</th>
              <th scope="col">{{ copy.logs.table.site }}</th>
              <th scope="col">{{ copy.logs.table.method }}</th>
              <th scope="col">{{ copy.logs.table.path }}</th>
              <th scope="col" :aria-sort="sortState('status')">
                <button type="button" class="sort" @click="toggleSort('status')">
                  {{ copy.logs.table.status }}
                </button>
              </th>
              <th scope="col">{{ copy.logs.table.ip }}</th>
              <th scope="col">{{ copy.logs.table.rule }}</th>
              <th scope="col" :aria-sort="sortState('duration_ms')">
                <button type="button" class="sort" @click="toggleSort('duration_ms')">
                  {{ copy.logs.table.duration }}
                </button>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in sorted"
              :key="row.id"
              :data-row-id="row.id"
              tabindex="0"
              class="clickable"
              @click="openDetail(row)"
              @keydown="rowKey"
            >
              <td class="mono">{{ formatDayTime(row.ts) }}</td>
              <td><span :class="actionClass(row.action)">{{ actionLabel(row.action) }}</span></td>
              <td class="wrap">{{ row.site }}</td>
              <td class="mono">{{ row.method }}</td>
              <td class="mono wrap path-cell">{{ row.path }}</td>
              <td class="numeric mono">{{ row.status }}</td>
              <td class="mono wrap">{{ row.client_ip }}</td>
              <td class="mono wrap">{{ row.rule_id }}</td>
              <td class="numeric mono">{{ row.duration_ms }} ms</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="cursor !== '' && !loading" class="panel-footer">
        <button type="button" class="btn" :disabled="loadingMore" @click="load(false)">
          {{ loadingMore ? copy.common.loading : copy.logs.loadMore }}
        </button>
      </div>
    </div>

    <Drawer :open="selected !== null" :title="copy.logs.detail.title" @close="closeDetail">
      <StateMessage v-if="detailLoading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="detailError" kind="error" :message="copy.common.errorTitle" />
      <template v-else-if="detail !== null">
        <section class="stack">
          <div class="row">
            <span :class="actionClass(detail.action)">{{ actionLabel(detail.action) }}</span>
            <span class="mono">{{ detail.method }} {{ detail.path }}{{ detail.query !== "" ? "?" + detail.query : "" }}</span>
            <span class="badge badge-allow">{{ detail.status }}</span>
          </div>
          <dl class="meta-grid">
            <dt>{{ copy.logs.detail.requestId }}</dt>
            <dd class="mono wrap">{{ detail.request_id }}</dd>
            <dt>{{ copy.logs.table.site }}</dt>
            <dd class="wrap">{{ detail.site }}</dd>
            <dt>{{ copy.logs.table.ip }}</dt>
            <dd class="mono wrap">{{ detail.client_ip }}</dd>
            <dt>{{ copy.logs.table.rule }}</dt>
            <dd class="mono wrap">{{ detail.rule_id || copy.reserved.none }}</dd>
            <dt>{{ copy.logs.table.duration }}</dt>
            <dd class="mono">{{ detail.duration_ms }} ms</dd>
            <dt>{{ copy.logs.detail.userAgent }}</dt>
            <dd class="wrap">{{ detail.user_agent }}</dd>
          </dl>
        </section>

        <section class="stack">
          <h3 class="panel-title">{{ copy.logs.detail.matched }}</h3>
          <pre class="text-block">{{ detail.matched || copy.reserved.none }}</pre>
        </section>
        <section class="stack">
          <h3 class="panel-title">{{ copy.logs.detail.headers }}</h3>
          <pre v-if="detail.request_headers" class="text-block">{{ detail.request_headers }}</pre>
          <p v-else class="muted">{{ copy.logs.detail.noHeaders }}</p>
        </section>
        <section class="stack">
          <h3 class="panel-title">{{ copy.logs.detail.body }}</h3>
          <pre v-if="detail.request_body" class="text-block">{{ detail.request_body }}</pre>
          <p v-else class="muted">{{ copy.logs.detail.noBody }}</p>
        </section>

        <div class="row">
          <button type="button" class="btn" :disabled="acting" @click="createEntry('allow')">
            {{ copy.logs.detail.allow }}
          </button>
          <button type="button" class="btn" :disabled="acting" @click="createEntry('deny')">
            {{ copy.logs.detail.deny }}
          </button>
          <button
            type="button"
            class="btn"
            :disabled="acting || !canDisableRule"
            :title="canDisableRule ? '' : copy.logs.detail.noRule"
            @click="disableRule"
          >
            {{ copy.logs.detail.disableRule }}
          </button>
        </div>
        <p v-if="actionMessage !== ''" :class="actionError ? 'action-error' : 'action-ok'" role="status">
          {{ actionMessage }}
        </p>
      </template>
    </Drawer>
  </div>
</template>

<style scoped>
.filter-panel {
  padding: var(--sp-16);
  display: flex;
  flex-direction: column;
  gap: var(--sp-12);
}

.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--sp-12);
}

.field span {
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.sort {
  background: none;
  border: 0;
  padding: 0;
  color: inherit;
  font: inherit;
  font-weight: 600;
  white-space: nowrap;
}

.sort:hover {
  color: var(--text);
  text-decoration: underline;
}

.clickable {
  cursor: pointer;
}

.path-cell {
  max-width: 320px;
}

.panel-footer {
  padding: var(--sp-12) var(--sp-16);
  border-top: 1px solid var(--border);
}

.meta-grid {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: var(--sp-4) var(--sp-12);
  margin: 0;
  font-size: var(--fs-13);
}

.meta-grid dt {
  color: var(--text-dim);
}

.meta-grid dd {
  margin: 0;
}

.text-block {
  margin: 0;
  padding: var(--sp-12);
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 240px;
  overflow-y: auto;
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

@media (max-width: 1000px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .filter-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
