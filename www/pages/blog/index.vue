<script setup lang="ts">
const { t } = useGp()
const localePath = useLocalePath()

const ALL = '__all__'
const activeCat = ref(ALL)

const posts = computed(() => t.value.blog.posts)
const cats = computed(() => Array.from(new Set(posts.value.map((p) => p.cat))))
const filtered = computed(() => (activeCat.value === ALL ? posts.value : posts.value.filter((p) => p.cat === activeCat.value)))
const featured = computed(() => filtered.value[0])
const rest = computed(() => filtered.value.slice(1))

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
        <div class="blog-page-tabs">
          <button class="blog-page-tab" :class="{ active: activeCat === ALL }" @click="activeCat = ALL">{{ t.blog.all }}</button>
          <button v-for="c in cats" :key="c" class="blog-page-tab" :class="{ active: activeCat === c }" @click="activeCat = c">{{ c }}</button>
        </div>
      </div>
    </section>

    <section class="blog-page-grid">
      <div class="container">
        <NuxtLink v-if="featured" :to="localePath('/blog/' + featured.id)" class="blog-feature">
          <div class="blog-feature__cover">
            <img :src="`https://picsum.photos/seed/gp-${featured.cover}/960/600`" alt="" loading="eager" />
          </div>
          <div>
            <div class="blog-feature__meta">
              <span class="blog-feature__cat">{{ featured.cat }}</span>
              <span>{{ featured.date }}</span><span>·</span><span>{{ featured.read }} {{ t.blog.min }}</span>
            </div>
            <h2>{{ featured.title }}</h2>
            <p>{{ featured.excerpt }}</p>
            <span class="btn btn-quiet btn-sm cta">{{ t.blog.readMore }} <GpIcon name="arrow-sm" style="width: 14px; height: 14px" /></span>
          </div>
        </NuxtLink>

        <div class="blog-grid">
          <NuxtLink v-for="p in rest" :key="p.id" :to="localePath('/blog/' + p.id)" class="blog-card">
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
  </div>
</template>
