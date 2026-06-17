// 纯函数辅助单测。用 Node 内置 test runner（node --test），无需额外依赖。
// 运行：node --test tests/
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatBlogDate, readingMinutes } from '../composables/useBlog.ts'
import { langToApi } from '../server/utils/content.ts'

test('langToApi 映射站点 locale → 中台语言码', () => {
  assert.equal(langToApi('zh'), 'zh-CN')
  assert.equal(langToApi('zh-CN'), 'zh-CN')
  assert.equal(langToApi('en'), 'en')
  assert.equal(langToApi(undefined), 'en')
  assert.equal(langToApi('fr'), 'en') // 未知语言回退英文
})

test('readingMinutes 估算阅读时长（至少 1 分钟）', () => {
  assert.equal(readingMinutes(''), 1)
  assert.equal(readingMinutes('<p></p>'), 1)
  // 200 英文词 ≈ 1 分钟
  const w200 = '<p>' + Array.from({ length: 200 }, () => 'word').join(' ') + '</p>'
  assert.equal(readingMinutes(w200), 1)
  // 401 英文词 → 3 分钟（ceil(401/200)=3）
  const w401 = Array.from({ length: 401 }, () => 'word').join(' ')
  assert.equal(readingMinutes(w401), 3)
  // 中文按字数：800 字 → 2 分钟（ceil(800/400)=2）
  const cjk = '中'.repeat(800)
  assert.equal(readingMinutes(`<p>${cjk}</p>`), 2)
  // HTML 标签被剔除，不计入字数
  assert.equal(readingMinutes('<div><span>hello</span> <b>world</b></div>'), 1)
})

test('formatBlogDate 本地化日期；非法输入返回空串', () => {
  assert.equal(formatBlogDate('', 'en'), '')
  assert.equal(formatBlogDate('not-a-date', 'en'), '')
  const en = formatBlogDate('2026-06-17T17:50:28Z', 'en')
  assert.match(en, /2026/)
  const zh = formatBlogDate('2026-06-17T17:50:28Z', 'zh')
  assert.match(zh, /2026/)
})
