import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// 与后端 toolCatalog() 同形：按领域分组、仅工具 code。共 18 个，与开放 API 能力一致。
const groups = [
  { group: 'phone', tools: ['list_phones', 'get_phone', 'create_phone', 'destroy_phone', 'power_phone', 'restart_phone', 'bind_proxy'] },
  { group: 'app', tools: ['list_apps', 'list_installed_apps', 'install_app', 'uninstall_app'] },
  { group: 'script', tools: ['list_scripts', 'run_script', 'get_task'] },
  { group: 'proxy', tools: ['list_proxies', 'create_proxy', 'update_proxy', 'delete_proxy'] },
]

export default defineFakeRoute([
  {
    url: '/v1/mcp/tools',
    method: 'get',
    response: () => ok(groups),
  },
])
