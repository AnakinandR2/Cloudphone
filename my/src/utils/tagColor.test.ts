import { describe, expect, it } from 'vitest'
import { TAG_COLORS, tagClass, tagDot } from './tagColor'

describe('tagColor', () => {
  it('TAG_COLORS 为九个预设色且以 slate 起始', () => {
    expect(TAG_COLORS[0]).toBe('slate')
    expect(TAG_COLORS).toContain('teal')
    expect(TAG_COLORS).toHaveLength(9)
  })

  it('tagClass 返回已知色的 badge 类', () => {
    expect(tagClass('teal')).toContain('bg-teal-100')
    expect(tagClass('red')).toContain('text-red-700')
  })

  it('tagClass 对未知/缺省/空串回退到 slate', () => {
    expect(tagClass('not-a-color')).toBe(tagClass('slate'))
    expect(tagClass(undefined)).toBe(tagClass('slate'))
    expect(tagClass('')).toBe(tagClass('slate'))
  })

  it('tagDot 返回已知色圆点类并回退', () => {
    expect(tagDot('green')).toBe('bg-green-500')
    expect(tagDot('nope')).toBe('bg-slate-400')
    expect(tagDot(undefined)).toBe('bg-slate-400')
  })

  it('每个预设色都映射到对应颜色的 badge 与 dot 类', () => {
    for (const c of TAG_COLORS) {
      expect(tagClass(c)).toContain(c)
      expect(tagDot(c)).toContain(c)
    }
  })
})
