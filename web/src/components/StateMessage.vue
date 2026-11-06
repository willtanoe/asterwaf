<script setup lang="ts">
import { copy } from "../copy";

defineProps<{
  kind: "loading" | "empty" | "error";
  message: string;
}>();

defineEmits<{ retry: [] }>();
</script>

<template>
  <div v-if="kind === 'loading'" class="state" aria-live="polite">{{ message }}</div>
  <div v-else-if="kind === 'error'" class="state state-error">
    <span>{{ message }}</span>
    <span>{{ copy.common.errorHint }}</span>
    <button type="button" class="btn btn-small" @click="$emit('retry')">{{ copy.common.retry }}</button>
  </div>
  <div v-else class="state">{{ message }}</div>
</template>
