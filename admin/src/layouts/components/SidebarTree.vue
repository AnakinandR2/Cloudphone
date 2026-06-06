<script setup lang="ts">
import type { AppRoute } from '@/router/routes'
import { ChevronRight } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { RouterLink, useRoute } from 'vue-router'
import Icon from '@/components/Icon.vue'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar'

const props = withDefaults(
  defineProps<{ nodes: AppRoute[], level?: number }>(),
  { level: 0 },
)

const route = useRoute()
const { t } = useI18n()

// 顶层用 MenuItem/MenuButton，子层用 SubItem/SubButton
const ItemComp = computed(() =>
  props.level === 0 ? SidebarMenuItem : SidebarMenuSubItem,
)
const BtnComp = computed(() =>
  props.level === 0 ? SidebarMenuButton : SidebarMenuSubButton,
)

function containsActive(node: AppRoute): boolean {
  if (node.path && (route.path === node.path || route.path.startsWith(`${node.path}/`))) {
    return true
  }
  return node.children?.some(containsActive) ?? false
}
</script>

<template>
  <template v-for="node in nodes" :key="node.path ?? node.name">
    <!-- 分支：可折叠（支持任意层级递归） -->
    <Collapsible
      v-if="node.children && node.children.length"
      as-child
      :default-open="containsActive(node)"
      class="group/collapsible"
    >
      <component :is="ItemComp">
        <CollapsibleTrigger as-child>
          <component :is="BtnComp">
            <Icon v-if="node.meta?.icon" :name="node.meta.icon" />
            <span>{{ t(node.meta?.title ?? "") }}</span>
            <ChevronRight
              class="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90"
            />
          </component>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            <SidebarTree :nodes="node.children" :level="level + 1" />
          </SidebarMenuSub>
        </CollapsibleContent>
      </component>
    </Collapsible>

    <!-- 叶子：路由链接 -->
    <component :is="ItemComp" v-else-if="node.path">
      <component
        :is="BtnComp"
        as-child
        :is-active="route.path === node.path"
        class="data-[active=true]:bg-sidebar-primary/10 data-[active=true]:text-sidebar-primary data-[active=true]:font-semibold"
      >
        <RouterLink :to="node.path">
          <Icon v-if="node.meta?.icon" :name="node.meta.icon" />
          <span>{{ t(node.meta?.title ?? "") }}</span>
        </RouterLink>
      </component>
    </component>
  </template>
</template>
