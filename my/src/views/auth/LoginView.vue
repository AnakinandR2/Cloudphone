<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import LocaleSwitcher from '@/components/LocaleSwitcher.vue'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const settingsStore = useSettingsStore()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const phone = ref('')
const password = ref('')
const loading = ref(false)

async function onSubmit() {
  loading.value = true
  try {
    await userStore.login({ phone: phone.value, password: password.value })
    toast.success(t('login.success'))
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  }
  catch {
    // 错误已在 axios 拦截器统一 toast
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="gp-auth">
    <main class="auth-split">
      <!-- 左侧：深绿品牌面板 -->
      <aside class="auth-aside">
        <div class="auth-aside__tiles" aria-hidden="true">
          <i /><i /><i /><i />
        </div>

        <a class="auth-aside__brand" href="/">
          <span class="brand-mark" />
          <span>{{ t('auth.brand') }}</span>
        </a>

        <div class="auth-aside__center">
          <h2 class="auth-aside__title">
            {{ t('auth.loginTitle') }}
          </h2>
          <p class="auth-aside__sub">
            {{ t('auth.loginSub') }}
          </p>
        </div>

        <div class="auth-aside__stats">
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.loginStats.d.v') }}
            </div>
            <div class="l">
              {{ t('auth.loginStats.d.l') }}
            </div>
          </div>
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.loginStats.c.v') }}
            </div>
            <div class="l">
              {{ t('auth.loginStats.c.l') }}
            </div>
          </div>
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.loginStats.s.v') }}
            </div>
            <div class="l">
              {{ t('auth.loginStats.s.l') }}
            </div>
          </div>
        </div>
      </aside>

      <!-- 右侧：表单 -->
      <section class="auth-form">
        <div class="auth-form__top">
          <a class="auth-form__brand" href="/">
            <span class="brand-mark" />
            <span>{{ t('auth.brand') }}</span>
          </a>
          <div class="auth-form__tools">
            <LocaleSwitcher />
            <button class="icon-btn" type="button" :title="t('header.toggleTheme')" @click="settingsStore.toggleDark()">
              <Moon v-if="settingsStore.isDark" :size="18" />
              <Sun v-else :size="18" />
            </button>
          </div>
        </div>

        <div class="auth-form__center">
          <h1>{{ t('login.welcome') }}</h1>
          <p class="sub">
            {{ t('login.subtitle') }}
          </p>

          <form @submit.prevent="onSubmit">
            <div class="auth-field">
              <label for="phone">{{ t('login.phone') }}</label>
              <input
                id="phone"
                v-model="phone"
                class="auth-input"
                type="tel"
                autocomplete="username"
                :placeholder="t('login.phonePlaceholder')"
              >
            </div>
            <div class="auth-field">
              <label for="password">{{ t('login.password') }}</label>
              <input
                id="password"
                v-model="password"
                class="auth-input"
                type="password"
                autocomplete="current-password"
                :placeholder="t('login.passwordPlaceholder')"
              >
            </div>

            <button type="submit" class="auth-submit" :disabled="loading">
              {{ loading ? t('login.submitting') : t('login.submit') }}
            </button>
          </form>

          <p class="auth-foot">
            {{ t('login.noAccount') }}
            <RouterLink to="/register">
              {{ t('login.toRegister') }}
            </RouterLink>
          </p>
        </div>
      </section>
    </main>
  </div>
</template>
