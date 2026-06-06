<script setup lang="ts">
import type { Tag } from '@/types/phone'
import { Plus, X } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import { Badge } from '@/components/ui/badge'
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
import { TAG_COLORS, tagClass, tagDot } from '@/utils/tagColor'

const props = defineProps<{ ids: number[], initial?: Tag[] }>()
const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>()
const { t } = useI18n()

const allTags = ref<Tag[]>([])
const selected = ref<Tag[]>([])
const input = ref('')
const currentColor = ref<string>(TAG_COLORS[0])
const saving = ref(false)

watch(open, async (v) => {
  if (!v)
    return
  selected.value = (props.initial ?? []).map(tg => ({ ...tg }))
  input.value = ''
  currentColor.value = TAG_COLORS[0]
  try {
    allTags.value = (await phoneApi.listTags()).data ?? []
  }
  catch {
    allTags.value = []
  }
})

const suggestions = computed(() => allTags.value.filter(tg => !selected.value.some(s => s.name === tg.name)))

function addTag(name: string, color: string) {
  const v = name.trim()
  if (!v)
    return
  if (selected.value.some(s => s.name === v)) {
    toast.info(t('phone.tag.exists'))
    input.value = ''
    return
  }
  // 若已有标签库中已存在同名，沿用其颜色，避免同名标签颜色不一致。
  const existing = allTags.value.find(tg => tg.name === v)
  selected.value.push({ name: v, color: existing?.color ?? color })
  input.value = ''
}
function removeTag(name: string) {
  selected.value = selected.value.filter(s => s.name !== name)
}

async function submit() {
  saving.value = true
  try {
    await phoneApi.setTags(props.ids, selected.value)
    toast.success(t('phone.tag.saveOk'))
    emit('success')
    open.value = false
  }
  catch { /* 拦截器已提示 */ }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ t('phone.tag.title') }}</DialogTitle>
        <DialogDescription>{{ t('phone.tag.desc', { n: ids.length }) }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-3">
        <!-- 已选标签 -->
        <div class="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border p-2">
          <Badge v-for="tg in selected" :key="tg.name" variant="outline" class="gap-1" :class="tagClass(tg.color)">
            {{ tg.name }}
            <button type="button" class="hover:opacity-70" @click="removeTag(tg.name)">
              <X class="size-3" />
            </button>
          </Badge>
          <span v-if="!selected.length" class="text-xs text-muted-foreground">{{ t('phone.tag.emptySelected') }}</span>
        </div>

        <!-- 颜色选择 -->
        <div class="space-y-1.5">
          <div class="text-xs text-muted-foreground">
            {{ t('phone.tag.color') }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="c in TAG_COLORS"
              :key="c"
              type="button"
              class="size-6 rounded-full ring-offset-2 ring-offset-background transition"
              :class="[tagDot(c), currentColor === c ? 'ring-2 ring-ring' : '']"
              @click="currentColor = c"
            />
          </div>
        </div>

        <!-- 输入新标签 -->
        <div class="flex gap-2">
          <Input v-model="input" :placeholder="t('phone.tag.placeholder')" @keyup.enter="addTag(input, currentColor)" />
          <Button type="button" variant="outline" :disabled="!input.trim()" @click="addTag(input, currentColor)">
            <Plus class="size-4" /> {{ t('phone.tag.add') }}
          </Button>
        </div>

        <!-- 已有标签建议 -->
        <div v-if="suggestions.length" class="space-y-1.5">
          <div class="text-xs text-muted-foreground">
            {{ t('phone.tag.existing') }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button v-for="tg in suggestions" :key="tg.name" type="button" @click="addTag(tg.name, tg.color)">
              <Badge variant="outline" class="cursor-pointer" :class="tagClass(tg.color)">
                {{ tg.name }}
              </Badge>
            </button>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button :disabled="saving" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
