import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import authApi from '@/api/modules/auth'

const PREFIX = import.meta.env.VITE_APP_STORAGE_PREFIX || 'admin'
const k = (key: string) => `${PREFIX}_${key}`

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem(k('token')) ?? '')
  const account = ref(localStorage.getItem(k('account')) ?? '')
  const name = ref(localStorage.getItem(k('name')) ?? '')
  const avatar = ref(localStorage.getItem(k('avatar')) ?? '')
  const isSuperuser = ref(localStorage.getItem(k('is_superuser')) === 'true')
  const permissions = ref<string[]>([])

  const isLogin = computed(() => !!token.value)

  /** 把登录结果写入状态与 localStorage（账号密码登录、SSO 登录共用） */
  function applyLoginResult(d: {
    token: string
    account: string
    avatar: string
    is_superuser: boolean
  }) {
    token.value = d.token
    account.value = d.account
    avatar.value = d.avatar
    isSuperuser.value = d.is_superuser
    localStorage.setItem(k('token'), d.token)
    localStorage.setItem(k('account'), d.account)
    localStorage.setItem(k('avatar'), d.avatar)
    localStorage.setItem(k('is_superuser'), String(d.is_superuser))
  }

  async function login(data: { account: string, password: string }) {
    const res = await authApi.login(data)
    applyLoginResult(res.data)
  }

  /** 小西通行证（SSO）登录：用 SSO 回调 token 换取登录态 */
  async function ssoLogin(ssoToken: string) {
    const res = await authApi.ssoLogin({ token: ssoToken })
    applyLoginResult(res.data)
    return res.data
  }

  /** 拉取用户信息与权限码 */
  async function getInfo() {
    const res = await authApi.me()
    const d = res.data
    permissions.value = d.permissions ?? []
    isSuperuser.value = d.is_superuser
    name.value = d.name
    if (d.avatar) {
      avatar.value = d.avatar
    }
    localStorage.setItem(k('name'), d.name)
    localStorage.setItem(k('is_superuser'), String(d.is_superuser))
    return d
  }

  function logout() {
    token.value = ''
    account.value = ''
    name.value = ''
    avatar.value = ''
    isSuperuser.value = false
    permissions.value = []
    ;['token', 'account', 'name', 'avatar', 'is_superuser'].forEach(key =>
      localStorage.removeItem(k(key)),
    )
  }

  return {
    token,
    account,
    name,
    avatar,
    isSuperuser,
    permissions,
    isLogin,
    login,
    ssoLogin,
    getInfo,
    logout,
  }
})
