import { describe, expect, it } from 'vitest'
import { TAG_COLORS, tagClass } from './tagColor'

describe('tagColor (admin)', () => {
  it('TAG_COLORS 为九个预设色且以 slate 起始', () => {
    expect(TAG_COLORS[0]).toBe('slate')
    expect(TAG_COLORS).toContain('teal')
    expect(TAG_COLORS).toHaveLength(9)
  })

  it('tagClass 返回已知色的 badge 类', () => {
    expect(tagClass('violet')).toContain('bg-violet-100')
    expect(tagClass('amber')).toContain('text-amber-700')
  })

  it('tagClass 对未知/缺省回退到 slate', () => {
    expect(tagClass('xxx')).toBe(tagClass('slate'))
    expect(tagClass(undefined)).toBe(tagClass('slate'))
    expect(tagClass('')).toBe(tagClass('slate'))
  })

  it('每个预设色都映射到对应颜色的 badge 类', () => {
    for (const c of TAG_COLORS) {
      expect(tagClass(c)).toContain(c)
    }
  })
})
