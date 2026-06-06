<script setup lang="ts">
const { t } = useGp()
const localePath = useLocalePath()
const social = ['X', 'IG', 'YT', 'DC', 'WX', 'GH']
</script>

<template>
  <footer class="footer">
    <div class="container">
      <div class="footer-grid">
        <div class="footer-col footer-brand">
          <NuxtLink :to="localePath('/')" class="brand">
            <span class="brand-mark" />
            <span>Gloryphone</span>
          </NuxtLink>
          <p class="desc">{{ t.footer.desc }}</p>
          <div class="footer-social">
            <a v-for="s in social" :key="s" class="icon-btn" href="#" :aria-label="s" style="width: 32px; height: 32px; font-size: 11px; font-weight: 600">{{ s }}</a>
          </div>
        </div>
        <div v-for="(c, i) in t.footer.cols" :key="i" class="footer-col">
          <h5>{{ c.title }}</h5>
          <ul>
            <li v-for="(l, j) in c.links" :key="j"><a href="#">{{ l }}</a></li>
          </ul>
        </div>
      </div>
      <div class="footer-bottom">
        <span>{{ t.footer.copy }}</span>
        <NuxtLink :to="localePath('/cookies')" class="footer-cookie-link">{{ t.cookies.banner.manage }}</NuxtLink>
        <span class="beian">{{ t.footer.beian }}</span>
      </div>
    </div>
  </footer>
</template>
