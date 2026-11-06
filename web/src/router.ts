import { createRouter, createWebHistory } from "vue-router";
import { auth, ensureAuth } from "./auth";
import AccessListsView from "./views/AccessListsView.vue";
import LoginView from "./views/LoginView.vue";
import LogsView from "./views/LogsView.vue";
import OverviewView from "./views/OverviewView.vue";
import RulesView from "./views/RulesView.vue";
import SitesView from "./views/SitesView.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginView },
    { path: "/", name: "overview", component: OverviewView },
    { path: "/logs", name: "logs", component: LogsView },
    { path: "/rules", name: "rules", component: RulesView },
    { path: "/access-lists", name: "access-lists", component: AccessListsView },
    { path: "/sites", name: "sites", component: SitesView },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
});

router.beforeEach(async (to) => {
  if (to.name === "login") {
    if (auth.user !== null) {
      return { name: "overview" };
    }
    return true;
  }
  const authenticated = await ensureAuth();
  if (!authenticated) {
    return { name: "login", query: { next: to.fullPath } };
  }
  return true;
});
