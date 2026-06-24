<script setup lang="ts">
import type { AutomationScript } from '@/types/automation'
import { Upload } from 'lucide-vue-next'
import { ref, watch } from 'vue'
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
import LuaEditor from './LuaEditor.vue'
import ParamsSchemaEditor from './ParamsSchemaEditor.vue'

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
const paramsSchema = ref('')
const fileName = ref('')
const saving = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

watch(open, (v) => {
  if (!v)
    return
  name.value = props.script?.name ?? ''
  description.value = props.script?.description ?? ''
  luaContent.value = props.script?.luaContent ?? ''
  paramsSchema.value = props.script?.paramsSchema ?? ''
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
    const body = { name: name.value, description: description.value, luaContent: luaContent.value, paramsSchema: paramsSchema.value, fileName: fileName.value }
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
            <Button variant="outline" size="sm" class="h-7" @click="pickFile">
              <Upload class="size-3.5" /> {{ t('script.upload') }}
            </Button>
            <input ref="fileInput" type="file" accept=".lua,text/*" class="hidden" @change="onFile">
          </div>
          <LuaEditor v-model="luaContent" />
        </div>
        <div class="grid gap-2">
          <Label>{{ t('script.params.title') }} <span class="text-xs text-muted-foreground">{{ t('script.params.hint') }}</span></Label>
          <ParamsSchemaEditor v-model="paramsSchema" />
        </div>
      </div>

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
