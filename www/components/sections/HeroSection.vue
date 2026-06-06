<script setup lang="ts">
const { t } = useGp()
const localePath = useLocalePath()

const partners: [string, string][] = [
  ['NovaCommerce', 'commerce'],
  ['Globepath', 'globe'],
  ['Anchorly', 'anchor'],
  ['Driftbase', 'drift'],
  ['Riverline', 'river'],
  ['Hexpoint', 'hex'],
  ['Saltcloud', 'cloud'],
  ['Yondermass', 'compass'],
]
const partnersRev = [...partners].reverse()
</script>

<template>
  <section class="hero hero-auth-shell">
    <div class="hero-bg" />
    <div class="container">
      <div class="hero-grid">
        <div>
          <span class="eyebrow">{{ t.hero.tag }}</span>
          <h1>
            {{ t.hero.title1 }} <span class="hl">{{ t.hero.titleHL }}</span> {{ t.hero.title2 }}
          </h1>
          <p class="hero-sub">{{ t.hero.sub }}</p>
          <div class="hero-ctas">
            <a href="#" class="btn btn-primary btn-lg">{{ t.hero.cta1 }} <GpIcon name="arrow" /></a>
            <a :href="localePath('/') + '#scenarios'" class="btn btn-ghost btn-lg">{{ t.hero.cta2 }}</a>
          </div>
          <div class="hero-trust">
            <span class="trust-item"><span class="dot" />{{ t.hero.trust1 }}</span>
            <span class="trust-item"><span class="dot" />{{ t.hero.trust2 }}</span>
            <span class="trust-item"><span class="dot" />{{ t.hero.trust3 }}</span>
          </div>
        </div>
        <div class="phone-stage hero-scene" aria-hidden="true">
          <div class="hero-scene__glow hero-scene__glow--a" />
          <div class="hero-scene__glow hero-scene__glow--b" />
          <div class="hero-scene__dotmap" />
          <img src="/images/hero-compress.png" alt="" class="hero-scene__img" loading="eager" decoding="async" />
        </div>
      </div>

      <div class="hero-stats">
        <div><div class="num">120,000+</div><div class="lbl">{{ t.stats.devices }} · {{ t.stats.devicesL }}</div></div>
        <div><div class="num">8,000+</div><div class="lbl">{{ t.stats.users }} · {{ t.stats.usersL }}</div></div>
        <div><div class="num">99.99%</div><div class="lbl">{{ t.stats.uptime }} · {{ t.stats.uptimeL }}</div></div>
        <div><div class="num">60+</div><div class="lbl">{{ t.stats.ips }} · {{ t.stats.ipsL }}</div></div>
      </div>

      <div class="logo-strip logo-strip--dual" :aria-label="t.logos">
        <div class="logo-track logo-track--ltr">
          <span v-for="(p, i) in [...partners, ...partners]" :key="`r1-${p[0]}-${i}`" class="logo-pill">
            <GpPartnerMark :type="p[1]" /><span>{{ p[0] }}</span>
          </span>
        </div>
        <div class="logo-track logo-track--rtl">
          <span v-for="(p, i) in [...partnersRev, ...partnersRev]" :key="`r2-${p[0]}-${i}`" class="logo-pill">
            <GpPartnerMark :type="p[1]" /><span>{{ p[0] }}</span>
          </span>
        </div>
      </div>
    </div>
  </section>
</template>
