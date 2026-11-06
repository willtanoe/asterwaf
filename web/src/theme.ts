import { ref } from "vue";

export type Theme = "dark" | "light";

const storageKey = "asterwaf-theme";

function initialTheme(): Theme {
  const stored = window.localStorage.getItem(storageKey);
  if (stored === "dark" || stored === "light") {
    return stored;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export const theme = ref<Theme>(initialTheme());

export function applyTheme(): void {
  document.documentElement.dataset.theme = theme.value;
  window.localStorage.setItem(storageKey, theme.value);
}

export function toggleTheme(): void {
  theme.value = theme.value === "dark" ? "light" : "dark";
  applyTheme();
}

applyTheme();
