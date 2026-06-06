import type { Router } from 'vue-router'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, reactive } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useQuerySync } from './useQuerySync'

async function mountWith(
  query: Record<string, string>,
  state: Record<string, string>,
  defaults: Record<string, string>,
) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', name: 'home', component: { render: () => null } }],
  })
  router.push({ path: '/', query })
  await router.isReady()

  const Comp = defineComponent({
    setup() {
      useQuerySync(state, defaults)
      return () => null
    },
  })
  mount(Comp, { global: { plugins: [router] } })
  return router
}

function q(router: Router) {
  return router.currentRoute.value.query
}

describe('useQuerySync', () => {
  it('初始化时用 URL query 填充 state', async () => {
    const state = reactive({ name: '', status: '' })
    await mountWith(
      { name: 'foo', status: 'active' },
      state,
      { name: '', status: 'all' },
    )
    expect(state.name).toBe('foo')
    expect(state.status).toBe('active')
  })

  it('state 变化写回 URL，等于默认值的项不写入', async () => {
    const state = reactive({ name: '', status: 'all' })
    const router = await mountWith({}, state, { name: '', status: 'all' })

    state.name = 'bar'
    await flushPromises()
    expect(q(router).name).toBe('bar')
    // status 仍为默认值 all → 不应出现在 URL
    expect(q(router).status).toBeUndefined()

    state.status = 'active'
    await flushPromises()
    expect(q(router).status).toBe('active')

    // 改回默认值后从 URL 移除
    state.name = ''
    await flushPromises()
    expect(q(router).name).toBeUndefined()
  })

  it('保留与筛选无关的其它 query 参数', async () => {
    const state = reactive({ name: '' })
    const router = await mountWith({ tab: 'x' }, state, { name: '' })

    state.name = 'z'
    await flushPromises()
    expect(q(router).name).toBe('z')
    expect(q(router).tab).toBe('x')
  })
})
