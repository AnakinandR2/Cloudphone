<script setup lang="ts">
import { TriangleAlert } from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

withDefaults(
  defineProps<{
    title?: string
    confirmText?: string
    cancelText?: string
    /** 弹出方向 */
    side?: 'top' | 'right' | 'bottom' | 'left'
    align?: 'start' | 'center' | 'end'
    /** 语气：danger=危险/不可逆(红)；warning=警告/可逆但有影响(琥珀) */
    tone?: 'danger' | 'warning'
  }>(),
  { side: 'top', align: 'end', tone: 'danger' },
)

const emit = defineEmits<{ confirm: [] }>()
const { t } = useI18n()
const open = ref(false)

function onConfirm() {
  open.value = false
  emit('confirm')
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <slot />
    </PopoverTrigger>
    <PopoverContent :side="side" :align="align" class="w-64 p-3">
      <div class="flex gap-2">
        <TriangleAlert class="text-amber-500 mt-0.5 size-4 shrink-0" />
        <p class="text-sm">
          {{ title }}
        </p>
      </div>
      <div class="mt-3 flex justify-end gap-2">
        <Button variant="outline" size="sm" class="h-7 px-2.5" @click="open = false">
          {{ cancelText || t('crud.cancel') }}
        </Button>
        <Button
          v-if="tone === 'warning'"
          variant="warning"
          size="sm"
          class="h-7 px-2.5"
          @click="onConfirm"
        >
          {{ confirmText || t('crud.confirm') }}
        </Button>
        <Button v-else variant="destructive" size="sm" class="h-7 px-2.5" @click="onConfirm">
          {{ confirmText || t('crud.confirm') }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
