<script setup lang="ts">
const { t } = useGp()
</script>

<template>
  <section class="section" id="customers">
    <div class="container">
      <header>
        <span class="eyebrow">{{ t.testi.eyebrow }}</span>
        <h2 class="section-title">{{ t.testi.title }}</h2>
      </header>
      <div class="testi-grid">
        <div v-for="(q, i) in t.testi.items" :key="i" class="testi-card">
          <div class="stars">★★★★★</div>
          <p class="quote">"{{ q.text }}"</p>
          <div class="who">
            <span class="avatar">{{ q.initials }}</span>
            <div>
              <div class="name">{{ q.name }}</div>
              <div class="role">{{ q.role }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
