<script setup lang="ts">
import { useRoute } from "vue-router";
import AppShell from "./components/AppShell.vue";

const route = useRoute();
</script>

<template>
  <AppShell v-if="route.name !== 'login'" />
  <RouterView v-else />
</template>
