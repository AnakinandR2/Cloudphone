<script setup lang="ts">
// Reflect the persisted accent on <html> for both SSR and client (no FOUC).
const accent = useCookie<string>('gp-accent', { default: () => 'green' })

useHead({
  htmlAttrs: {
    'data-accent': accent,
  },
})

// 全站 SEO 配置（中台按 URL 路径统一注入 TDK / 任意 meta）。
useSeoConfig()
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>
