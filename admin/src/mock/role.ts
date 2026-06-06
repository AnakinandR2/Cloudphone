import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}

function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

interface RoleRec {
  id: number
  name: string
  description: string
  is_builtin: boolean
  permissions: string[]
  user_count: number
  created_at: string
  updated_at: string
}

// 内存角色表（支持增删改查）
const roles: RoleRec[] = [
  {
    id: 1,
    name: '超级管理员',
    description: '拥有系统全部权限',
    is_builtin: true,
    permissions: ['*'],
    user_count: 1,
    created_at: '2026-01-01 00:00:00',
    updated_at: '2026-01-01 00:00:00',
  },
  {
    id: 2,
    name: '编辑',
    description: '可管理用户与内容',
    is_builtin: false,
    permissions: [
      'dashboard:view',
      'staff:view',
      'staff:create',
      'staff:edit',
      'user:view',
      'user:manage',
      'role:view',
      'cloudphone:view',
      'proxy:view',
      'proxy:manage',
      'phone:view',
      'phone:manage',
      'example:view',
      'example:create',
      'example:edit',
    ],
    user_count: 3,
    created_at: '2026-02-15 10:20:00',
    updated_at: '2026-02-15 10:20:00',
  },
  {
    id: 3,
    name: '只读',
    description: '仅查看权限',
    is_builtin: false,
    permissions: [
      'dashboard:view',
      'staff:view',
      'user:view',
      'role:view',
      'access_log:view',
      'cloudphone:view',
      'proxy:view',
      'phone:view',
      'example:view',
    ],
    user_count: 5,
    created_at: '2026-03-20 09:00:00',
    updated_at: '2026-03-20 09:00:00',
  },
]
let roleSeq = 100

const PERMISSION_GROUPS = [
  {
    module: '仪表盘',
    module_key: 'dashboard',
    permissions: [{ key: 'dashboard:view', label: '查看' }],
  },
  {
    module: '管理员管理',
    module_key: 'staff',
    permissions: [
      { key: 'staff:view', label: '查看' },
      { key: 'staff:create', label: '新增' },
      { key: 'staff:edit', label: '编辑' },
      { key: 'staff:delete', label: '删除' },
    ],
  },
  {
    module: '用户管理',
    module_key: 'user',
    permissions: [
      { key: 'user:view', label: '查看' },
      { key: 'user:manage', label: '启用/禁用' },
    ],
  },
  {
    module: '角色管理',
    module_key: 'role',
    permissions: [
      { key: 'role:view', label: '查看' },
      { key: 'role:create', label: '新增' },
      { key: 'role:edit', label: '编辑' },
      { key: 'role:delete', label: '删除' },
    ],
  },
  {
    module: '访问日志',
    module_key: 'access_log',
    permissions: [{ key: 'access_log:view', label: '查看' }],
  },
  {
    module: '云手机资源',
    module_key: 'cloudphone',
    permissions: [{ key: 'cloudphone:view', label: '查看' }],
  },
  {
    module: '代理池管理',
    module_key: 'proxy',
    permissions: [
      { key: 'proxy:view', label: '查看' },
      { key: 'proxy:manage', label: '删除管理' },
    ],
  },
  {
    module: '实例管理',
    module_key: 'phone',
    permissions: [
      { key: 'phone:view', label: '查看' },
      { key: 'phone:manage', label: '删除管理' },
    ],
  },
  {
    module: '示例管理',
    module_key: 'example',
    permissions: [
      { key: 'example:view', label: '查看' },
      { key: 'example:create', label: '新增' },
      { key: 'example:edit', label: '编辑' },
      { key: 'example:delete', label: '删除' },
    ],
  },
]

const ALL_LEAF_KEYS = PERMISSION_GROUPS.flatMap(g => g.permissions.map(p => p.key))

function permCount(r: RoleRec) {
  return r.permissions.includes('*') ? ALL_LEAF_KEYS.length : r.permissions.length
}
function toListItem(r: RoleRec) {
  const { permissions, ...rest } = r
  void permissions
  return { ...rest, permission_count: permCount(r) }
}

/** 供 staff mock 复用的角色简表 */
export const roleBriefs = () => roles.map(r => ({ id: r.id, name: r.name }))

export default defineFakeRoute([
  {
    url: '/v1/role/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const name = (query.name as string) || ''
      let list = roles
      if (name) list = list.filter(r => r.name.includes(name))
      const total = list.length
      const start = (page - 1) * size
      return ok({
        list: list.slice(start, start + size).map(toListItem),
        total,
      })
    },
  },
  {
    url: '/v1/role/permissions',
    method: 'get',
    response: () => ok(PERMISSION_GROUPS),
  },
  {
    url: '/v1/role/:id',
    method: 'get',
    response: ({ params }) => {
      const role = roles.find(r => r.id === Number(params.id))
      return role ? ok(role) : fail('角色不存在')
    },
  },
  {
    url: '/v1/role/create',
    method: 'post',
    response: ({ body }) => {
      const rec: RoleRec = {
        id: ++roleSeq,
        name: body.name,
        description: body.description || '',
        is_builtin: false,
        permissions: body.permissions || [],
        user_count: 0,
        created_at: now(),
        updated_at: now(),
      }
      roles.push(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/role/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const role = roles.find(r => r.id === Number(params.id))
      if (!role) return fail('角色不存在')
      role.name = body.name ?? role.name
      role.description = body.description ?? role.description
      if (body.permissions) role.permissions = body.permissions
      role.updated_at = now()
      return ok(role)
    },
  },
  {
    url: '/v1/role/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = roles.findIndex(r => r.id === Number(params.id))
      if (idx === -1) return fail('角色不存在')
      if (roles[idx].is_builtin) return fail('内置角色不可删除')
      roles.splice(idx, 1)
      return ok(null)
    },
  },
])
