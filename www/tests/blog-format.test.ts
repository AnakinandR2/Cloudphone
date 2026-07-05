// 纯函数辅助单测。用 Node 内置 test runner（node --test），无需额外依赖。
// 运行：node --test tests/
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { blogListPath, formatBlogDate, pageWindow, readingMinutes } from '../composables/useBlog.ts'
import { langToApi, normalizeArticle } from '../server/utils/content.ts'

test('blogListPath 构造伪静态路径', () => {
  assert.equal(blogListPath('all'), '/blog')
  assert.equal(blogListPath('all', undefined, 1), '/blog')
  assert.equal(blogListPath('all', undefined, 3), '/blog/page/3')
  assert.equal(blogListPath('category', 'frontend'), '/blog-categories/frontend')
  assert.equal(blogListPath('category', 'frontend', 2), '/blog-categories/frontend/page/2')
  assert.equal(blogListPath('tag', 'vue'), '/blog-tags/vue')
  assert.equal(blogListPath('tag', 'vue', 5), '/blog-tags/vue/page/5')
})

test('pageWindow 计算页码窗口（含首尾 + 当前页邻域）', () => {
  assert.deepEqual(pageWindow(1, 1), [1])
  assert.deepEqual(pageWindow(1, 3), [1, 2, 3])
  assert.deepEqual(pageWindow(1, 5), [1, 2, '...', 5])
  assert.deepEqual(pageWindow(5, 10), [1, '...', 4, 5, 6, '...', 10])
  assert.deepEqual(pageWindow(10, 10), [1, '...', 9, 10])
  assert.deepEqual(pageWindow(2, 4), [1, 2, 3, 4]) // 相邻不插省略号
})

test('langToApi 映射站点 locale → 中台语言码', () => {
  assert.equal(langToApi('zh'), 'zh-CN')
  assert.equal(langToApi('zh-CN'), 'zh-CN')
  assert.equal(langToApi('en'), 'en')
  assert.equal(langToApi(undefined), 'en')
  assert.equal(langToApi('fr'), 'en') // 未知语言回退英文
})

test('normalizeArticle 把中台 null/缺省 标签兜底为数组（Go 空切片 → JSON null）', () => {
  // 无标签文章：中台回 null → 兜底为 []，避免模板 tags.length 崩溃
  assert.deepEqual(normalizeArticle({ id: 1, title: 't', tags: null }).tags, [])
  // 字段缺省同样兜底
  assert.deepEqual(normalizeArticle({ id: 2, title: 't' }).tags, [])
  // 已是数组时原样保留
  const tags = [{ id: 9, slug: 'go', name: 'Go' }]
  assert.deepEqual(normalizeArticle({ id: 3, title: 't', tags }).tags, tags)
  // 不改动其它字段
  assert.equal(normalizeArticle({ id: 4, title: '你好', tags: null }).title, '你好')
  // 传入空值安全返回（不抛错）
  assert.equal(normalizeArticle(null), null)
  assert.equal(normalizeArticle(undefined), undefined)
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
