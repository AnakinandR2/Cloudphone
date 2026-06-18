// 帮助文档纯函数单测（node --test）。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildToc, firstArticleUrl } from '../composables/useHelp.ts'

test('buildToc 给 h2/h3 注入锚点 id 并产出 TOC', () => {
  const { html, toc } = buildToc('<h2>Quick Start</h2><p>x</p><h3>Sub Section</h3>')
  assert.equal(toc.length, 2)
  assert.deepEqual(toc[0], { id: 'quick-start', text: 'Quick Start', level: 2 })
  assert.deepEqual(toc[1], { id: 'sub-section', text: 'Sub Section', level: 3 })
  assert.match(html, /<h2 id="quick-start">Quick Start<\/h2>/)
  assert.match(html, /<h3 id="sub-section">Sub Section<\/h3>/)
})

test('buildToc 重复标题 id 去重', () => {
  const { toc } = buildToc('<h2>Setup</h2><h2>Setup</h2>')
  assert.deepEqual(toc.map((t) => t.id), ['setup', 'setup-1'])
})

test('buildToc 纯 CJK 标题回退 sec 并保留原文本', () => {
  const { html, toc } = buildToc('<h2>内容中台是什么</h2><h2>下一步</h2>')
  assert.deepEqual(toc.map((t) => t.id), ['sec', 'sec-1'])
  assert.deepEqual(toc.map((t) => t.text), ['内容中台是什么', '下一步'])
  assert.match(html, /id="sec"/)
})

test('buildToc 沿用已有 id', () => {
  const { html, toc } = buildToc('<h2 id="custom">A</h2>')
  assert.equal(toc[0].id, 'custom')
  assert.equal(html, '<h2 id="custom">A</h2>')
})

test('buildToc 跳过空标题', () => {
  const { toc } = buildToc('<h2></h2><h2>Real</h2>')
  assert.deepEqual(toc.map((t) => t.text), ['Real'])
})

test('firstArticleUrl 深度优先取第一篇 url', () => {
  const tree = [
    { kind: 'group', title: 'G1', slug: 'g1', collapsed: false, children: [
      { kind: 'article', title: 'A1', slug: 'a1', url: '/help/a1', collapsed: false },
      { kind: 'article', title: 'A2', slug: 'a2', url: '/help/a2', collapsed: false },
    ] },
  ]
  assert.equal(firstArticleUrl(tree), '/help/a1')
  assert.equal(firstArticleUrl([]), undefined)
})
