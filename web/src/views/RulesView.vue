<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { api, ApiError } from "../api/client";
import type { Rule, RuleVersion } from "../api/types";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { formatDayTime, formatNumber } from "../format";

interface RuleDraft {
  id: string;
  name: string;
  category: string;
  pattern: string;
  enabled: boolean;
  score: number;
}

const categories = ["sqli", "xss", "traversal", "rce", "scanner"];

const rules = ref<Rule[]>([]);
const version = ref(0);
const loading = ref(true);
const error = ref(false);
const saving = ref(false);
const saveError = ref("");
const message = ref("");
const messageOk = ref(true);

const draft = ref<RuleDraft | null>(null);
const isNew = ref(true);
const payload = ref("");
const testing = ref(false);
const testMessage = ref("");
const testMatched = ref(false);

const versions = ref<RuleVersion[]>([]);
const versionsLoading = ref(true);
const versionsError = ref(false);
const rollingBack = ref(false);

const editor = reactive<RuleDraft>({
  id: "",
  name: "",
  category: "sqli",
  pattern: "",
  enabled: true,
  score: 0,
});

function startCreate(): void {
  isNew.value = true;
  editor.id = "";
  editor.name = "";
  editor.category = "sqli";
  editor.pattern = "";
  editor.enabled = true;
  editor.score = 0;
  draft.value = editor;
  payload.value = "";
  testMessage.value = "";
  saveError.value = "";
  message.value = "";
}

function startEdit(rule: Rule): void {
  isNew.value = false;
  editor.id = rule.id;
  editor.name = rule.name;
  editor.category = rule.category;
  editor.pattern = rule.pattern;
  editor.enabled = rule.enabled;
  editor.score = rule.score;
  draft.value = editor;
  payload.value = "";
  testMessage.value = "";
  saveError.value = "";
  message.value = "";
}

function closeEditor(): void {
  draft.value = null;
  saveError.value = "";
  testMessage.value = "";
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = false;
  versionsLoading.value = true;
  versionsError.value = false;
  try {
    const response = await api.rules();
    rules.value = response.rules;
    version.value = response.version;
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
  try {
    versions.value = (await api.ruleVersions()).versions;
  } catch {
    versionsError.value = true;
  } finally {
    versionsLoading.value = false;
  }
}

async function save(): Promise<void> {
  if (draft.value === null) {
    return;
  }
  saving.value = true;
  saveError.value = "";
  try {
    const body = {
      name: editor.name,
      category: editor.category,
      pattern: editor.pattern,
      enabled: editor.enabled,
      score: editor.score,
    };
    if (isNew.value) {
      await api.createRule({ ...body, id: editor.id });
    } else {
      await api.updateRule(editor.id, body);
    }
    closeEditor();
    await load();
  } catch (caught) {
    saveError.value = caught instanceof ApiError ? caught.message : copy.rules.editor.saveFailed;
  } finally {
    saving.value = false;
  }
}

async function toggle(rule: Rule): Promise<void> {
  message.value = "";
  try {
    await api.toggleRule(rule.id);
    await load();
  } catch {
    messageOk.value = false;
    message.value = copy.rules.loadFailed;
  }
}

async function remove(rule: Rule): Promise<void> {
  if (!window.confirm(copy.rules.confirmDelete)) {
    return;
  }
  message.value = "";
  try {
    await api.deleteRule(rule.id);
    await load();
  } catch {
    messageOk.value = false;
    message.value = copy.rules.loadFailed;
  }
}

async function runTest(): Promise<void> {
  if (draft.value === null) {
    return;
  }
  testing.value = true;
  testMessage.value = "";
  try {
    const result = await api.testRule(
      {
        id: editor.id,
        name: editor.name,
        category: editor.category,
        pattern: editor.pattern,
        score: editor.score,
      },
      payload.value,
    );
    testMatched.value = result.matched;
    testMessage.value = result.matched ? copy.rules.editor.matched : copy.rules.editor.notMatched;
  } catch (caught) {
    testMatched.value = false;
    testMessage.value = caught instanceof ApiError ? caught.message : copy.rules.editor.saveFailed;
  } finally {
    testing.value = false;
  }
}

async function rollback(item: RuleVersion): Promise<void> {
  if (!window.confirm(copy.rules.versions.confirmRollback)) {
    return;
  }
  rollingBack.value = true;
  message.value = "";
  try {
    await api.rollbackRuleVersion(item.id);
    messageOk.value = true;
    message.value = copy.rules.versions.rolledBack;
    await load();
  } catch {
    messageOk.value = false;
    message.value = copy.rules.loadFailed;
  } finally {
    rollingBack.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.rules.title }}</h1>
        <p class="page-subtitle">{{ copy.rules.subtitle }}</p>
      </div>
      <div class="row">
        <span class="badge badge-allow mono">v{{ version }}</span>
        <button type="button" class="btn btn-primary" @click="startCreate">{{ copy.rules.add }}</button>
      </div>
    </header>

    <p v-if="message !== ''" :class="messageOk ? 'action-ok' : 'action-error'" role="status">{{ message }}</p>

    <section v-if="draft !== null" class="panel">
      <div class="panel-header">
        <h2 class="panel-title">
          {{ isNew ? copy.rules.editor.createTitle : copy.rules.editor.editTitle }}
        </h2>
        <button type="button" class="btn btn-small" @click="closeEditor">
          {{ copy.rules.editor.cancel }}
        </button>
      </div>
      <form class="panel-body editor" @submit.prevent="save">
        <div class="editor-grid">
          <label class="field">
            <span>{{ copy.rules.editor.id }}</span>
            <input
              v-model="editor.id"
              class="input"
              :disabled="!isNew"
              maxlength="64"
              pattern="[a-z0-9._-]+"
              required
            />
            <span class="hint">{{ copy.rules.editor.idHint }}</span>
          </label>
          <label class="field">
            <span>{{ copy.rules.editor.name }}</span>
            <input v-model="editor.name" class="input" maxlength="120" required />
          </label>
          <label class="field">
            <span>{{ copy.rules.editor.category }}</span>
            <select v-model="editor.category" class="select">
              <option v-for="category in categories" :key="category" :value="category">{{ category }}</option>
            </select>
          </label>
          <label class="field">
            <span>{{ copy.rules.editor.score }}</span>
            <input :value="editor.score" class="input" disabled />
            <span class="hint">{{ copy.rules.editor.scoreNote }}</span>
          </label>
        </div>
        <label class="field">
          <span>{{ copy.rules.editor.pattern }}</span>
          <textarea v-model="editor.pattern" class="input mono" rows="4" maxlength="1024" required />
        </label>
        <label class="check-row">
          <input v-model="editor.enabled" type="checkbox" />
          <span>{{ copy.rules.editor.enabled }}</span>
        </label>
        <div class="row">
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ saving ? copy.rules.editor.saving : copy.rules.editor.save }}
          </button>
        </div>
        <p v-if="saveError !== ''" class="action-error" role="alert">{{ saveError }}</p>

        <div class="test-block">
          <h3 class="panel-title">{{ copy.rules.editor.test }}</h3>
          <label class="field">
            <span>{{ copy.rules.editor.payload }}</span>
            <textarea v-model="payload" class="input mono" rows="3" />
          </label>
          <div class="row">
            <button type="button" class="btn" :disabled="testing || payload === ''" @click="runTest">
              {{ testing ? copy.rules.editor.testing : copy.rules.editor.runTest }}
            </button>
            <span v-if="testMessage !== ''" :class="testMatched ? 'action-ok' : 'action-error'">
              {{ testMessage }}
            </span>
          </div>
        </div>
      </form>
    </section>

    <div class="panel">
      <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="error" kind="error" :message="copy.rules.loadFailed" @retry="load" />
      <StateMessage v-else-if="rules.length === 0" kind="empty" :message="copy.rules.empty" />
      <div v-else class="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">{{ copy.rules.table.id }}</th>
              <th scope="col">{{ copy.rules.table.name }}</th>
              <th scope="col">{{ copy.rules.table.category }}</th>
              <th scope="col">{{ copy.rules.table.pattern }}</th>
              <th scope="col">{{ copy.rules.table.enabled }}</th>
              <th scope="col">{{ copy.rules.table.source }}</th>
              <th scope="col" class="numeric">{{ copy.rules.table.score }}</th>
              <th scope="col">{{ copy.rules.table.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.id">
              <td class="mono wrap">{{ rule.id }}</td>
              <td class="wrap">{{ rule.name }}</td>
              <td class="mono">{{ rule.category }}</td>
              <td class="mono pattern-cell" :title="rule.pattern">{{ rule.pattern }}</td>
              <td>
                <span :class="rule.enabled ? 'badge badge-ok' : 'badge badge-unknown'">
                  {{ rule.enabled ? copy.common.yes : copy.common.no }}
                </span>
              </td>
              <td class="mono">{{ rule.source }}</td>
              <td class="numeric mono">{{ formatNumber(rule.score) }}</td>
              <td>
                <div class="row">
                  <button type="button" class="btn btn-small" @click="startEdit(rule)">
                    {{ copy.rules.edit }}
                  </button>
                  <button type="button" class="btn btn-small" @click="toggle(rule)">
                    {{ rule.enabled ? copy.rules.toggleOff : copy.rules.toggleOn }}
                  </button>
                  <button type="button" class="btn btn-small btn-danger" @click="remove(rule)">
                    {{ copy.rules.delete }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <section class="panel">
      <div class="panel-header">
        <h2 class="panel-title">{{ copy.rules.versions.title }}</h2>
      </div>
      <StateMessage v-if="versionsLoading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="versionsError" kind="error" :message="copy.common.errorTitle" @retry="load" />
      <StateMessage v-else-if="versions.length === 0" kind="empty" :message="copy.rules.versions.empty" />
      <div v-else class="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col" class="numeric">{{ copy.rules.versions.id }}</th>
              <th scope="col">{{ copy.rules.versions.created }}</th>
              <th scope="col">{{ copy.rules.versions.actor }}</th>
              <th scope="col">{{ copy.rules.versions.note }}</th>
              <th scope="col" class="numeric">{{ copy.rules.versions.count }}</th>
              <th scope="col">{{ copy.rules.table.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in versions" :key="item.id">
              <td class="numeric mono">{{ item.id }}</td>
              <td class="mono">{{ formatDayTime(item.created_at) }}</td>
              <td class="mono">{{ item.actor }}</td>
              <td class="wrap">{{ item.note }}</td>
              <td class="numeric mono">{{ item.rule_count }}</td>
              <td>
                <button type="button" class="btn btn-small" :disabled="rollingBack" @click="rollback(item)">
                  {{ copy.rules.versions.rollback }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: var(--sp-12);
}

.editor-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--sp-12);
}

.field span {
  color: var(--text-dim);
  font-size: var(--fs-12);
}

.hint {
  color: var(--text-faint);
  font-size: var(--fs-12);
}

.check-row {
  display: flex;
  align-items: center;
  gap: var(--sp-8);
  font-size: var(--fs-13);
}

.test-block {
  display: flex;
  flex-direction: column;
  gap: var(--sp-8);
  padding-top: var(--sp-12);
  border-top: 1px solid var(--border);
}

.pattern-cell {
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

@media (max-width: 640px) {
  .editor-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
