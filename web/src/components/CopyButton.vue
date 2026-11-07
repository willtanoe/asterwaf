<script setup lang="ts">
import { ref } from "vue";
import { copy } from "../copy";

const props = defineProps<{
  value: string;
  /** What is being copied, for the accessible name. */
  subject: string;
}>();

const copied = ref(false);
let timer = 0;

async function copyValue(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.value);
    copied.value = true;
    window.clearTimeout(timer);
    timer = window.setTimeout(() => {
      copied.value = false;
    }, 1500);
  } catch {
    // Clipboard dapat ditolak oleh izin browser; biarkan tombol diam.
  }
}
</script>

<template>
  <button
    type="button"
    class="copy-btn"
    :aria-label="`${copied ? copy.common.copied : copy.common.copy}: ${subject}`"
    @click.stop="copyValue"
  >
    {{ copied ? copy.common.copied : copy.common.copy }}
  </button>
</template>

<style scoped>
.copy-btn {
  flex: none;
  padding: 0 var(--sp-8);
  min-height: 22px;
  background: transparent;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius);
  color: var(--text-dim);
  font-size: var(--fs-12);
  font-variant-numeric: tabular-nums;
}

.copy-btn:hover {
  border-color: var(--accent);
  color: var(--text);
}
</style>
