import { useUserStore } from '@/stores/user'

/**
 * 权限判断。权限码格式 `resource:action`，特殊值：
 *   '*'         —— 拥有该权限即拥有全部权限
 *   'superuser' —— 是否超级管理员
 */
export function useAuth() {
  function hasPermission(permission: string): boolean {
    const userStore = useUserStore()
    if (permission === 'superuser') {
      return userStore.isSuperuser
    }
    return (
      userStore.permissions.includes('*')
      || userStore.permissions.includes(permission)
    )
  }

  /** 任一满足（OR）；传空则放行 */
  function auth(value: string | string[]): boolean {
    if (typeof value === 'string') {
      return value !== '' ? hasPermission(value) : true
    }
    return value.length > 0 ? value.some(v => hasPermission(v)) : true
  }

  /** 全部满足（AND） */
  function authAll(value: string[]): boolean {
    return value.length > 0 ? value.every(v => hasPermission(v)) : true
  }

  return { auth, authAll, hasPermission }
}
