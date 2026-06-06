<script setup lang="ts">
const { t } = useGp()
const platformIcons = ['win', 'apple', 'android', 'apple']
</script>

<template>
  <section class="section" id="download" style="background: rgb(var(--bg-sunken))">
    <div class="container">
      <header style="text-align: center">
        <span class="eyebrow">{{ t.download.eyebrow }}</span>
        <h2 class="section-title">{{ t.download.title }}</h2>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ t.download.sub }}</p>
      </header>
      <div class="dl-grid">
        <div v-for="(d, i) in t.download.items" :key="i" class="dl-card">
          <span class="dl-icon"><GpIcon :name="platformIcons[i] || 'win'" /></span>
          <div>
            <h4>{{ d.name }}</h4>
            <div class="ver">{{ d.ver }} · {{ d.size }}</div>
          </div>
          <a href="#" class="btn btn-quiet" style="width: 100%">{{ t.download.btn }}</a>
        </div>
      </div>
    </div>
  </section>
</template>
