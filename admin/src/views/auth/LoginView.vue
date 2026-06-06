<script setup lang="ts">
import { IdCard, Moon, ShieldCheck, Sun } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import LocaleSwitcher from '@/components/LocaleSwitcher.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const settingsStore = useSettingsStore()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const title = import.meta.env.VITE_APP_TITLE || '管理后台'

const account = ref('admin')
const password = ref('admin123')
const loading = ref(false)
const ssoLoading = ref(false)

// 小西通行证（SSO）配置
const SSO_PROJECT_ID = import.meta.env.VITE_SSO_PROJECT_ID || 'unset'
const SSO_LOGIN_URL = import.meta.env.VITE_SSO_LOGIN_URL || 'https://sso.xiaoxitech.com/login'

async function onSubmit() {
  loading.value = true
  try {
    await userStore.login({ account: account.value, password: password.value })
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

function quickFill(acc: string) {
  account.value = acc
  // 与后端内置用户对应（Mock 模式下密码任意亦可）
  password.value = `${acc}123`
}

// 跳转到小西通行证登录页
function handleSSOLogin() {
  const { origin, pathname } = window.location
  // 暂存重定向地址，回调后继续跳转
  if (route.query.redirect) {
    localStorage.setItem('sso_redirect', route.query.redirect as string)
  }
  // 回调带上标记参数，回到本页后据此处理 token
  const callbackURL = `${origin}${pathname}?sso_callback=1`
  window.location.href = `${SSO_LOGIN_URL}?project=${SSO_PROJECT_ID}&cb=${encodeURIComponent(callbackURL)}`
}

// 处理 SSO 回调：用 token 换取登录态
async function handleSSOCallback(token: string) {
  ssoLoading.value = true
  try {
    await userStore.ssoLogin(token)
    const redirect = localStorage.getItem('sso_redirect') || '/'
    localStorage.removeItem('sso_redirect')
    // 清掉 URL 上的回调参数
    window.history.replaceState({}, '', window.location.pathname)
    toast.success(t('login.success'))
    router.push(redirect)
  }
  catch {
    // 错误已在 axios 拦截器统一 toast
    window.history.replaceState({}, '', window.location.pathname)
  }
  finally {
    ssoLoading.value = false
  }
}

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (params.get('sso_callback') && token) {
    handleSSOCallback(token)
  }
})
</script>

<template>
  <div class="bg-background relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <!-- 柔和渐变光斑背景 -->
    <div class="login-aurora" aria-hidden="true" />

    <!-- 右上角：语言 / 明暗 -->
    <div class="absolute top-4 right-4 z-10 flex items-center gap-1">
      <LocaleSwitcher />
      <Button variant="ghost" size="icon" :title="t('header.toggleTheme')" @click="settingsStore.toggleDark()">
        <Moon v-if="settingsStore.isDark" class="size-4" />
        <Sun v-else class="size-4" />
      </Button>
    </div>

    <!-- 登录卡片：左品牌 + 右表单 -->
    <div class="bg-card relative z-10 grid w-full max-w-4xl overflow-hidden rounded-2xl border shadow-2xl lg:grid-cols-2">
      <!-- 品牌横幅（小屏隐藏） -->
      <div class="login-banner relative hidden flex-col justify-between p-10 text-white lg:flex">
        <div class="relative z-10 flex items-center gap-2">
          <div class="bg-white/15 flex size-9 items-center justify-center rounded-xl backdrop-blur">
            <ShieldCheck class="size-5" />
          </div>
          <span class="text-lg font-semibold">{{ title }}</span>
        </div>
        <div class="relative z-10 space-y-3">
          <h2 class="text-3xl font-bold leading-tight">
            {{ t('login.welcome') }}
          </h2>
          <p class="text-white/80 max-w-xs text-sm">
            {{ t('login.brandSlogan') }}
          </p>
        </div>
        <p class="text-white/50 relative z-10 text-xs">
          © {{ title }}
        </p>
      </div>

      <!-- 表单 -->
      <div class="flex flex-col justify-center p-8 sm:p-10">
        <div class="mb-6 space-y-1">
          <h1 class="text-2xl font-bold tracking-tight">
            {{ t('login.welcome') }}
          </h1>
          <p class="text-muted-foreground text-sm">
            {{ t('login.subtitle') }}
          </p>
        </div>

        <form class="space-y-4" @submit.prevent="onSubmit">
          <div class="space-y-2">
            <label class="text-sm font-medium" for="account">{{ t('login.account') }}</label>
            <Input id="account" v-model="account" :placeholder="t('login.account')" autocomplete="username" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium" for="password">{{ t('login.password') }}</label>
            <Input id="password" v-model="password" type="password" :placeholder="t('login.password')" autocomplete="current-password" />
          </div>

          <div class="text-muted-foreground flex items-center gap-2 text-xs">
            <span>{{ t('login.quick') }}</span>
            <button type="button" class="text-primary hover:underline" @click="quickFill('admin')">
              admin
            </button>
            <button type="button" class="text-primary hover:underline" @click="quickFill('test')">
              test
            </button>
          </div>

          <Button type="submit" size="lg" class="w-full" :disabled="loading || ssoLoading">
            {{ loading ? t('login.submitting') : t('login.submit') }}
          </Button>

          <div class="flex items-center gap-3 py-1">
            <Separator class="flex-1" />
            <span class="text-muted-foreground text-xs">{{ t('login.or') }}</span>
            <Separator class="flex-1" />
          </div>

          <Button
            type="button"
            variant="outline"
            size="lg"
            class="w-full"
            :disabled="loading || ssoLoading"
            @click="handleSSOLogin"
          >
            <IdCard class="mr-2 size-4" />
            {{ ssoLoading ? t('login.ssoLoggingIn') : t('login.ssoLogin') }}
          </Button>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 背景光斑：随主题色变化的柔和模糊渐变 */
.login-aurora {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(closest-side, color-mix(in oklch, var(--primary) 28%, transparent), transparent) no-repeat,
    radial-gradient(closest-side, color-mix(in oklch, var(--chart-4, var(--primary)) 22%, transparent), transparent) no-repeat;
  background-position: 12% 18%, 88% 82%;
  background-size: 55vw 55vw, 50vw 50vw;
  filter: blur(80px);
  opacity: 0.7;
}

/* 品牌横幅：主题色渐变 */
.login-banner {
  background:
    linear-gradient(135deg,
      color-mix(in oklch, var(--primary) 92%, black 4%),
      color-mix(in oklch, var(--primary) 70%, var(--chart-4, var(--primary)))
    );
}

.login-banner::before {
  position: absolute;
  inset: 0;
  content: "";
  background:
    radial-gradient(closest-side, rgb(255 255 255 / 18%), transparent) no-repeat,
    radial-gradient(closest-side, rgb(255 255 255 / 12%), transparent) no-repeat;
  background-position: 90% 10%, 10% 90%;
  background-size: 60% 60%, 50% 50%;
}
</style>
