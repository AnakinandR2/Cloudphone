<script setup lang="ts">
// Reflect the persisted accent on <html> for both SSR and client (no FOUC).
const accent = useCookie<string>('gp-accent', { default: () => 'green' })

useHead({
  htmlAttrs: {
    'data-accent': accent,
  },
})
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
