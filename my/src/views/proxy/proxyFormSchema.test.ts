import { describe, expect, it } from 'vitest'
import { buildProxyFormSchema } from './proxyFormSchema'

const t = (k: string) => k
const schema = buildProxyFormSchema(t)

const valid = { name: '香港节点', host: '1.2.3.4', port: 1080 }

function errorFor(input: unknown, field: string) {
  const r = schema.safeParse(input)
  if (r.success)
    return undefined
  return r.error.issues.find(i => i.path[0] === field)?.message
}

describe('proxyFormSchema', () => {
  it('合法数据通过（port 数字或字符串均可）', () => {
    expect(schema.safeParse(valid).success).toBe(true)
    expect(schema.safeParse({ ...valid, port: '8080' }).success).toBe(true)
  })

  it('name 为空报必填', () => {
    expect(errorFor({ ...valid, name: '   ' }, 'name')).toBe('proxy.errNameRequired')
  })

  it('host 为空报必填', () => {
    expect(errorFor({ ...valid, host: '' }, 'host')).toBe('proxy.errHostRequired')
  })

  it('port 为空/非数字报必填', () => {
    expect(errorFor({ ...valid, port: '' }, 'port')).toBe('proxy.errPortRequired')
    expect(errorFor({ ...valid, port: 'abc' }, 'port')).toBe('proxy.errPortRequired')
    expect(errorFor({ ...valid, port: 0 }, 'port')).toBe('proxy.errPortRequired')
  })

  it('port 越界报范围错误', () => {
    expect(errorFor({ ...valid, port: 70000 }, 'port')).toBe('proxy.errPortRange')
    expect(errorFor({ ...valid, port: 80.5 }, 'port')).toBe('proxy.errPortRange')
  })
})
