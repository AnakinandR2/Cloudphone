<script setup lang="ts">
const { t } = useGp()
</script>

<template>
  <div class="container">
    <div class="cta-banner">
      <div>
        <h2>{{ t.cta.title }}</h2>
        <p>{{ t.cta.sub }}</p>
      </div>
      <div class="actions">
        <a href="#" class="btn btn-primary btn-lg">{{ t.cta.btn1 }}</a>
        <a href="#" class="btn btn-ghost btn-lg">{{ t.cta.btn2 }}</a>
      </div>
    </div>
  </div>
</template>
