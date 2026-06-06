import type { App, DirectiveBinding } from 'vue'
import { watch } from 'vue'

import { useAuth } from '@/composables/useAuth'

/**
 * 按钮级权限指令：无权限则隐藏元素。
 *   v-auth="'staff:create'"              单个权限
 *   v-auth="['staff:edit','staff:view']"  任一满足
 *   v-auth.all="['staff:edit','staff:delete']"  全部满足
 */
export function setupAuthDirective(app: App) {
  app.directive('auth', (el: HTMLElement, binding: DirectiveBinding) => {
    const { auth, authAll } = useAuth()
    watch(
      () => (binding.modifiers.all ? authAll(binding.value) : auth(binding.value)),
      (ok) => {
        el.style.display = ok ? '' : 'none'
      },
      { immediate: true },
    )
  })
}
