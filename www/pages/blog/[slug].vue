<script setup lang="ts">
const { t, bodies } = useGp()
const localePath = useLocalePath()
const route = useRoute()

const slug = computed(() => String(route.params.slug))
const post = computed(() => t.value.blog.posts.find((p) => p.id === slug.value))

if (!post.value) {
  throw createError({ statusCode: 404, statusMessage: 'Post not found', fatal: true })
}

// Structured article body (lede + sections + lists); fall back to the excerpt.
const blocks = computed(() => bodies.value[slug.value] ?? [{ k: 'p' as const, t: post.value!.excerpt }])

const related = computed(() => t.value.blog.posts.filter((p) => p.id !== slug.value).slice(0, 3))

const crumbs = computed(() => [
  { label: t.value.nav.home, to: localePath('/') },
  { label: t.value.blog.eyebrow, to: localePath('/blog') },
  { label: post.value!.title },
])

useHead({ title: () => `${post.value!.title} — Gloryphone` })
</script>

<template>
  <article v-if="post">
    <div class="section article-hero" style="padding-bottom: 0">
      <div class="container">
        <Breadcrumb :items="crumbs" />
        <div class="blog-feature__meta" style="margin-top: 18px">
          <span class="blog-feature__cat">{{ post.cat }}</span>
          <span>{{ post.date }}</span><span>·</span><span>{{ post.read }} {{ t.blog.min }}</span>
        </div>
        <div class="article-cover">
          <img :src="`https://picsum.photos/seed/gp-${post.cover}/1200/600`" alt="" loading="eager" />
        </div>
      </div>
    </div>

    <div class="article-body">
      <div class="container">
        <h1>{{ post.title }}</h1>
        <template v-for="(b, i) in blocks" :key="i">
          <h2 v-if="b.k === 'h2'">{{ b.t }}</h2>
          <ul v-else-if="b.k === 'ul'">
            <li v-for="(it, j) in b.items" :key="j">{{ it }}</li>
          </ul>
          <p v-else :class="{ lead: i === 0 }">{{ b.t }}</p>
        </template>
      </div>
    </div>

    <section class="section" style="padding-top: 0">
      <div class="container">
        <h2 class="section-title" style="font-size: 24px; margin-bottom: 8px">{{ t.blog.title }}</h2>
        <div class="blog-grid">
          <NuxtLink v-for="rp in related" :key="rp.id" :to="localePath('/blog/' + rp.id)" class="blog-card">
            <div class="blog-card__cover">
              <img :src="`https://picsum.photos/seed/gp-${rp.cover}/720/440`" alt="" loading="lazy" />
              <span class="blog-card__cat">{{ rp.cat }}</span>
            </div>
            <div class="blog-card__body">
              <div class="blog-card__meta">
                <span>{{ rp.date }}</span><span>·</span><span>{{ rp.read }} {{ t.blog.min }}</span>
              </div>
              <h3 class="blog-card__title">{{ rp.title }}</h3>
              <p class="blog-card__excerpt">{{ rp.excerpt }}</p>
              <span class="blog-card__more">{{ t.blog.readMore }} <GpIcon name="arrow" /></span>
            </div>
          </NuxtLink>
        </div>
      </div>
    </section>
  </article>
</template>
