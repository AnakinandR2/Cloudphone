import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { useUserStore } from '@/stores/user'
import { useAuth } from './useAuth'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('useAuth.hasPermission', () => {
  it('superuser 特判：取决于 isSuperuser 标记', () => {
    const store = useUserStore()
    const { hasPermission } = useAuth()

    store.isSuperuser = false
    expect(hasPermission('superuser')).toBe(false)
    store.isSuperuser = true
    expect(hasPermission('superuser')).toBe(true)
  })

  it('通配符 * 拥有全部权限', () => {
    const store = useUserStore()
    store.permissions = ['*']
    const { hasPermission } = useAuth()
    expect(hasPermission('staff:view')).toBe(true)
    expect(hasPermission('anything:else')).toBe(true)
  })

  it('精确匹配具体权限码', () => {
    const store = useUserStore()
    store.permissions = ['staff:view', 'role:view']
    const { hasPermission } = useAuth()
    expect(hasPermission('staff:view')).toBe(true)
    expect(hasPermission('staff:edit')).toBe(false)
  })
})

describe('useAuth.auth (OR)', () => {
  beforeEach(() => {
    useUserStore().permissions = ['staff:view']
  })

  it('空字符串 / 空数组放行', () => {
    const { auth } = useAuth()
    expect(auth('')).toBe(true)
    expect(auth([])).toBe(true)
  })

  it('字符串：单权限判定', () => {
    const { auth } = useAuth()
    expect(auth('staff:view')).toBe(true)
    expect(auth('staff:edit')).toBe(false)
  })

  it('数组：任一满足即可', () => {
    const { auth } = useAuth()
    expect(auth(['staff:edit', 'staff:view'])).toBe(true)
    expect(auth(['staff:edit', 'role:edit'])).toBe(false)
  })
})

describe('useAuth.authAll (AND)', () => {
  beforeEach(() => {
    useUserStore().permissions = ['staff:view', 'role:view']
  })

  it('空数组放行', () => {
    expect(useAuth().authAll([])).toBe(true)
  })

  it('需全部满足', () => {
    const { authAll } = useAuth()
    expect(authAll(['staff:view', 'role:view'])).toBe(true)
    expect(authAll(['staff:view', 'role:edit'])).toBe(false)
  })
})
