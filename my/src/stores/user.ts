import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import userApi from '@/api/modules/user'
import type { AuthResult, LoginRequest, RegisterRequest } from '@/types/user'

const PREFIX = import.meta.env.VITE_APP_STORAGE_PREFIX || 'my'
const k = (key: string) => `${PREFIX}_${key}`

/** 前台用户（user）状态 */
export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem(k('token')) ?? '')
  const id = ref(Number(localStorage.getItem(k('id')) ?? 0))
  const phone = ref(localStorage.getItem(k('phone')) ?? '')
  const nickname = ref(localStorage.getItem(k('nickname')) ?? '')
  const avatar = ref(localStorage.getItem(k('avatar')) ?? '')
  // 是否已拉取过用户信息（避免每次导航重复请求）
  const loaded = ref(false)

  const isLogin = computed(() => !!token.value)
  const displayName = computed(() => nickname.value || phone.value)

  function applyAuth(d: AuthResult) {
    token.value = d.token
    id.value = d.id
    phone.value = d.phone
    nickname.value = d.nickname
    avatar.value = d.avatar
    localStorage.setItem(k('token'), d.token)
    localStorage.setItem(k('id'), String(d.id))
    localStorage.setItem(k('phone'), d.phone)
    localStorage.setItem(k('nickname'), d.nickname)
    localStorage.setItem(k('avatar'), d.avatar)
  }

  async function login(data: LoginRequest) {
    const res = await userApi.login(data)
    applyAuth(res.data)
  }

  async function register(data: RegisterRequest) {
    const res = await userApi.register(data)
    applyAuth(res.data)
  }

  /** 拉取当前用户资料 */
  async function getInfo() {
    const res = await userApi.me()
    const d = res.data
    id.value = d.id
    phone.value = d.phone
    nickname.value = d.nickname
    avatar.value = d.avatar
    loaded.value = true
    localStorage.setItem(k('phone'), d.phone)
    localStorage.setItem(k('nickname'), d.nickname)
    localStorage.setItem(k('avatar'), d.avatar)
    return d
  }

  function clear() {
    token.value = ''
    id.value = 0
    phone.value = ''
    nickname.value = ''
    avatar.value = ''
    loaded.value = false
    ;['token', 'id', 'phone', 'nickname', 'avatar'].forEach(key =>
      localStorage.removeItem(k(key)),
    )
  }

  async function logout() {
    try {
      await userApi.logout()
    }
    catch {
      /* 即便接口失败也清除本地登录态 */
    }
    clear()
  }

  return {
    token,
    id,
    phone,
    nickname,
    avatar,
    loaded,
    isLogin,
    displayName,
    login,
    register,
    getInfo,
    logout,
    clear,
  }
})
