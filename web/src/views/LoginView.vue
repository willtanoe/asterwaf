<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError } from "../api/client";
import { auth, setUser } from "../auth";
import { copy } from "../copy";

const router = useRouter();
const route = useRoute();
const username = ref("");
const password = ref("");
const error = ref("");
const submitting = ref(false);

onMounted(() => {
  if (auth.user !== null) {
    void router.replace({ name: "overview" });
  }
});

async function submit(): Promise<void> {
  error.value = "";
  if (username.value.trim() === "" || password.value === "") {
    error.value = copy.login.required;
    return;
  }
  submitting.value = true;
  try {
    const response = await api.login(username.value.trim(), password.value);
    setUser(response);
    const next = typeof route.query.next === "string" ? route.query.next : "/";
    await router.replace(next.startsWith("/") ? next : "/");
  } catch (caught) {
    if (caught instanceof ApiError) {
      error.value = caught.code === "rate_limited" ? copy.login.rateLimited : caught.message;
    } else {
      error.value = copy.login.network;
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-card" aria-labelledby="login-title">
      <div class="brand">
        {{ copy.appName }}
        <span class="brand-subtitle">{{ copy.appSubtitle }}</span>
      </div>
      <h1 id="login-title" class="page-title">{{ copy.login.title }}</h1>
      <p class="page-subtitle">{{ copy.login.subtitle }}</p>
      <form class="stack" @submit.prevent="submit">
        <div class="field">
          <label for="username">{{ copy.login.username }}</label>
          <input
            id="username"
            v-model="username"
            class="input"
            name="username"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            required
          />
        </div>
        <div class="field">
          <label for="password">{{ copy.login.password }}</label>
          <input
            id="password"
            v-model="password"
            class="input"
            type="password"
            name="password"
            autocomplete="current-password"
            required
          />
        </div>
        <p v-if="error !== ''" class="login-error" role="alert">{{ error }}</p>
        <button type="submit" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? copy.login.submitting : copy.login.submit }}
        </button>
      </form>
    </section>
  </main>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: var(--sp-16);
  background: var(--bg);
}

.login-card {
  width: 100%;
  max-width: 380px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--sp-24);
  display: flex;
  flex-direction: column;
  gap: var(--sp-16);
}

.login-card .page-title {
  font-size: var(--fs-20);
}

.login-error {
  margin: 0;
  color: var(--blocked);
  font-size: var(--fs-13);
}
</style>
