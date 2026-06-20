// 反馈纯函数辅助单测。node --test tests/
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildMeta, buildVisitorId, genVisitorId, nextVote } from '../composables/useFeedback.ts'

test('buildVisitorId 登录用 u:<id>，不持久化', () => {
  assert.deepEqual(buildVisitorId({ id: 42 }, 'anon-x'), { id: 'u:42', persist: false })
  assert.deepEqual(buildVisitorId({ id: 42 }, null), { id: 'u:42', persist: false })
})

test('buildVisitorId 匿名：有存用存、无存生成并标记持久化', () => {
  assert.deepEqual(buildVisitorId(null, 'anon-x'), { id: 'anon-x', persist: false })
  assert.deepEqual(buildVisitorId(null, null, () => 'gen-1'), { id: 'gen-1', persist: true })
})

test('buildMeta 登录→JSON 字符串(user_id/nickname/phone)，匿名→undefined', () => {
  assert.equal(buildMeta(null), undefined)
  const s = buildMeta({ id: 7, nickname: '阿强', phone: '138****0000' })
  assert.equal(typeof s, 'string')
  assert.deepEqual(JSON.parse(s!), { user_id: 7, nickname: '阿强', phone: '138****0000' })
  // 缺失字段回退空串
  assert.deepEqual(JSON.parse(buildMeta({ id: 9 })!), { user_id: 9, nickname: '', phone: '' })
})

test('nextVote 切换：再次点同向撤销为 0', () => {
  assert.equal(nextVote(0, 1), 1)
  assert.equal(nextVote(1, 1), 0) // 撤销赞
  assert.equal(nextVote(-1, 1), 1) // 踩→赞
  assert.equal(nextVote(0, -1), -1)
  assert.equal(nextVote(-1, -1), 0) // 撤销踩
  assert.equal(nextVote(1, -1), -1) // 赞→踩
})

test('genVisitorId 返回非空且两次不同', () => {
  const a = genVisitorId()
  const b = genVisitorId()
  assert.ok(a.length > 0)
  assert.notEqual(a, b)
})
