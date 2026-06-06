<script setup lang="ts">
const { t } = useGp()
const localePath = useLocalePath()
const posts = computed(() => t.value.blog.posts.slice(0, 3))
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
      <div class="blog-grid">
        <NuxtLink v-for="p in posts" :key="p.id" :to="localePath('/blog') + '#' + p.id" class="blog-card">
          <div class="blog-card__cover">
            <img :src="`https://picsum.photos/seed/gp-${p.cover}/720/440`" alt="" loading="lazy" />
            <span class="blog-card__cat">{{ p.cat }}</span>
          </div>
          <div class="blog-card__body">
            <div class="blog-card__meta">
              <span>{{ p.date }}</span><span>·</span><span>{{ p.read }} {{ t.blog.min }}</span>
            </div>
            <h3 class="blog-card__title">{{ p.title }}</h3>
            <p class="blog-card__excerpt">{{ p.excerpt }}</p>
            <span class="blog-card__more">{{ t.blog.readMore }} <GpIcon name="arrow" /></span>
          </div>
        </NuxtLink>
      </div>
    </div>
  </section>
</template>
