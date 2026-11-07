<script setup lang="ts">
import { useRouter } from "vue-router";
import { copy } from "../copy";
import { clearUser } from "../auth";
import { theme, toggleTheme } from "../theme";
import { api } from "../api/client";

const router = useRouter();

// Vue Router re-renders the route while the clicked link keeps focus, and the
// browser then treats that retained focus as keyboard-like, so the yellow ring
// shows after a mouse click. event.detail is 0 for keyboard activation, so
// only pointer clicks drop focus.
function onNavClick(event: MouseEvent): void {
  if (event.detail > 0 && event.currentTarget instanceof HTMLElement) {
    event.currentTarget.blur();
  }
}

async function signOut(): Promise<void> {
  try {
    await api.logout();
  } catch {
    // 会话可能已过期；仍要离开管理界面。
  }
  clearUser();
  await router.replace({ name: "login" });
}
</script>

<template>
  <a class="skip-link" href="#main">{{ copy.skipLink }}</a>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <img class="brand-logo" src="/aster.svg" alt="" width="36" height="36" />
        <span class="brand-text">
          {{ copy.appName }}
          <span class="brand-subtitle">{{ copy.appSubtitle }}</span>
        </span>
      </div>
      <nav class="nav" :aria-label="copy.nav.label">
        <RouterLink class="nav-link" :to="{ name: 'overview' }" @click="onNavClick">{{ copy.nav.overview }}</RouterLink>
        <RouterLink class="nav-link" :to="{ name: 'logs' }" @click="onNavClick">{{ copy.nav.logs }}</RouterLink>
        <RouterLink class="nav-link" :to="{ name: 'rules' }" @click="onNavClick">{{ copy.nav.rules }}</RouterLink>
        <RouterLink class="nav-link" :to="{ name: 'access-lists' }" @click="onNavClick">{{ copy.nav.accessLists }}</RouterLink>
        <RouterLink class="nav-link" :to="{ name: 'sites' }" @click="onNavClick">{{ copy.nav.sites }}</RouterLink>
      </nav>
      <div class="sidebar-footer">
        <button type="button" class="btn btn-small" @click="toggleTheme">
          <span class="theme-label">{{ copy.common.theme }}:</span>
          {{ theme === "dark" ? copy.common.themeDark : copy.common.themeLight }}
        </button>
        <button type="button" class="btn btn-small" @click="signOut">{{ copy.common.signOut }}</button>
      </div>
    </aside>
    <main id="main" class="main" tabindex="-1">
      <RouterView />
    </main>
  </div>
</template>
