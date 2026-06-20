<script setup lang="ts">
// 博客列表（共享）：全部 / 分类 / 标签三种模式，伪静态路由驱动。
// 分类、标签 chips 与分页均为真实 <a> 链接。
import type { PubCategory, PubTaxon } from '~/types/content'

const props = defineProps<{
  mode: 'all' | 'category' | 'tag'
  slug?: string
  page: number
}>()

const { t } = useGp()
const { locale } = useI18n()
const localePath = useLocalePath()

const PAGE_SIZE = 12

// 先 await taxonomy：渲染 chips 并校验 slug。分类已改为按 slug 过滤（无需 id），
// 标签仍只认 tag_id，故 await 后用列表把标签 slug 解析成 id（避免 SSR 时序漏过滤）。
const { data: taxo } = await useBlogTaxonomy()
const categories = computed(() => taxo.value?.categories ?? [])
const tags = computed(() => taxo.value?.tags ?? [])

const activeCategory = computed<PubCategory | undefined>(() =>
  props.mode === 'category' ? categories.value.find((c) => c.slug === props.slug) : undefined,
)
const activeTag = computed<PubTaxon | undefined>(() =>
  props.mode === 'tag' ? tags.value.find((tg) => tg.slug === props.slug) : undefined,
)

// 无效的分类 / 标签 slug → 404
if ((props.mode === 'category' && !activeCategory.value) || (props.mode === 'tag' && !activeTag.value)) {
  throw createError({ statusCode: 404, statusMessage: 'Not found', fatal: true })
}

const { list, total, pending, error, refresh } = useBlogPosts(
  computed(() => ({
    page: props.page,
    size: PAGE_SIZE,
    group: activeCategory.value?.slug,
    tagId: activeTag.value?.id,
    sort: 'published_desc' as const,
  })),
)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const isFiltered = computed(() => props.mode !== 'all')
// 仅「全部」第一页才用首条做 featured 大图。
const featured = computed(() => (props.mode === 'all' && props.page === 1 ? list.value[0] : undefined))
const rest = computed(() => (featured.value ? list.value.slice(1) : list.value))

const fmt = (iso: string) => formatBlogDate(iso, locale.value)
const hrefFor = (p: number) => localePath(blogListPath(props.mode, props.slug, p))

const isCatActive = (slug: string) => props.mode === 'category' && props.slug === slug
const isTagActive = (slug: string) => props.mode === 'tag' && props.slug === slug

// 标题 / 副标题 / 面包屑
const heroTitle = computed(() => {
  if (props.mode === 'category') return activeCategory.value?.name ?? ''
  if (props.mode === 'tag') return '#' + (activeTag.value?.name ?? '')
  return t.value.blog.title
})
const pageSuffix = computed(() => (props.page > 1 ? ` (${props.page})` : ''))
useHead({
  title: () => {
    const base =
      props.mode === 'category'
        ? `${activeCategory.value?.name} — ${t.value.blog.eyebrow}`
        : props.mode === 'tag'
          ? `#${activeTag.value?.name} — ${t.value.blog.eyebrow}`
          : t.value.blog.eyebrow
    return `${base}${pageSuffix.value} — Gloryphone`
  },
})

const crumbs = computed(() => {
  const items: { label: string; to?: string }[] = [{ label: t.value.nav.home, to: localePath('/') }]
  if (isFiltered.value) {
    items.push({ label: t.value.blog.eyebrow, to: localePath('/blog') })
    items.push({ label: heroTitle.value })
  } else {
    items.push({ label: t.value.blog.eyebrow })
  }
  return items
})
</script>

<template>
  <div>
    <section class="blog-page-hero">
      <div class="container">
        <Breadcrumb :items="crumbs" style="margin-bottom: 18px" />
        <h1>{{ heroTitle }}</h1>
        <p v-if="!isFiltered">{{ t.blog.sub }}</p>

        <!-- 分类（真实 <a>） -->
        <div class="blog-page-tabs">
          <NuxtLink :to="localePath('/blog')" class="blog-page-tab" :class="{ active: mode === 'all' }">{{ t.blog.all }}</NuxtLink>
          <NuxtLink
            v-for="c in categories"
            :key="c.slug"
            :to="localePath('/blog-categories/' + c.slug)"
            class="blog-page-tab"
            :class="{ active: isCatActive(c.slug) }"
          >
            {{ c.name }}
          </NuxtLink>
        </div>

        <!-- 标签（真实 <a>） -->
        <div v-if="tags.length" class="blog-tags">
          <span class="blog-tags__label">{{ t.blog.tags }}:</span>
          <NuxtLink :to="localePath('/blog')" class="blog-tag" :class="{ active: mode !== 'tag' }">{{ t.blog.all }}</NuxtLink>
          <NuxtLink
            v-for="tg in tags"
            :key="tg.id"
            :to="localePath('/blog-tags/' + tg.slug)"
            class="blog-tag"
            :class="{ active: isTagActive(tg.slug) }"
          >
            #{{ tg.name }}
          </NuxtLink>
        </div>
      </div>
    </section>

    <section class="blog-page-grid">
      <div class="container">
        <div v-if="error" class="blog-state">
          <p>{{ t.blog.error }}</p>
          <button class="btn btn-quiet btn-sm" @click="refresh()">{{ t.blog.retry }}</button>
        </div>

        <div v-else-if="pending && !list.length" class="blog-state"><p>…</p></div>

        <div v-else-if="!list.length" class="blog-state"><p>{{ t.blog.empty }}</p></div>

        <template v-else>
          <NuxtLink v-if="featured" :to="localePath('/blog/' + featured.slug)" class="blog-feature">
            <div class="blog-feature__cover">
              <img :src="featured.cover_url" :alt="featured.cover_alt" loading="eager" />
            </div>
            <div>
              <div class="blog-feature__meta">
                <span v-if="featured.group" class="blog-feature__cat">{{ featured.group.name }}</span>
                <span>{{ fmt(featured.published_at) }}</span>
              </div>
              <h2>{{ featured.title }}</h2>
              <p>{{ featured.summary }}</p>
              <span class="btn btn-quiet btn-sm cta">{{ t.blog.readMore }} <GpIcon name="arrow-sm" style="width: 14px; height: 14px" /></span>
            </div>
          </NuxtLink>

          <div class="blog-grid">
            <NuxtLink v-for="p in rest" :key="p.id" :to="localePath('/blog/' + p.slug)" class="blog-card">
              <div class="blog-card__cover">
                <img :src="p.cover_url" :alt="p.cover_alt" loading="lazy" />
                <span v-if="p.group" class="blog-card__cat">{{ p.group.name }}</span>
              </div>
              <div class="blog-card__body">
                <div class="blog-card__meta">
                  <span>{{ fmt(p.published_at) }}</span>
                </div>
                <h3 class="blog-card__title">{{ p.title }}</h3>
                <p class="blog-card__excerpt">{{ p.summary }}</p>
                <div v-if="p.tags.length" class="blog-card__tags">
                  <span v-for="tg in p.tags.slice(0, 3)" :key="tg.id" class="blog-card__tag">#{{ tg.name }}</span>
                </div>
              </div>
            </NuxtLink>
          </div>

          <BlogPager :page="page" :total-pages="totalPages" :href-for="hrefFor" />
        </template>
      </div>
    </section>
  </div>
</template>

<style scoped>
.blog-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.blog-tags__label { font-size: 12.5px; color: rgb(var(--fg-muted)); font-family: var(--font-mono); }
.blog-tag {
  padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 600;
  background: rgb(var(--bg-sunken)); border: 1px solid rgb(var(--border)); color: rgb(var(--fg-muted));
  transition: all 0.15s ease;
}
.blog-tag:hover { color: rgb(var(--fg)); border-color: rgb(var(--border-strong)); }
.blog-tag.active { background: var(--accent-color); color: rgb(var(--accent-fg)); border-color: var(--accent-color); }

.blog-card__tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }
.blog-card__tag { font-size: 11.5px; color: rgb(var(--fg-muted)); font-family: var(--font-mono); }

.blog-state { text-align: center; padding: 64px 0; color: rgb(var(--fg-muted)); display: flex; flex-direction: column; align-items: center; gap: 14px; }
</style>
