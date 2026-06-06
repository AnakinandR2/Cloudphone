<script setup lang="ts">
import { RouterView } from 'vue-router'
import ProgressBar from '@/components/ProgressBar.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import { Toaster } from '@/components/ui/sonner'
</script>

<template>
  <ProgressBar />
  <RouterView />
  <Toaster rich-colors position="top-right" close-button />
  <SettingsPanel />
</template>
