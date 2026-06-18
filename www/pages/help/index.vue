<script setup lang="ts">
// /help —— 重定向到目录第一篇文档。目录为空/出错时显示提示，不强跳。
const { t } = useGp()
const localePath = useLocalePath()

const { data: tree } = await useHelpDirectory()
const first = computed(() => firstArticleUrl(tree.value ?? []))

if (first.value) {
  await navigateTo(localePath(first.value), { redirectCode: 302 })
}

useHead({ title: () => `${t.value.nav.docs} — Gloryphone` })
</script>

<template>
  <div class="container" style="padding: 80px 0; text-align: center; color: rgb(var(--fg-muted))">
    <p>{{ t.blog.error }}</p>
    <NuxtLink :to="localePath('/')" class="btn btn-quiet btn-sm" style="margin-top: 14px">{{ t.nav.home }}</NuxtLink>
  </div>
</template>
