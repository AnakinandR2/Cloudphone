<script setup lang="ts">
// 文章详情：渲染中台的 body_html，展示分类 / 日期 / 阅读时长 / 可点击标签 / 推荐。
const { t } = useGp()
const { locale } = useI18n()
const localePath = useLocalePath()
const route = useRoute()

const slug = computed(() => String(route.params.slug))

// 可 await：SSR 阶段即可拿到 error，404 透传为真实 404 页。
const { data: post, error } = await useBlogPost(slug)
if (error.value || !post.value) {
  throw createError({
    statusCode: error.value?.statusCode === 404 ? 404 : 502,
    statusMessage: error.value?.statusMessage || 'Post not found',
    fatal: true,
  })
}

// 同分类推荐，排除当前篇。
const { list: relatedRaw } = useBlogPosts(
  computed(() => ({ size: 4, categoryId: post.value?.category?.id, sort: 'published_desc' as const })),
)
const related = computed(() => relatedRaw.value.filter((p) => p.slug !== slug.value).slice(0, 3))

const fmt = (iso: string) => formatBlogDate(iso, locale.value)
const minutes = computed(() => readingMinutes(post.value?.body_html ?? ''))

const crumbs = computed(() => [
  { label: t.value.nav.home, to: localePath('/') },
  { label: t.value.blog.eyebrow, to: localePath('/blog') },
  { label: post.value!.title },
])

useSeoMeta({
  title: () => (post.value ? `${post.value.seo_title || post.value.title} — Gloryphone` : 'Gloryphone'),
  description: () => post.value?.seo_description || post.value?.summary || '',
  keywords: () => post.value?.seo_keywords || '',
  ogTitle: () => post.value?.title || '',
  ogDescription: () => post.value?.seo_description || post.value?.summary || '',
  ogImage: () => post.value?.cover_url || '',
  ogType: 'article',
})
</script>

<template>
  <article v-if="post">
    <div class="section article-hero" style="padding-bottom: 0">
      <div class="container">
        <Breadcrumb :items="crumbs" />
        <div class="blog-feature__meta" style="margin-top: 18px">
          <span v-if="post.category" class="blog-feature__cat">{{ post.category.name }}</span>
          <span>{{ fmt(post.published_at) }}</span><span>·</span><span>{{ minutes }} {{ t.blog.min }}</span>
        </div>
        <div class="article-cover">
          <img :src="post.cover_url" :alt="post.cover_alt" loading="eager" />
        </div>
      </div>
    </div>

    <div class="article-body">
      <div class="container">
        <h1>{{ post.title }}</h1>
        <ArticleBody :html="post.body_html" />

        <div v-if="post.tags.length" class="article-tags">
          <span class="article-tags__label">{{ t.blog.tags }}:</span>
          <NuxtLink
            v-for="tg in post.tags"
            :key="tg.id"
            :to="localePath('/blog-tags/' + tg.slug)"
            class="article-tag"
          >
            #{{ tg.name }}
          </NuxtLink>
        </div>
      </div>
    </div>

    <section v-if="related.length" class="section" style="padding-top: 0">
      <div class="container">
        <h2 class="section-title" style="font-size: 24px; margin-bottom: 8px">{{ t.blog.title }}</h2>
        <div class="blog-grid">
          <NuxtLink v-for="rp in related" :key="rp.id" :to="localePath('/blog/' + rp.slug)" class="blog-card">
            <div class="blog-card__cover">
              <img :src="rp.cover_url" :alt="rp.cover_alt" loading="lazy" />
              <span v-if="rp.category" class="blog-card__cat">{{ rp.category.name }}</span>
            </div>
            <div class="blog-card__body">
              <div class="blog-card__meta">
                <span>{{ fmt(rp.published_at) }}</span>
              </div>
              <h3 class="blog-card__title">{{ rp.title }}</h3>
              <p class="blog-card__excerpt">{{ rp.summary }}</p>
              <span class="blog-card__more">{{ t.blog.readMore }} <GpIcon name="arrow" /></span>
            </div>
          </NuxtLink>
        </div>
      </div>
    </section>
  </article>
</template>

<style scoped>
.article-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-top: 36px; }
.article-tags__label { font-size: 12.5px; color: rgb(var(--fg-muted)); font-family: var(--font-mono); }
.article-tag {
  padding: 5px 12px; border-radius: 999px; font-size: 12.5px; font-weight: 600;
  background: rgb(var(--accent) / 0.1); color: var(--accent-color); transition: background 0.15s ease;
}
.article-tag:hover { background: rgb(var(--accent) / 0.2); }
</style>
