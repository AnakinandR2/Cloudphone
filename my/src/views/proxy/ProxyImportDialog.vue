<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import proxyApi from '@/api/modules/proxy'
import { parseProxyImport } from '@/utils/proxyParse'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const raw = ref('')
const submitting = ref(false)

// 支持的格式示例（技术文本，含 @ / ://，不走 i18n —— vue-i18n 会把 @ 当链接语法报错）
// 首列可选「名称,」——不填则名称自动取 host:port。
const FORMATS = [
  'name,host:port',
  'host:port',
  'host:port:user:pass',
  'socks5://user:pass@host:port',
  'user:pass@host:port',
].join('\n')

// 解析逻辑抽到 @/utils/proxyParse（纯函数，便于单测）：区分有效条目与无法识别的行号。
const result = computed(() => parseProxyImport(raw.value))
const parsed = computed(() => result.value.items)
const invalidRows = computed(() => result.value.invalidRows)

watch(open, (v) => {
  if (v) {
    raw.value = ''
    submitting.value = false
  }
})

async function submit() {
  if (parsed.value.length === 0) {
    toast.error(t('proxy.importEmpty'))
    return
  }
  submitting.value = true
  try {
    const res = await proxyApi.batch(parsed.value)
    toast.success(t('proxy.importOk', { created: res.data.created, total: res.data.total }))
    open.value = false
    emit('success')
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>{{ t('proxy.importTitle') }}</DialogTitle>
        <DialogDescription>{{ t('proxy.importDesc') }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-3 py-2">
        <pre class="bg-muted/50 text-muted-foreground rounded-md p-2 text-xs leading-relaxed">{{ FORMATS }}</pre>
        <Textarea v-model="raw" :placeholder="t('proxy.importPlaceholder')" class="min-h-48 font-mono text-sm" />
        <p class="text-muted-foreground text-xs">
          {{ t('proxy.importParsed', { n: parsed.length }) }}
        </p>
        <!-- 无法识别的行提示（#48）：避免非法行被静默丢弃，导入前给出行号。 -->
        <p v-if="invalidRows.length" class="text-destructive text-xs">
          {{ t('proxy.importInvalidRows', { n: invalidRows.length, rows: invalidRows.join('、') }) }}
        </p>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button :disabled="submitting || parsed.length === 0" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
