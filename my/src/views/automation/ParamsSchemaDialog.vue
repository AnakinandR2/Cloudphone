<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { extractSchemaComment, parseSchema, upsertSchemaComment } from '@/utils/paramsComment'
import ParamsSchemaEditor from './ParamsSchemaEditor.vue'

// 独立的参数编辑对话框：打开时从脚本顶部注释反序列化，保存时序列化写回脚本草稿。
const props = defineProps<{ lua: string }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [lua: string] }>()
const { t } = useI18n()

const schema = ref('') // 我们的数组格式 JSON（ParamsSchemaEditor 的载体）

watch(open, (v) => {
  if (!v)
    return
  const inner = extractSchemaComment(props.lua)
  schema.value = inner ? JSON.stringify(parseSchema(inner)) : ''
})

function save() {
  const specs = schema.value ? parseSchema(schema.value) : []
  emit('saved', upsertSchemaComment(props.lua, specs))
  open.value = false
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[88vh] flex-col gap-0 p-0 sm:max-w-2xl">
      <DialogHeader class="border-b p-4">
        <DialogTitle>{{ t('script.params.title') }}</DialogTitle>
        <DialogDescription>{{ t('script.params.hint') }}</DialogDescription>
      </DialogHeader>
      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        <ParamsSchemaEditor v-model="schema" />
      </div>
      <DialogFooter class="border-t p-3">
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button @click="save">
          {{ t('crud.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
