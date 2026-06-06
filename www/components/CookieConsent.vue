<script setup lang="ts">
import type { ConsentCategory } from '~/composables/useCookieConsent'

const { t } = useGp()
const localePath = useLocalePath()
const { hasDecided, consent, acceptAll, rejectAll, savePreferences } = useCookieConsent()

const expanded = ref(false)
const local = reactive({ functional: false, analytics: false, marketing: false })
const cats: Array<{ key: ConsentCategory, fixed: boolean }> = [
  { key: 'necessary', fixed: true },
  { key: 'functional', fixed: false },
  { key: 'analytics', fixed: false },
  { key: 'marketing', fixed: false },
]
type Toggleable = 'functional' | 'analytics' | 'marketing'

function openCustomize() {
  local.functional = consent.value?.functional ?? false
  local.analytics = consent.value?.analytics ?? false
  local.marketing = consent.value?.marketing ?? false
  expanded.value = true
}
function onSave() {
  savePreferences({ ...local })
}
</script>

<template>
  <Teleport to="body">
    <div v-if="!hasDecided" class="cookie-consent" role="dialog" aria-live="polite" aria-label="Cookie">
      <div class="cookie-consent__inner">
        <div class="cookie-consent__text">
          <strong>{{ t.cookies.banner.title }}</strong>
          <p>
            {{ t.cookies.banner.body }}
            <NuxtLink :to="localePath('/cookies')" class="cookie-consent__more">{{ t.cookies.banner.learnMore }}</NuxtLink>
          </p>
        </div>

        <div v-if="expanded" class="cookie-consent__cats">
          <div v-for="c in cats" :key="c.key" class="cookie-consent__cat">
            <label class="cookie-consent__cat-head">
              <input
                type="checkbox"
                :checked="c.fixed ? true : local[c.key as Toggleable]"
                :disabled="c.fixed"
                @change="!c.fixed && (local[c.key as Toggleable] = ($event.target as HTMLInputElement).checked)"
              >
              <span class="cookie-consent__cat-name">{{ t.cookies.categories[c.key].name }}</span>
              <span v-if="c.fixed" class="cookie-consent__badge">{{ t.cookies.prefs.alwaysOn }}</span>
            </label>
            <p class="cookie-consent__cat-desc">{{ t.cookies.categories[c.key].desc }}</p>
          </div>
        </div>

        <div class="cookie-consent__actions">
          <button class="btn btn-ghost btn-sm" @click="rejectAll">{{ t.cookies.banner.reject }}</button>
          <button v-if="!expanded" class="btn btn-ghost btn-sm" @click="openCustomize">{{ t.cookies.banner.customize }}</button>
          <button v-else class="btn btn-ghost btn-sm" @click="onSave">{{ t.cookies.prefs.save }}</button>
          <button class="btn btn-primary btn-sm" @click="acceptAll">{{ t.cookies.banner.accept }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.cookie-consent { position: fixed; left: 0; right: 0; bottom: 0; z-index: 120; padding: 12px; }
.cookie-consent__inner {
  max-width: 1080px; margin: 0 auto; background: #fff; color: #0f172a;
  border: 1px solid #e2e8f0; border-radius: 14px;
  box-shadow: 0 12px 48px -12px rgb(0 0 0 / 0.3); padding: 16px 18px;
  display: flex; flex-direction: column; gap: 12px;
}
:global(.dark) .cookie-consent__inner { background: #1e293b; color: #f1f5f9; border-color: #334155; }
.cookie-consent__text strong { font-size: 15px; }
.cookie-consent__text p { margin: 4px 0 0; font-size: 13px; opacity: 0.8; }
.cookie-consent__more { color: var(--accent-strong-color); text-decoration: underline; }
.cookie-consent__cats { display: grid; gap: 10px; }
.cookie-consent__cat-head { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; cursor: pointer; }
.cookie-consent__cat-desc { margin: 2px 0 0 24px; font-size: 12px; opacity: 0.65; }
.cookie-consent__badge { font-size: 11px; padding: 1px 6px; border-radius: 999px; background: rgb(var(--accent) / 0.14); color: var(--accent-strong-color); }
.cookie-consent__actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
@media (max-width: 720px) { .cookie-consent__actions .btn { flex: 1; } }
</style>
