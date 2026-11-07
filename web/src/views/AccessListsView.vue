<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { api, ApiError } from "../api/client";
import type { AccessEntry, AccessList, RateLimitConfig } from "../api/types";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import Select from "../components/Select.vue";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { formatDayTime } from "../format";

const list = ref<AccessList | null>(null);
const entries = computed(() => list.value?.entries ?? []);
const loading = ref(true);
const error = ref(false);
const message = ref("");
const messageOk = ref(true);
const deleting = ref("");

const form = reactive({
  action: "deny" as "allow" | "deny",
  address: "",
  note: "",
  enabled: true,
});
const adding = ref(false);
const pendingDelete = ref<AccessEntry | null>(null);

// Server-side validation stays authoritative; this mirrors it so the form
// rejects malformed CIDRs before a request is sent.
function isValidCidr(raw: string): boolean {
  const value = raw.trim();
  if (value === "") {
    return false;
  }
  const slash = value.indexOf("/");
  const address = slash === -1 ? value : value.slice(0, slash);
  const prefixRaw = slash === -1 ? undefined : value.slice(slash + 1);
  let prefix: number | undefined;
  if (prefixRaw !== undefined) {
    if (!/^\d+$/.test(prefixRaw)) {
      return false;
    }
    prefix = Number(prefixRaw);
  }
  const v4 = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(address);
  if (v4 !== null) {
    for (let index = 1; index <= 4; index += 1) {
      if (Number(v4[index]) > 255) {
        return false;
      }
    }
    return prefix === undefined || prefix <= 32;
  }
  if (!/^[0-9a-fA-F:]+$/.test(address) || !address.includes(":")) {
    return false;
  }
  if (address.split("::").length - 1 > 1) {
    return false;
  }
  return prefix === undefined || prefix <= 128;
}

const addressValid = computed(() => isValidCidr(form.address));

const rate = reactive<RateLimitConfig>({
  enabled: false,
  requests_per_minute: 600,
  burst: 100,
});
const rateLoading = ref(true);
const rateSaving = ref(false);
const rateMessage = ref("");
const rateOk = ref(true);

async function load(): Promise<void> {
  loading.value = true;
  error.value = false;
  try {
    list.value = await api.accessLists();
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadRate(): Promise<void> {
  rateLoading.value = true;
  try {
    const value = await api.rateLimit();
    rate.enabled = value.enabled;
    rate.requests_per_minute = value.requests_per_minute;
    rate.burst = value.burst;
  } catch {
    rateMessage.value = copy.access.rate.failed;
    rateOk.value = false;
  } finally {
    rateLoading.value = false;
  }
}

async function addEntry(): Promise<void> {
  adding.value = true;
  message.value = "";
  try {
    await api.createAccessEntry({
      action: form.action,
      address: form.address,
      note: form.note,
      enabled: form.enabled,
    });
    form.address = "";
    form.note = "";
    form.enabled = true;
    messageOk.value = true;
    message.value = copy.access.entryAdded;
    await load();
  } catch (caught) {
    messageOk.value = false;
    message.value = caught instanceof ApiError ? caught.message : copy.access.loadFailed;
  } finally {
    adding.value = false;
  }
}

async function removeEntry(): Promise<void> {
  const entry = pendingDelete.value;
  pendingDelete.value = null;
  if (entry === null) {
    return;
  }
  deleting.value = entry.id;
  message.value = "";
  try {
    await api.deleteAccessEntry(entry.id);
    messageOk.value = true;
    message.value = copy.access.entryDeleted;
    await load();
  } catch {
    messageOk.value = false;
    message.value = copy.access.loadFailed;
  } finally {
    deleting.value = "";
  }
}

async function saveRate(): Promise<void> {
  rateSaving.value = true;
  rateMessage.value = "";
  try {
    const saved = await api.saveRateLimit({
      enabled: rate.enabled,
      requests_per_minute: Number(rate.requests_per_minute),
      burst: Number(rate.burst),
    });
    rate.requests_per_minute = saved.requests_per_minute;
    rate.burst = saved.burst;
    rateOk.value = true;
    rateMessage.value = copy.access.rate.saved;
  } catch (caught) {
    rateOk.value = false;
    rateMessage.value = caught instanceof ApiError ? caught.message : copy.access.rate.failed;
  } finally {
    rateSaving.value = false;
  }
}

onMounted(() => {
  void load();
  void loadRate();
});
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.access.title }}</h1>
        <p class="page-subtitle">{{ copy.access.subtitle }}</p>
        <p class="seed-note">{{ copy.access.seedNote }}</p>
      </div>
    </header>

    <p v-if="message !== ''" :class="messageOk ? 'action-ok' : 'action-error'" role="status">{{ message }}</p>

    <section class="panel">
      <div class="panel-header">
        <h2 class="panel-title">{{ copy.access.entries }}</h2>
        <span class="badge" :class="list?.allow_only ? 'badge-down' : 'badge-unknown'">
          {{ copy.access.allowOnly }}: {{ list?.allow_only ? copy.common.yes : copy.common.no }}
        </span>
      </div>
      <div class="panel-body stack">
        <p class="muted">
          {{ list?.allow_only ? copy.access.allowOnlyOn : copy.access.allowOnlyOff }}
        </p>
        <form class="entry-form" @submit.prevent="addEntry">
          <div class="field">
            <span>{{ copy.access.form.action }}</span>
            <Select
              :model-value="form.action"
              :options="[{ value: 'deny', label: 'deny' }, { value: 'allow', label: 'allow' }]"
              :label="copy.access.form.action"
              @update:model-value="form.action = $event as 'allow' | 'deny'"
            />
          </div>
          <label class="field">
            <span>{{ copy.access.form.address }}</span>
            <input
              v-model="form.address"
              class="input mono"
              required
              placeholder="203.0.113.0/24 or 2001:db8::/32"
              :aria-invalid="form.address !== '' && !addressValid"
            />
            <span v-if="form.address !== '' && !addressValid" class="field-error">{{ copy.access.invalidCidr }}</span>
          </label>
          <label class="field">
            <span>{{ copy.access.form.note }}</span>
            <input v-model="form.note" class="input" maxlength="200" />
          </label>
          <label class="check-row">
            <input v-model="form.enabled" type="checkbox" />
            <span>{{ copy.access.form.enabled }}</span>
          </label>
          <button type="submit" class="btn btn-primary" :disabled="adding || form.address === '' || !addressValid">
            {{ adding ? copy.access.form.submitting : copy.access.form.submit }}
          </button>
        </form>

        <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
        <StateMessage v-else-if="error" kind="error" :message="copy.access.loadFailed" @retry="load" />
        <StateMessage v-else-if="entries.length === 0" kind="empty" :message="copy.access.empty" />
        <div v-else class="table-scroll">
          <table>
            <thead>
              <tr>
                <th scope="col">{{ copy.access.table.id }}</th>
                <th scope="col">{{ copy.access.table.action }}</th>
                <th scope="col">{{ copy.access.table.address }}</th>
                <th scope="col">{{ copy.access.table.note }}</th>
                <th scope="col">{{ copy.access.table.enabled }}</th>
                <th scope="col">{{ copy.access.table.created }}</th>
                <th scope="col">{{ copy.access.table.source }}</th>
                <th scope="col">{{ copy.rules.table.actions }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in entries" :key="entry.id">
                <td class="mono wrap">{{ entry.id }}</td>
                <td>
                  <span :class="entry.action === 'deny' ? 'badge badge-block' : 'badge badge-ok'">
                    {{ entry.action }}
                  </span>
                </td>
                <td class="mono wrap">{{ entry.address }}</td>
                <td class="wrap">{{ entry.note }}</td>
                <td class="mono">{{ entry.enabled ? copy.common.yes : copy.common.no }}</td>
                <td class="mono">{{ entry.created_at ? formatDayTime(entry.created_at) : "—" }}</td>
                <td class="mono">
                  <RouterLink v-if="entry.source_log_id" :to="{ name: 'logs', query: { log: String(entry.source_log_id) } }">
                    #{{ entry.source_log_id }}
                  </RouterLink>
                  <span v-else>{{ copy.reserved.none }}</span>
                </td>
                <td>
                  <button
                    type="button"
                    class="btn btn-small btn-danger"
                    :disabled="deleting === entry.id"
                    @click="pendingDelete = entry"
                  >
                    {{ copy.access.delete }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-header">
        <h2 class="panel-title">{{ copy.access.rate.title }}</h2>
      </div>
      <form class="panel-body rate-form" @submit.prevent="saveRate">
        <p class="muted">{{ copy.access.rate.note }}</p>
        <StateMessage v-if="rateLoading" kind="loading" :message="copy.common.loading" />
        <template v-else>
          <label class="check-row">
            <input v-model="rate.enabled" type="checkbox" />
            <span>{{ copy.access.rate.enabled }}</span>
          </label>
          <label class="field">
            <span>{{ copy.access.rate.rpm }}</span>
            <input v-model.number="rate.requests_per_minute" class="input" type="number" min="1" required />
          </label>
          <label class="field">
            <span>{{ copy.access.rate.burst }}</span>
            <input v-model.number="rate.burst" class="input" type="number" min="1" required />
          </label>
          <div class="row">
            <button type="submit" class="btn btn-primary" :disabled="rateSaving">
              {{ rateSaving ? copy.access.rate.saving : copy.access.rate.save }}
            </button>
            <span v-if="rateMessage !== ''" :class="rateOk ? 'action-ok' : 'action-error'">{{ rateMessage }}</span>
          </div>
        </template>
      </form>
    </section>

    <ConfirmDialog
      :open="pendingDelete !== null"
      :title="copy.access.delete"
      :message="`${copy.access.confirmDelete} (${pendingDelete?.address ?? ''})`"
      :confirm-label="copy.access.delete"
      danger
      @confirm="removeEntry"
      @cancel="pendingDelete = null"
    />
  </div>
</template>

<style scoped>
.seed-note {
  margin: var(--sp-4) 0 0;
  max-width: 560px;
  color: var(--text-faint);
  font-size: var(--fs-12);
}

.entry-form {
  display: grid;
  grid-template-columns: 120px minmax(180px, 1.4fr) minmax(160px, 1fr) max-content max-content;
  gap: var(--sp-12);
  align-items: end;
}

.entry-form .input {
  height: 38px;
}

.entry-form :deep(.select-box .select-button) {
  height: 38px;
  padding-block: 0;
}

.entry-form .check-row {
  height: 38px;
  padding-bottom: var(--sp-2);
}

.entry-form .btn {
  height: 38px;
}

.field span {
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.check-row {
  display: flex;
  align-items: center;
  gap: var(--sp-8);
  font-size: var(--fs-13);
  white-space: nowrap;
}

.rate-form {
  display: grid;
  grid-template-columns: max-content max-content max-content;
  gap: var(--sp-12);
  align-items: end;
  justify-content: start;
}

.rate-form .muted {
  grid-column: 1 / -1;
  margin: 0;
  font-size: var(--fs-13);
}

.rate-form .row {
  grid-column: 1 / -1;
}

.field-error {
  color: var(--blocked);
  font-size: var(--fs-12);
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
  .entry-form {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .entry-form,
  .rate-form {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
