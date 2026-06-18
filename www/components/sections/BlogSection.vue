<script setup lang="ts">
// 首页博客模块：取中台最新 3 篇。中台不可用 / 无内容时仅渲染区块标题，不崩溃。
const { t } = useGp()
const { locale } = useI18n()
const localePath = useLocalePath()
const { list } = useBlogPosts({ size: 3, sort: 'published_desc' })
const posts = computed(() => list.value.slice(0, 3))
const fmt = (iso: string) => formatBlogDate(iso, locale.value)
</script>

<template>
  <section class="section" id="blog">
    <div class="container">
      <header style="display: flex; justify-content: space-between; align-items: flex-end; flex-wrap: wrap; gap: 16px">
        <div>
          <span class="eyebrow">{{ t.blog.eyebrow }}</span>
          <h2 class="section-title">{{ t.blog.title }}</h2>
          <p class="section-sub" style="max-width: 640px">{{ t.blog.sub }}</p>
        </div>
        <NuxtLink :to="localePath('/blog')" class="btn btn-ghost btn-sm">{{ t.blog.all }} <GpIcon name="arrow" /></NuxtLink>
      </header>
      <div v-if="posts.length" class="blog-grid">
        <NuxtLink v-for="p in posts" :key="p.id" :to="localePath('/blog/' + p.slug)" class="blog-card">
          <div class="blog-card__cover">
            <img :src="p.cover_url" :alt="p.cover_alt" loading="lazy" />
            <span v-if="p.category" class="blog-card__cat">{{ p.category.name }}</span>
          </div>
          <div class="blog-card__body">
            <div class="blog-card__meta">
              <span>{{ fmt(p.published_at) }}</span>
            </div>
            <h3 class="blog-card__title">{{ p.title }}</h3>
            <p class="blog-card__excerpt">{{ p.summary }}</p>
            <span class="blog-card__more">{{ t.blog.readMore }} <GpIcon name="arrow" /></span>
          </div>
        </NuxtLink>
      </div>
    </div>
  </section>
</template>
