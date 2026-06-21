<script setup lang="ts">
import { ChevronsUpDown, House, LogOut, Settings, User } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useMenuStore } from '@/stores/menu'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'

const props = withDefaults(
  defineProps<{
    /** 折叠态：仅显示头像（用于双层模式的窄图标栏） */
    collapsed?: boolean
    /** 下拉浮层弹出方向 */
    side?: 'top' | 'right' | 'bottom' | 'left'
    align?: 'start' | 'center' | 'end'
  }>(),
  {
    collapsed: false,
    side: 'right',
    align: 'end',
  },
)

const router = useRouter()
const userStore = useUserStore()
const settingsStore = useSettingsStore()
const menuStore = useMenuStore()
const { t } = useI18n()

const displayName = computed(() => userStore.displayName || 'User')

function goHome() {
  router.push(menuStore.firstPath || '/phone')
}

function openPreferences() {
  settingsStore.openPanel()
}

async function logout() {
  await userStore.logout()
  // replace 而非 push，避免后退键回到受保护页。
  router.replace('/login')
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <!-- 折叠态：仅头像（双层模式窄图标栏专用） -->
      <Avatar v-if="props.collapsed" class="mx-auto size-9 cursor-pointer">
        <AvatarImage :src="userStore.avatar" :alt="userStore.phone" />
        <AvatarFallback>
          <User class="text-muted-foreground size-4" />
        </AvatarFallback>
      </Avatar>
      <!-- 展开态：头像 + 名称 + 展开图标；侧栏折叠时自动收起为居中头像 -->
      <button
        v-else
        class="hover:bg-sidebar-accent hover:text-sidebar-accent-foreground flex w-full items-center gap-2 rounded-md p-2 text-left transition-colors group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-1.5"
      >
        <Avatar class="size-8 shrink-0">
          <AvatarImage :src="userStore.avatar" :alt="userStore.phone" />
          <AvatarFallback>
            <User class="text-muted-foreground size-4" />
          </AvatarFallback>
        </Avatar>
        <div class="grid min-w-0 flex-1 leading-tight group-data-[collapsible=icon]:hidden">
          <span class="truncate text-sm font-medium">{{ displayName }}</span>
          <span class="text-muted-foreground truncate text-xs">{{ userStore.phone }}</span>
        </div>
        <ChevronsUpDown class="text-muted-foreground size-4 shrink-0 group-data-[collapsible=icon]:hidden" />
      </button>
    </DropdownMenuTrigger>

    <DropdownMenuContent
      :side="props.side"
      :align="props.align"
      :side-offset="8"
      class="w-60"
    >
      <DropdownMenuLabel class="font-normal">
        <div class="text-muted-foreground mb-2 text-xs">
          {{ t('account.current') }}
        </div>
        <div class="flex items-center gap-2">
          <Avatar class="size-9">
            <AvatarImage :src="userStore.avatar" :alt="userStore.phone" />
            <AvatarFallback>
              <User class="text-muted-foreground size-4" />
            </AvatarFallback>
          </Avatar>
          <div class="grid min-w-0 flex-1 leading-tight">
            <span class="truncate text-sm font-semibold">{{ displayName }}</span>
            <span class="text-muted-foreground truncate text-xs">{{ userStore.phone }}</span>
          </div>
        </div>
      </DropdownMenuLabel>

      <DropdownMenuSeparator />
      <DropdownMenuItem @click="goHome">
        <House class="size-4" />
        {{ t('account.home') }}
      </DropdownMenuItem>
      <DropdownMenuItem @click="openPreferences">
        <Settings class="size-4" />
        {{ t('account.preferences') }}
      </DropdownMenuItem>

      <DropdownMenuSeparator />
      <DropdownMenuItem @click="logout">
        <LogOut class="size-4" />
        {{ t('account.logout') }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
