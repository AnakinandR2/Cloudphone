<script setup lang="ts">
// /help/<slug> —— 文档三栏：左目录树 + 正文(注入锚点) + 右 TOC。
const route = useRoute()
const slug = computed(() => String(route.params.slug))

// 目录树（侧栏）与文章（正文）均可 await：SSR 即可拿到，404 透传。
const { data: tree } = await useHelpDirectory()
const { data: article, error } = await useHelpArticle(slug)
if (error.value || !article.value) {
  throw createError({
    statusCode: error.value?.statusCode === 404 ? 404 : 502,
    statusMessage: error.value?.statusMessage || 'Doc not found',
    fatal: true,
  })
}

const built = computed(() => buildToc(article.value?.body_html ?? ''))

useSeoMeta({
  title: () => (article.value ? `${article.value.seo_title || article.value.title} — Gloryphone` : 'Gloryphone'),
  description: () => article.value?.seo_description || article.value?.summary || '',
  ogTitle: () => article.value?.title || '',
})
</script>

<template>
  <DocsLayout v-if="article" :tree="tree ?? []" :current-slug="slug" :toc="built.toc">
    <article class="article-body docs-article">
      <h1>{{ article.title }}</h1>
      <ArticleBody :html="built.html" />
    </article>
  </DocsLayout>
</template>
