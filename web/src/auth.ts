import { reactive } from "vue";
import { api } from "./api/client";
import type { MeResponse } from "./api/types";

export const auth = reactive<{ user: MeResponse | null; checked: boolean }>({
  user: null,
  checked: false,
});

export async function ensureAuth(): Promise<boolean> {
  if (auth.checked) {
    return auth.user !== null;
  }
  try {
    auth.user = await api.me();
  } catch {
    auth.user = null;
  }
  auth.checked = true;
  return auth.user !== null;
}

export function setUser(user: MeResponse): void {
  auth.user = user;
  auth.checked = true;
}

export function clearUser(): void {
  auth.user = null;
  auth.checked = true;
}
