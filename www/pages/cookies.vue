<script setup lang="ts">
const { t } = useGp()
const { consent, acceptAll, rejectAll, savePreferences } = useCookieConsent()

const local = reactive({ functional: false, analytics: false, marketing: false })
watchEffect(() => {
  local.functional = consent.value?.functional ?? false
  local.analytics = consent.value?.analytics ?? false
  local.marketing = consent.value?.marketing ?? false
})
const cats = ['necessary', 'functional', 'analytics', 'marketing'] as const
type Toggleable = 'functional' | 'analytics' | 'marketing'

const savedAtText = computed(() => {
  if (!consent.value)
    return t.value.cookies.prefs.notDecided
  const d = new Date(consent.value.ts)
  return t.value.cookies.prefs.savedAt.replace('{time}', d.toLocaleString())
})

function onSave() {
  savePreferences({ ...local })
}

useHead(() => ({ title: t.value.cookies.policy.title }))
</script>

<template>
  <div class="cookies-page">
    <header class="cookies-head">
      <h1>{{ t.cookies.policy.title }}</h1>
      <p class="cookies-updated">{{ t.cookies.policy.updated }}</p>
      <p class="cookies-intro">{{ t.cookies.policy.intro }}</p>
    </header>

    <!-- 偏好编辑 -->
    <section class="cookies-prefs">
      <h2>{{ t.cookies.prefs.title }}</h2>
      <div class="cookies-prefs__list">
        <div v-for="c in cats" :key="c" class="cookies-prefs__row">
          <div class="cookies-prefs__meta">
            <div class="cookies-prefs__name">
              {{ t.cookies.categories[c].name }}
              <span v-if="c === 'necessary'" class="cookies-badge">{{ t.cookies.prefs.alwaysOn }}</span>
            </div>
            <p class="cookies-prefs__desc">{{ t.cookies.categories[c].desc }}</p>
          </div>
          <label class="cookies-switch">
            <input
              type="checkbox"
              :checked="c === 'necessary' ? true : local[c as Toggleable]"
              :disabled="c === 'necessary'"
              @change="c !== 'necessary' && (local[c as Toggleable] = ($event.target as HTMLInputElement).checked)"
            >
          </label>
        </div>
      </div>
      <div class="cookies-prefs__foot">
        <span class="cookies-saved">{{ savedAtText }}</span>
        <div class="cookies-prefs__btns">
          <button class="btn btn-ghost btn-sm" @click="rejectAll">{{ t.cookies.prefs.rejectAll }}</button>
          <button class="btn btn-ghost btn-sm" @click="acceptAll">{{ t.cookies.prefs.acceptAll }}</button>
          <button class="btn btn-primary btn-sm" @click="onSave">{{ t.cookies.prefs.save }}</button>
        </div>
      </div>
    </section>

    <!-- 政策正文 -->
    <section class="cookies-policy">
      <article v-for="(sec, i) in t.cookies.policy.sections" :key="i">
        <h3>{{ sec.h }}</h3>
        <p>{{ sec.p }}</p>
      </article>
    </section>
  </div>
</template>

<style scoped>
.cookies-page { max-width: 820px; margin: 0 auto; padding: 48px 20px 80px; }
.cookies-head h1 { font-size: 30px; font-weight: 700; }
.cookies-updated { margin-top: 6px; font-size: 13px; opacity: 0.6; }
.cookies-intro { margin-top: 12px; opacity: 0.8; }
.cookies-prefs { margin-top: 32px; border: 1px solid rgb(var(--border, 226 232 240) / 0.6); border-radius: 16px; padding: 20px; }
.cookies-prefs h2 { font-size: 18px; font-weight: 600; }
.cookies-prefs__list { margin-top: 14px; display: grid; gap: 14px; }
.cookies-prefs__row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.cookies-prefs__name { font-weight: 600; display: flex; align-items: center; gap: 8px; }
.cookies-prefs__desc { margin-top: 3px; font-size: 13px; opacity: 0.7; }
.cookies-badge { font-size: 11px; padding: 1px 6px; border-radius: 999px; background: rgb(var(--accent) / 0.14); color: var(--accent-strong-color); }
.cookies-switch input { width: 18px; height: 18px; accent-color: var(--accent-color); }
.cookies-prefs__foot { margin-top: 18px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; }
.cookies-saved { font-size: 12px; opacity: 0.6; }
.cookies-prefs__btns { display: flex; gap: 8px; }
.cookies-policy { margin-top: 36px; display: grid; gap: 22px; }
.cookies-policy h3 { font-size: 16px; font-weight: 600; }
.cookies-policy p { margin-top: 6px; opacity: 0.8; line-height: 1.7; }
</style>
