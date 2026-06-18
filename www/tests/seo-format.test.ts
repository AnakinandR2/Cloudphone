// SEO 配置纯函数单测（node --test）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { seoMetasToHead } from '../composables/useSeoConfig.ts'

test('seoMetasToHead 区分 title / name / property', () => {
  const { title, meta } = seoMetasToHead([
    { key: '__title__', value: '博客 - Gloryphone', attr: '' },
    { key: 'description', value: 'desc', attr: 'name' },
    { key: 'og:type', value: 'article', attr: 'property' },
    { key: 'robots', value: 'index,follow' }, // 缺省按 name
  ])
  assert.equal(title, '博客 - Gloryphone')
  assert.deepEqual(meta, [
    { name: 'description', content: 'desc' },
    { property: 'og:type', content: 'article' },
    { name: 'robots', content: 'index,follow' },
  ])
})

test('seoMetasToHead 空/无 key 安全', () => {
  const { title, meta } = seoMetasToHead([])
  assert.equal(title, undefined)
  assert.deepEqual(meta, [])
  // @ts-expect-error 容错：脏数据
  const r = seoMetasToHead([null, { value: 'x' }])
  assert.deepEqual(r.meta, [])
})
