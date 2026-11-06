<script setup lang="ts">
import { copy } from "../copy";
import StateMessage from "../components/StateMessage.vue";
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ copy.pages.logs.title }}</h1>
        <p class="page-subtitle">{{ copy.pages.logs.subtitle }}</p>
      </div>
    </header>
    <div class="panel">
      <StateMessage kind="empty" :message="copy.pages.pending" />
    </div>
  </div>
</template>
