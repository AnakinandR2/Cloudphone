<script setup lang="ts">
import type { AutomationScript } from '@/types/automation'
import { Upload } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import automationApi from '@/api/modules/automation'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { extractSchemaComment, parseSchema } from '@/utils/paramsComment'
import LuaEditor from './LuaEditor.vue'
import ParamsSchemaDialog from './ParamsSchemaDialog.vue'

const props = defineProps<{
  script: AutomationScript | null // 非空=编辑
  store?: boolean // true=运营商店脚本（走 admin api 由父层处理；此处仅 my）
}>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()

const name = ref('')
const description = ref('')
const luaContent = ref('')
const fileName = ref('')
const saving = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const paramsDialogOpen = ref(false)

// 参数个数：现从 luaContent 顶部注释解析（脚本注释是唯一真源）。
const paramCount = computed(() => parseSchema(extractSchemaComment(luaContent.value) || '').length)

watch(open, (v) => {
  if (!v)
    return
  name.value = props.script?.name ?? ''
  description.value = props.script?.description ?? ''
  luaContent.value = props.script?.luaContent ?? ''
  fileName.value = props.script?.fileName ?? ''
})

function pickFile() {
  fileInput.value?.click()
}
async function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f)
    return
  luaContent.value = await f.text()
  fileName.value = f.name
  if (!name.value)
    name.value = f.name.replace(/\.lua$/i, '')
  // 上传的 .lua 顶部注释随 luaContent 一起进来；参数计数 computed 会自动反映。
}

async function save() {
  if (!name.value.trim()) {
    toast.error(t('script.errName'))
    return
  }
  if (!luaContent.value.trim()) {
    toast.error(t('script.errContent'))
    return
  }
  saving.value = true
  try {
    // 参数已由参数对话框写进 luaContent 顶部注释，直接上传（后端从注释推导校验）。
    const body = { name: name.value, description: description.value, luaContent: luaContent.value, fileName: fileName.value }
    if (props.script)
      await automationApi.updateScript(props.script.id, body)
    else
      await automationApi.createScript(body)
    toast.success(t('script.saveOk'))
    open.value = false
    emit('saved')
  }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[88vh] flex-col gap-0 p-0 sm:max-w-2xl">
      <DialogHeader class="border-b p-4">
        <DialogTitle>{{ script ? t('script.editTitle') : t('script.newScript') }}</DialogTitle>
        <DialogDescription>{{ t('script.editDesc') }}</DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
        <div class="grid gap-2">
          <Label>{{ t('script.colName') }}</Label>
          <Input v-model="name" :placeholder="t('script.namePh')" />
        </div>
        <div class="grid gap-2">
          <Label>{{ t('script.colDesc') }}</Label>
          <Textarea v-model="description" :rows="2" :placeholder="t('script.descPh')" />
        </div>
        <div class="grid gap-2">
          <div class="flex items-center justify-between">
            <Label>{{ t('script.code') }} <span class="text-xs text-muted-foreground">(Lua)</span></Label>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" class="h-7" @click="paramsDialogOpen = true">
                {{ t('script.params.edit') }}<span class="ml-1 text-muted-foreground">({{ paramCount || t('script.params.none') }})</span>
              </Button>
              <Button variant="outline" size="sm" class="h-7" @click="pickFile">
                <Upload class="size-3.5" /> {{ t('script.upload') }}
              </Button>
            </div>
            <input ref="fileInput" type="file" accept=".lua,text/*" class="hidden" @change="onFile">
          </div>
          <LuaEditor v-model="luaContent" />
        </div>
      </div>

      <ParamsSchemaDialog v-model:open="paramsDialogOpen" :lua="luaContent" @saved="(v) => (luaContent = v)" />

      <DialogFooter class="border-t p-3">
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button :disabled="saving" @click="save">
          {{ t('crud.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
