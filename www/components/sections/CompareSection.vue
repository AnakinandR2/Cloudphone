<script setup lang="ts">
const { t } = useGp()
const perfIcons = ['cpu', 'shield', 'wifi', 'clock']
</script>

<template>
  <section class="section" id="specs">
    <div class="container">
      <header>
        <span class="eyebrow">{{ t.compare.eyebrow }}</span>
        <h2 class="section-title">{{ t.compare.title }}</h2>
        <p class="section-sub">{{ t.compare.sub }}</p>
      </header>

      <div class="compare-table">
        <div class="compare-row head">
          <div
            v-for="(c, i) in t.compare.cols"
            :key="i"
            class="compare-cell"
            :class="{ featured: i === 4, 'hide-sm': i === 2 || i === 3 }"
          >{{ c }}</div>
        </div>
        <div v-for="(row, ri) in t.compare.rows" :key="ri" class="compare-row">
          <div
            v-for="(cell, ci) in row"
            :key="ci"
            class="compare-cell"
            :class="{ label: ci === 0, featured: ci === 4, 'hide-sm': ci === 2 || ci === 3 }"
          >
            <template v-if="ci === 0">{{ cell }}</template>
            <span v-else-if="cell === '✓'" class="check">●</span>
            <span v-else-if="cell === '✗'" class="x">—</span>
            <template v-else>{{ cell }}</template>
          </div>
        </div>
      </div>

      <div class="perf-grid" style="margin-top: 80px">
        <div class="perf-card">
          <span class="eyebrow">{{ t.perf.eyebrow }}</span>
          <h3 style="font-size: 24px; margin-top: 14px; margin-bottom: 6px">{{ t.perf.title }}</h3>
          <p style="color: rgb(var(--fg-muted)); font-size: 14.5px">{{ t.perf.sub }}</p>
          <div style="margin-top: 22px">
            <div v-for="([name, val], i) in t.perf.bars" :key="i" class="perf-bar">
              <span class="name">{{ name }}</span>
              <span class="bar-track"><span class="bar-fill" :style="{ width: `${val}%` }" /></span>
              <span class="val">{{ val }}%</span>
            </div>
          </div>
        </div>
        <div class="perf-stat-list">
          <div v-for="([v, l], i) in t.perf.stats" :key="i" class="perf-stat">
            <span class="pico"><GpIcon :name="perfIcons[i] || 'cpu'" /></span>
            <div>
              <div class="pv">{{ v }}</div>
              <div class="pl">{{ l }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
