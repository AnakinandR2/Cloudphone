import { describe, expect, it } from 'vitest'
import { cn } from './utils'

describe('cn', () => {
  it('合并多个类名', () => {
    expect(cn('px-2', 'py-1')).toBe('px-2 py-1')
  })

  it('后者覆盖前者冲突的 Tailwind 类（twMerge）', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4')
    expect(cn('text-sm', 'text-lg')).toBe('text-lg')
  })

  it('支持条件类名（clsx 语义）', () => {
    expect(cn('base', { active: true, disabled: false })).toBe('base active')
    expect(cn('a', false, null, undefined, 'b')).toBe('a b')
  })

  it('支持数组入参', () => {
    expect(cn(['px-2', 'py-1'], 'px-4')).toBe('py-1 px-4')
  })

  it('空输入返回空串', () => {
    expect(cn()).toBe('')
  })
})
