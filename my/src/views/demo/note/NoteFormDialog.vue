<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import noteApi from '@/api/modules/note'
import { formatDateTime } from '@/utils/date'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{ id: number, mode: 'create' | 'edit' | 'view' }>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')
const submitting = ref(false)
const titleError = ref('')

const form = reactive({ title: '', content: '', created_at: '', updated_at: '' })

const dialogTitle = computed(() =>
  props.mode === 'view'
    ? t('note.viewTitle')
    : isCreate.value
      ? t('note.createTitle')
      : t('note.editTitle'),
)

watch(open, async (v) => {
  if (v) {
    form.title = ''
    form.content = ''
    form.created_at = ''
    form.updated_at = ''
    titleError.value = ''
    if (props.id !== 0) {
      const res = await noteApi.detail(props.id)
      Object.assign(form, res.data)
    }
  }
})

async function submit() {
  titleError.value = ''
  if (!form.title.trim()) {
    titleError.value = t('note.errTitleRequired')
    return
  }
  submitting.value = true
  try {
    if (isCreate.value) {
      await noteApi.create({ title: form.title, content: form.content })
      toast.success(t('crud.createOk'))
    }
    else {
      await noteApi.update(props.id, { title: form.title, content: form.content })
      toast.success(t('crud.updateOk'))
    }
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
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ dialogTitle }}</DialogTitle>
        <DialogDescription class="sr-only">{{ dialogTitle }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="n-title">{{ t('note.fTitle') }}</Label>
          <Input id="n-title" v-model="form.title" :disabled="readonly" :placeholder="t('note.fTitlePlaceholder')" />
          <p v-if="titleError" class="text-destructive text-xs">{{ titleError }}</p>
        </div>
        <div class="space-y-2">
          <Label for="n-content">{{ t('note.fContent') }}</Label>
          <Textarea id="n-content" v-model="form.content" :disabled="readonly" :rows="5" :placeholder="t('note.fContentPlaceholder')" />
        </div>
        <div v-if="!isCreate" class="text-muted-foreground grid grid-cols-2 gap-2 text-xs">
          <span>{{ t('table.createdAt') }}: {{ formatDateTime(form.created_at) }}</span>
          <span>{{ t('note.updatedAt') }}: {{ formatDateTime(form.updated_at) }}</span>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ readonly ? t('crud.close') : t('crud.cancel') }}
        </Button>
        <Button v-if="!readonly" :disabled="submitting" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
