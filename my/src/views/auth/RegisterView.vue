<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import LocaleSwitcher from '@/components/LocaleSwitcher.vue'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const settingsStore = useSettingsStore()
const router = useRouter()
const { t } = useI18n()

const phone = ref('')
const nickname = ref('')
const password = ref('')
const confirm = ref('')
const agree = ref(false)
const loading = ref(false)
const error = ref('')

async function onSubmit() {
  error.value = ''
  if (!agree.value) {
    error.value = t('auth.agreeRequired')
    return
  }
  if (password.value !== confirm.value) {
    error.value = t('register.pwdMismatch')
    return
  }
  loading.value = true
  try {
    await userStore.register({
      phone: phone.value,
      password: password.value,
      nickname: nickname.value,
    })
    toast.success(t('register.success'))
    router.push('/')
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
            {{ t('auth.registerTitle') }}
          </h2>
          <p class="auth-aside__sub">
            {{ t('auth.registerSub') }}
          </p>
        </div>

        <div class="auth-aside__stats">
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.registerStats.a.v') }}
            </div>
            <div class="l">
              {{ t('auth.registerStats.a.l') }}
            </div>
          </div>
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.registerStats.b.v') }}
            </div>
            <div class="l">
              {{ t('auth.registerStats.b.l') }}
            </div>
          </div>
          <div class="auth-aside__stat">
            <div class="v">
              {{ t('auth.registerStats.c.v') }}
            </div>
            <div class="l">
              {{ t('auth.registerStats.c.l') }}
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
          <h1>{{ t('register.title') }}</h1>
          <p class="sub">
            {{ t('register.subtitle') }}
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
              <label for="nickname">{{ t('register.nickname') }}</label>
              <input
                id="nickname"
                v-model="nickname"
                class="auth-input"
                type="text"
                :placeholder="t('register.nicknamePlaceholder')"
              >
            </div>
            <div class="auth-field">
              <label for="password">{{ t('login.password') }}</label>
              <input
                id="password"
                v-model="password"
                class="auth-input"
                type="password"
                autocomplete="new-password"
                :placeholder="t('register.pwdPlaceholder')"
              >
            </div>
            <div class="auth-field">
              <label for="confirm">{{ t('register.confirm') }}</label>
              <input
                id="confirm"
                v-model="confirm"
                class="auth-input"
                type="password"
                autocomplete="new-password"
                :placeholder="t('register.confirmPlaceholder')"
              >
            </div>

            <label class="auth-terms">
              <input v-model="agree" type="checkbox">
              <span>
                {{ t('auth.termsPre') }}
                <a href="#" @click.prevent>{{ t('auth.termsService') }}</a>
                {{ t('auth.termsAnd') }}
                <a href="#" @click.prevent>{{ t('auth.termsPrivacy') }}</a>
                {{ t('auth.termsPost') }}
              </span>
            </label>

            <p v-if="error" class="auth-error">
              {{ error }}
            </p>

            <button type="submit" class="auth-submit" :disabled="loading">
              {{ loading ? t('register.submitting') : t('register.submit') }}
            </button>
          </form>

          <p class="auth-foot">
            {{ t('register.hasAccount') }}
            <RouterLink to="/login">
              {{ t('register.toLogin') }}
            </RouterLink>
          </p>
        </div>
      </section>
    </main>
  </div>
</template>
