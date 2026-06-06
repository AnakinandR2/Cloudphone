<script setup lang="ts">
import { Command, LogOut, Moon, Sun, User } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'

import Breadcrumbs from '@/components/Breadcrumbs.vue'
import Icon from '@/components/Icon.vue'

import LocaleSwitcher from '@/components/LocaleSwitcher.vue'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import { useMenuStore } from '@/stores/menu'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'
import AccountMenu from './components/AccountMenu.vue'
import SidebarTree from './components/SidebarTree.vue'

const menuStore = useMenuStore()
const userStore = useUserStore()
const settingsStore = useSettingsStore()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const appTitle = import.meta.env.VITE_APP_TITLE || '管理后台'
const isDouble = computed(() => settingsStore.settings.menuMode === 'double')
const activeTitle = computed(() => {
  const key = menuStore.mainMenus[menuStore.active]?.meta.title
  return key ? t(key) : '管理后台'
})
const transitionName = computed(() =>
  settingsStore.settings.pageTransition === 'none'
    ? ''
    : settingsStore.settings.pageTransition,
)

const providerStyle = computed(() =>
  isDouble.value
    ? { '--sidebar-width': 'calc(5rem + 14rem)', '--sidebar-width-icon': '5rem' }
    : { '--sidebar-width': '15rem', '--sidebar-width-icon': '3rem' },
)

function logout() {
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <SidebarProvider :style="providerStyle">
    <Sidebar collapsible="icon" class="overflow-hidden *:data-[sidebar=sidebar]:flex-row">
      <!-- 第一层：图标栏（仅双层模式） -->
      <Sidebar
        v-if="isDouble"
        collapsible="none"
        class="w-[calc(var(--sidebar-width-icon)+1px)]! border-r"
      >
        <SidebarHeader class="items-center p-3">
          <RouterLink
            to="/"
            :title="appTitle"
            class="bg-sidebar-primary text-sidebar-primary-foreground flex size-9 items-center justify-center rounded-lg transition-transform hover:scale-105"
          >
            <Command class="size-5" />
          </RouterLink>
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup class="px-1.5">
            <SidebarGroupContent>
              <SidebarMenu class="gap-1">
                <SidebarMenuItem v-for="(group, index) in menuStore.mainMenus" :key="index">
                  <SidebarMenuButton
                    :tooltip="t(group.meta.title)"
                    :is-active="index === menuStore.active"
                    class="h-auto flex-col gap-1 py-2 text-xs data-[active=true]:bg-sidebar-primary data-[active=true]:text-sidebar-primary-foreground data-[active=true]:hover:bg-sidebar-primary data-[active=true]:hover:text-sidebar-primary-foreground data-[active=true]:shadow-sm"
                    @click="menuStore.setActive(index)"
                  >
                    <Icon :name="group.meta.icon" class="size-5!" />
                    <span class="text-center leading-tight">{{ t(group.meta.title) }}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter class="items-center p-2">
          <!-- 双层模式：窄图标栏 → 仅头像，下拉向右弹出 -->
          <AccountMenu collapsed side="right" align="end" />
        </SidebarFooter>
      </Sidebar>

      <!-- 第二层：子菜单（双层=当前分组；单层=全部分组带标题） -->
      <Sidebar collapsible="none" class="flex flex-1">
        <SidebarHeader class="h-14 justify-center border-b p-0">
          <div
            class="flex h-14 items-center gap-2 px-4 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
          >
            <!-- 单层模式：图标 + 站点名作为品牌标识，点击回首页 -->
            <RouterLink
              v-if="!isDouble"
              to="/"
              class="flex items-center gap-2 transition-opacity hover:opacity-80"
            >
              <div
                class="bg-sidebar-primary text-sidebar-primary-foreground flex size-8 shrink-0 items-center justify-center rounded-lg"
              >
                <Command class="size-4" />
              </div>
              <span class="truncate text-sm font-semibold group-data-[collapsible=icon]:hidden">
                {{ appTitle }}
              </span>
            </RouterLink>
            <!-- 双层模式：显示当前分组标题（非品牌，不跳首页） -->
            <span v-else class="truncate text-sm font-semibold">
              {{ activeTitle }}
            </span>
          </div>
        </SidebarHeader>
        <SidebarContent>
          <!-- 双层：仅当前分组 -->
          <template v-if="isDouble">
            <SidebarGroup>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarTree :nodes="menuStore.sidebarMenus" />
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </template>
          <!-- 单层：所有分组 + 分组标题 -->
          <template v-else>
            <SidebarGroup v-for="(group, index) in menuStore.mainMenus" :key="index">
              <SidebarGroupLabel>{{ t(group.meta.title) }}</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarTree :nodes="group.children" />
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </template>
        </SidebarContent>
        <!-- 单层模式：在唯一侧边栏底部放置完整账号按钮，下拉向上弹出 -->
        <SidebarFooter v-if="!isDouble" class="border-t p-2">
          <AccountMenu side="top" align="start" />
        </SidebarFooter>
      </Sidebar>
    </Sidebar>

    <!-- 主区域 -->
    <SidebarInset>
      <header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
        <SidebarTrigger class="-ml-1" />
        <Separator orientation="vertical" class="h-4!" />
        <Breadcrumbs />
        <div class="flex-1" />
        <LocaleSwitcher />
        <Button variant="ghost" size="icon" :title="t('header.toggleTheme')" @click="settingsStore.toggleDark()">
          <Moon v-if="!settingsStore.isDark" class="size-4" />
          <Sun v-else class="size-4" />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <button class="hover:bg-accent flex items-center gap-2 rounded-md px-2 py-1">
              <Avatar class="size-8">
                <AvatarImage :src="userStore.avatar" :alt="userStore.account" />
                <AvatarFallback>
                  <User class="text-muted-foreground size-4" />
                </AvatarFallback>
              </Avatar>
              <span class="text-sm font-medium">{{ userStore.name || userStore.account }}</span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-44">
            <DropdownMenuLabel>{{ userStore.name || userStore.account }}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem @click="logout">
              <LogOut class="size-4" />
              {{ t('header.logout') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </header>
      <main class="bg-muted/30 flex-1 overflow-auto p-6">
        <RouterView v-slot="{ Component }">
          <Transition :name="transitionName" mode="out-in" appear>
            <component :is="Component" :key="route.path" />
          </Transition>
        </RouterView>
      </main>
    </SidebarInset>
  </SidebarProvider>
</template>
