<script setup lang="ts">
import type { CloudPhone } from '@/types/phone'
import { useI18n } from 'vue-i18n'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import RemoteAppPanel from './RemoteAppPanel.vue'

defineProps<{ phone: CloudPhone | null }>()
const open = defineModel<boolean>({ default: false })

const { t } = useI18n()
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent side="right" class="flex w-full flex-col gap-0 p-0 sm:max-w-md">
      <SheetHeader class="border-b">
        <SheetTitle>
          {{ t('phone.app.title') }}<span v-if="phone" class="text-muted-foreground"> — {{ phone.name }}</span>
        </SheetTitle>
        <SheetDescription>{{ t('phone.app.desc') }}</SheetDescription>
      </SheetHeader>
      <!-- 与远程控制一致：我的应用 / 应用市场 / 已安装 三个 tab -->
      <div v-if="open && phone" class="min-h-0 flex-1">
        <RemoteAppPanel :phone-ids="[phone.id]" :master-id="phone.id" />
      </div>
    </SheetContent>
  </Sheet>
</template>
