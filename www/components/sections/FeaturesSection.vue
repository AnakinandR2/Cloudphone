<script setup lang="ts">
const { t } = useGp()
const icons = ['fingerprint', 'wifi', 'cpu', 'clock', 'users', 'code']
</script>

<template>
  <section class="section" id="features">
    <div class="container">
      <header>
        <span class="eyebrow">{{ t.features.eyebrow }}</span>
        <h2 class="section-title">{{ t.features.title }}</h2>
        <p class="section-sub">{{ t.features.sub }}</p>
      </header>
      <div class="features-grid">
        <div v-for="(it, i) in t.features.items" :key="i" class="card feature-card">
          <span class="ficon"><GpIcon :name="icons[i] || 'shield'" /></span>
          <h3>{{ it.t }}</h3>
          <p>{{ it.d }}</p>
        </div>
      </div>

      <div class="feature-spotlight">
        <div class="copy">
          <span class="eyebrow">{{ t.spotlight.eyebrow }}</span>
          <h3 style="margin-top: 14px">{{ t.spotlight.title }}</h3>
          <p>{{ t.spotlight.sub }}</p>
          <ul>
            <li v-for="(b, i) in t.spotlight.bullets" :key="i">
              <GpIcon name="check-bold" style="width: 18px; height: 18px" />
              <span>{{ b }}</span>
            </li>
          </ul>
          <a href="#" class="btn btn-quiet btn-sm" style="margin-top: 26px">{{ t.spotlight.more }} <GpIcon name="arrow" /></a>
        </div>
        <div class="visual">
          <img class="spotlight-img theme-adaptive-img" src="/images/spotlight-cloud-console.jpg" alt="" loading="lazy" />
        </div>
      </div>
    </div>
  </section>
</template>
