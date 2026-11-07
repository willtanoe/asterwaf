<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { api, ApiError } from "../api/client";
import type { Rule, RuleMatch, RuleVersion } from "../api/types";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import Drawer from "../components/Drawer.vue";
import Select from "../components/Select.vue";
import StateMessage from "../components/StateMessage.vue";
import { copy } from "../copy";
import { formatDayTime } from "../format";

interface RuleDraft {
  id: string;
  name: string;
  category: string;
  pattern: string;
  enabled: boolean;
  score: number;
}

const categories = ["sqli", "xss", "traversal", "rce", "scanner"];
const targetCategories = ["query", "header", "path", "cookie", "form", "json", "body"];

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
const target = ref("query");
const testing = ref(false);
const testMessage = ref("");
const testOk = ref(false);
const testMatches = ref<RuleMatch[]>([]);
const draftMatch = ref<string | null>(null);

const versions = ref<RuleVersion[]>([]);
const versionsLoading = ref(true);
const versionsError = ref(false);
const rollingBack = ref(false);

const pendingDelete = ref<Rule | null>(null);
const pendingRollback = ref<RuleVersion | null>(null);

const editor = reactive<RuleDraft>({
  id: "",
  name: "",
  category: "sqli",
  pattern: "",
  enabled: true,
  score: 0,
});

const byteLength = (value: string): number => new TextEncoder().encode(value).length;

const idValid = computed(() => /^[a-z0-9._-]+$/.test(editor.id) && byteLength(editor.id) <= 64);
const nameValid = computed(
  () => editor.name.trim() !== "" && byteLength(editor.name) <= 120,
);
const patternBytes = computed(() => byteLength(editor.pattern));
const patternValid = computed(
  () => editor.pattern !== "" && patternBytes.value <= 1024 && patternRegexError.value === "",
);
const patternRegexError = computed(() => {
  if (editor.pattern === "") {
    return "";
  }
  try {
    new RegExp(editor.pattern);
    return "";
  } catch {
    return copy.rules.editor.invalidRegex;
  }
});

const formValid = computed(() => idValid.value && nameValid.value && patternValid.value);

const targetOptions = computed(() =>
  targetCategories.map((category) => ({ value: category, label: category })),
);

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
  testMatches.value = [];
  draftMatch.value = null;
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
  testMatches.value = [];
  draftMatch.value = null;
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
  if (draft.value === null || !formValid.value) {
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

async function remove(): Promise<void> {
  const rule = pendingDelete.value;
  pendingDelete.value = null;
  if (rule === null) {
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
  if (draft.value === null || payload.value === "") {
    return;
  }
  testing.value = true;
  testMessage.value = "";
  testMatches.value = [];
  draftMatch.value = null;
  testOk.value = false;
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
      "all",
      target.value,
    );
    testMatches.value = result.matches ?? [];
    testOk.value = result.matched;
    testMessage.value = result.matched ? copy.rules.editor.fired : copy.rules.editor.noFired;
    // The saved ruleset cannot contain the unsaved draft; check it client-side
    // so the author sees what saving would change.
    try {
      const match = new RegExp(editor.pattern).exec(payload.value);
      draftMatch.value = match?.[0] ?? null;
    } catch {
      draftMatch.value = null;
    }
  } catch (caught) {
    testOk.value = false;
    testMessage.value = caught instanceof ApiError ? caught.message : copy.rules.editor.saveFailed;
  } finally {
    testing.value = false;
  }
}

async function rollback(): Promise<void> {
  const item = pendingRollback.value;
  pendingRollback.value = null;
  if (item === null) {
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

    <div class="panel">
      <StateMessage v-if="loading" kind="loading" :message="copy.common.loading" />
      <StateMessage v-else-if="error" kind="error" :message="copy.rules.loadFailed" @retry="load" />
      <StateMessage v-else-if="rules.length === 0" kind="empty" :message="copy.rules.empty" />
      <div v-else class="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">{{ copy.rules.table.id }}</th>
              <th scope="col">{{ copy.rules.table.category }}</th>
              <th scope="col">{{ copy.rules.table.enabled }}</th>
              <th scope="col">{{ copy.rules.table.source }}</th>
              <th scope="col">{{ copy.rules.table.updated }}</th>
              <th scope="col" class="numeric">{{ copy.rules.table.version }}</th>
              <th scope="col">{{ copy.rules.table.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.id">
              <td class="mono">
                <span class="cell-stack">
                  <span class="truncate block-cell" :title="`${rule.id} — ${rule.name}`">{{ rule.id }}</span>
                  <span class="rule-name truncate block-cell" :title="rule.name">{{ rule.name }}</span>
                </span>
              </td>
              <td class="mono">{{ rule.category }}</td>
              <td>
                <button
                  type="button"
                  class="switch"
                  role="switch"
                  :aria-checked="rule.enabled"
                  :aria-label="`${copy.rules.editor.toggleAria} ${rule.id}`"
                  @click="toggle(rule)"
                >
                  <span class="switch-knob" aria-hidden="true" />
                </button>
              </td>
              <td class="mono">{{ rule.source }}</td>
              <td class="mono">{{ formatDayTime(rule.updated_at) }}</td>
              <td class="numeric mono">v{{ rule.defaults_version }}</td>
              <td>
                <div class="row">
                  <button type="button" class="btn btn-small" @click="startEdit(rule)">
                    {{ copy.rules.edit }}
                  </button>
                  <button type="button" class="btn btn-small btn-danger" @click="pendingDelete = rule">
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
                <button type="button" class="btn btn-small" :disabled="rollingBack" @click="pendingRollback = item">
                  {{ copy.rules.versions.rollback }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <Drawer
      :open="draft !== null"
      :title="isNew ? copy.rules.editor.createTitle : copy.rules.editor.editTitle"
      @close="closeEditor"
    >
      <form class="editor" @submit.prevent="save">
        <label class="field">
          <span>{{ copy.rules.editor.id }}</span>
          <input
            v-model="editor.id"
            class="input mono"
            :disabled="!isNew"
            :aria-invalid="!idValid"
            required
          />
          <span v-if="!idValid" class="field-error">{{ copy.rules.editor.invalidId }}</span>
          <span v-else class="hint">{{ copy.rules.editor.idHint }}</span>
        </label>
        <label class="field">
          <span>{{ copy.rules.editor.name }}</span>
          <input v-model="editor.name" class="input" :aria-invalid="!nameValid" required />
          <span v-if="!nameValid" class="field-error">{{ copy.rules.editor.invalidName }}</span>
        </label>
        <div class="editor-grid">
          <div class="field">
            <span>{{ copy.rules.editor.category }}</span>
            <Select :model-value="editor.category" :options="categories.map((c) => ({ value: c, label: c }))" :label="copy.rules.editor.category" @update:model-value="editor.category = $event" />
          </div>
          <label class="field">
            <span>{{ copy.rules.editor.score }}</span>
            <input :value="editor.score" class="input" disabled />
            <span class="hint">{{ copy.rules.editor.scoreNote }}</span>
          </label>
        </div>
        <label class="field">
          <span>{{ copy.rules.editor.pattern }}</span>
          <textarea
            v-model="editor.pattern"
            class="input mono"
            rows="4"
            :aria-invalid="!patternValid"
            required
          />
          <span class="hint mono">{{ patternBytes }}/1024</span>
          <span v-if="patternRegexError !== ''" class="field-error">{{ patternRegexError }}</span>
          <span v-else-if="patternBytes > 1024" class="field-error">{{ copy.rules.editor.invalidPattern }}</span>
        </label>
        <label class="check-row">
          <input v-model="editor.enabled" type="checkbox" />
          <span>{{ copy.rules.editor.enabled }}</span>
        </label>
        <div class="row">
          <button type="submit" class="btn btn-primary" :disabled="saving || !formValid">
            {{ saving ? copy.rules.editor.saving : copy.rules.editor.save }}
          </button>
        </div>
        <p v-if="saveError !== ''" class="action-error" role="alert">{{ saveError }}</p>

        <div class="test-block">
          <h3 class="panel-title">{{ copy.rules.editor.test }}</h3>
          <p class="hint">{{ copy.rules.editor.runAll }}</p>
          <div class="editor-grid">
            <div class="field">
              <span>{{ copy.rules.editor.target }}</span>
              <Select :model-value="target" :options="targetOptions" :label="copy.rules.editor.target" @update:model-value="target = $event" />
            </div>
          </div>
          <label class="field">
            <span>{{ copy.rules.editor.payload }}</span>
            <textarea v-model="payload" class="input mono" rows="3" />
          </label>
          <div class="row">
            <button type="button" class="btn" :disabled="testing || payload === ''" @click="runTest">
              {{ testing ? copy.rules.editor.testing : copy.rules.editor.runTest }}
            </button>
            <span v-if="testMessage !== ''" :class="testOk ? 'action-ok' : 'action-error'">
              {{ testMessage }}
            </span>
          </div>
          <ul v-if="testMatches.length > 0" class="match-list">
            <li v-for="match in testMatches" :key="match.rule_id" class="match-row">
              <span class="mono">{{ match.rule_id }}</span>
              <span class="mono muted">{{ match.category }}</span>
              <code class="match-fragment">{{ match.match }}</code>
            </li>
          </ul>
          <p v-if="draftMatch !== null" class="hint">
            {{ copy.rules.editor.draftCheck }}: <code class="match-fragment">{{ draftMatch }}</code>
          </p>
        </div>
      </form>
    </Drawer>

    <ConfirmDialog
      :open="pendingDelete !== null"
      :title="copy.rules.delete"
      :message="copy.rules.confirmDelete"
      :confirm-label="copy.rules.delete"
      danger
      @confirm="remove"
      @cancel="pendingDelete = null"
    />

    <ConfirmDialog
      :open="pendingRollback !== null"
      :title="copy.rules.versions.rollback"
      :message="copy.rules.versions.confirmRollback"
      :confirm-label="copy.rules.versions.rollback"
      @confirm="rollback"
      @cancel="pendingRollback = null"
    />
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

.field-error {
  color: var(--blocked);
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

.match-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
}

.match-row {
  display: flex;
  align-items: center;
  gap: var(--sp-8);
  flex-wrap: wrap;
  font-size: var(--fs-12);
}

.match-fragment {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0 var(--sp-4);
  overflow-wrap: anywhere;
}

.cell-stack {
  display: flex;
  flex-direction: column;
  gap: 0;
  min-width: 0;
}

.rule-name {
  color: var(--text-dim);
  font-size: var(--fs-12);
  max-width: 220px;
}

.block-cell {
  display: block;
  max-width: 220px;
}

.switch {
  position: relative;
  width: 36px;
  height: 20px;
  border-radius: 10px;
  border: 1px solid var(--border-strong);
  background: var(--bg-raised);
  padding: 0;
}

.switch[aria-checked="true"] {
  background: var(--ok-soft);
  border-color: var(--ok);
}

.switch-knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--text-dim);
  transition: transform 0.15s ease;
}

.switch[aria-checked="true"] .switch-knob {
  transform: translateX(16px);
  background: var(--ok);
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
