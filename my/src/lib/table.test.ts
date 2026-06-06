import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { valueUpdater } from './table'

describe('valueUpdater', () => {
  it('直接赋值（updater 为普通值）', () => {
    const r = ref(1)
    valueUpdater(2, r)
    expect(r.value).toBe(2)
  })

  it('函数式更新（updater 为函数，基于旧值计算）', () => {
    const r = ref(10)
    valueUpdater(old => old + 5, r)
    expect(r.value).toBe(15)
  })

  it('支持对象状态的函数式更新', () => {
    const r = ref<Record<string, boolean>>({ a: true })
    valueUpdater(old => ({ ...old, b: false }), r)
    expect(r.value).toEqual({ a: true, b: false })
  })
})
