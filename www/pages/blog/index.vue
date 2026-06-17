<script setup lang="ts">
// 博客列表：分类 + 标签筛选 + 分页，数据来自内容中台。
// 筛选态写入 URL（?category=&tag=&page=），可分享、可后退。
const { t } = useGp()
const { locale } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 12

// 先 await taxonomy：slug→id 映射依赖它，确保 SSR 构造文章查询时已就绪。
const { data: taxo } = await useBlogTaxonomy()
const categories = computed(() => taxo.value?.categories ?? [])
const tags = computed(() => taxo.value?.tags ?? [])

// URL → 选中态
const activeCat = computed(() => (route.query.category ? String(route.query.category) : ''))
const activeTag = computed(() => (route.query.tag ? String(route.query.tag) : ''))
const page = computed(() => Math.max(1, Number.parseInt(String(route.query.page ?? '1'), 10) || 1))
const hasFilter = computed(() => !!activeCat.value || !!activeTag.value)

// slug → id（taxonomy 加载完成后才有值，未命中则不带该过滤）
const categoryId = computed(() => categories.value.find((c) => c.slug === activeCat.value)?.id)
const tagId = computed(() => tags.value.find((tg) => tg.slug === activeTag.value)?.id)

const { list, total, pending, error, refresh } = useBlogPosts(
  computed(() => ({
    page: page.value,
    size: PAGE_SIZE,
    categoryId: categoryId.value,
    tagId: tagId.value,
    sort: 'published_desc' as const,
  })),
)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
// 仅首页且无筛选时，用首条做 featured 大图。
const featured = computed(() => (page.value === 1 && !hasFilter.value ? list.value[0] : undefined))
const rest = computed(() => (featured.value ? list.value.slice(1) : list.value))

function pushQuery(next: { category?: string; tag?: string; page?: number }) {
  const q: Record<string, string> = {}
  if (next.category) q.category = next.category
  if (next.tag) q.tag = next.tag
  if (next.page && next.page > 1) q.page = String(next.page)
  router.push({ query: q })
}

function selectCat(slug: string) {
  // 切分类时清空页码，保留标签
  pushQuery({ category: slug || undefined, tag: activeTag.value || undefined })
}
function toggleTag(slug: string) {
  const nextTag = activeTag.value === slug ? undefined : slug
  pushQuery({ category: activeCat.value || undefined, tag: nextTag })
}
function goPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  pushQuery({ category: activeCat.value || undefined, tag: activeTag.value || undefined, page: p })
  if (import.meta.client) window.scrollTo({ top: 0, behavior: 'smooth' })
}

const fmt = (iso: string) => formatBlogDate(iso, locale.value)

const crumbs = computed(() => [
  { label: t.value.nav.home, to: localePath('/') },
  { label: t.value.blog.eyebrow },
])

useHead({ title: () => `${t.value.blog.eyebrow} — Gloryphone` })
</script>

<template>
  <div>
    <section class="blog-page-hero">
      <div class="container">
        <Breadcrumb :items="crumbs" style="margin-bottom: 18px" />
        <h1>{{ t.blog.title }}</h1>
        <p>{{ t.blog.sub }}</p>

        <!-- 分类 -->
        <div class="blog-page-tabs">
          <button class="blog-page-tab" :class="{ active: !activeCat }" @click="selectCat('')">{{ t.blog.all }}</button>
          <button
            v-for="c in categories"
            :key="c.id"
            class="blog-page-tab"
            :class="{ active: activeCat === c.slug }"
            @click="selectCat(c.slug)"
          >
            {{ c.name }}
          </button>
        </div>

        <!-- 标签（可点击筛选） -->
        <div v-if="tags.length" class="blog-tags">
          <span class="blog-tags__label">{{ t.blog.tags }}:</span>
          <button
            v-for="tg in tags"
            :key="tg.id"
            class="blog-tag"
            :class="{ active: activeTag === tg.slug }"
            @click="toggleTag(tg.slug)"
          >
            #{{ tg.name }}
          </button>
        </div>
      </div>
    </section>

    <section class="blog-page-grid">
      <div class="container">
        <!-- 错误态 -->
        <div v-if="error" class="blog-state">
          <p>{{ t.blog.error }}</p>
          <button class="btn btn-quiet btn-sm" @click="refresh()">{{ t.blog.retry }}</button>
        </div>

        <!-- 加载态 -->
        <div v-else-if="pending && !list.length" class="blog-state">
          <p>…</p>
        </div>

        <!-- 空态 -->
        <div v-else-if="!list.length" class="blog-state">
          <p>{{ t.blog.empty }}</p>
        </div>

        <template v-else>
          <NuxtLink v-if="featured" :to="localePath('/blog/' + featured.slug)" class="blog-feature">
            <div class="blog-feature__cover">
              <img :src="featured.cover_url" :alt="featured.cover_alt" loading="eager" />
            </div>
            <div>
              <div class="blog-feature__meta">
                <span v-if="featured.category" class="blog-feature__cat">{{ featured.category.name }}</span>
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
                <span v-if="p.category" class="blog-card__cat">{{ p.category.name }}</span>
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

          <!-- 分页 -->
          <div v-if="totalPages > 1" class="blog-pager">
            <button class="btn btn-quiet btn-sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t.blog.prev }}</button>
            <span class="blog-pager__info">{{ page }} / {{ totalPages }}</span>
            <button class="btn btn-quiet btn-sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t.blog.next }}</button>
          </div>
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
  cursor: pointer; transition: all 0.15s ease;
}
.blog-tag:hover { color: rgb(var(--fg)); border-color: rgb(var(--border-strong)); }
.blog-tag.active { background: var(--accent-color); color: rgb(var(--accent-fg)); border-color: var(--accent-color); }

.blog-card__tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }
.blog-card__tag { font-size: 11.5px; color: rgb(var(--fg-muted)); font-family: var(--font-mono); }

.blog-state { text-align: center; padding: 64px 0; color: rgb(var(--fg-muted)); display: flex; flex-direction: column; align-items: center; gap: 14px; }

.blog-pager { display: flex; align-items: center; justify-content: center; gap: 18px; margin-top: 48px; }
.blog-pager__info { font-size: 13.5px; color: rgb(var(--fg-muted)); font-family: var(--font-mono); }
.blog-pager button[disabled] { opacity: 0.4; pointer-events: none; }
</style>
