<script setup lang="ts">
// /api-docs —— 用 Scalar 渲染内容中台列表「第一篇」API 文档。
const { t } = useGp()
const localePath = useLocalePath()

const { docs, pending, error, refresh } = useApiDocs()
const first = computed(() => docs.value[0])
const specUrl = computed(() => (first.value ? `/_content/api-docs/${first.value.slug}/spec` : ''))

useSeoMeta({
  title: () => `${first.value?.spec_title || first.value?.name || t.value.nav.apiDocs} — Gloryphone`,
})
// 文档页让 footer 紧贴正文（去掉全站 footer 的 40px 顶部留白），仅本页生效。
useHead({ bodyAttrs: { class: 'page-api-docs' } })
</script>

<template>
  <div class="api-docs-page">
    <div v-if="error" class="api-docs-state">
      <p>{{ t.blog.error }}</p>
      <button class="btn btn-quiet btn-sm" @click="refresh()">{{ t.blog.retry }}</button>
    </div>
    <div v-else-if="pending && !first" class="api-docs-state"><p>…</p></div>
    <div v-else-if="!first" class="api-docs-state"><p>{{ t.blog.empty }}</p></div>
    <ScalarDoc v-else :spec-url="specUrl" />
  </div>
</template>

<style scoped>
.api-docs-page { min-height: 60vh; }
.api-docs-state { text-align: center; padding: 80px 0; color: rgb(var(--fg-muted)); display: flex; flex-direction: column; align-items: center; gap: 14px; }
</style>
