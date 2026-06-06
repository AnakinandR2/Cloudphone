<script setup lang="ts">
const { t } = useGp()
const tab = ref(0)
const panel = computed(() => t.value.scenarios.panels[tab.value])

const slugs = ['commerce', 'social', 'gaming', 'testing', 'office']
const exts = ['png', 'png', 'png', 'jpg', 'jpg']
const visual = computed(() => {
  const i = tab.value >= 0 && tab.value < slugs.length ? tab.value : 0
  return {
    src: `/images/scenario-${slugs[i]}.${exts[i]}`,
    cls: exts[i] === 'png' ? 'usecase-img usecase-img--contain theme-adaptive-img' : 'usecase-img theme-adaptive-img',
    key: slugs[i],
  }
})
</script>

<template>
  <section class="section" id="scenarios" style="background: rgb(var(--bg-sunken))">
    <div class="container">
      <header>
        <span class="eyebrow">{{ t.scenarios.eyebrow }}</span>
        <h2 class="section-title">{{ t.scenarios.title }}</h2>
      </header>
      <div class="usecase-tabs">
        <button
          v-for="(label, i) in t.scenarios.tabs"
          :key="i"
          class="usecase-tab"
          :class="{ active: i === tab }"
          @click="tab = i"
        >{{ label }}</button>
      </div>
      <div class="usecase-panel">
        <div>
          <h3>{{ panel.t }}</h3>
          <p>{{ panel.d }}</p>
          <div class="usecase-metrics">
            <div v-for="(mm, i) in panel.m" :key="i" class="usecase-metric">
              <div class="v">{{ mm[0] }}</div>
              <div class="l">{{ mm[1] }}</div>
            </div>
          </div>
        </div>
        <div class="usecase-visual">
          <img :key="visual.key" :class="visual.cls" :src="visual.src" alt="" loading="lazy" />
        </div>
      </div>
    </div>
  </section>
</template>
