<script setup lang="ts">
import type { LibraryFile, LibraryFolder } from '@/types/library'
import { Folder, FolderInput, MoreHorizontal, Pencil } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

// 文件「更多」操作：重命名（就地输入）/ 移动到（文件夹选择）。删除沿用外部的 Popconfirm 按钮。
const props = defineProps<{
  file: LibraryFile
  folders: LibraryFolder[]
}>()
const emit = defineEmits<{
  'rename': [id: number, name: string]
  'move': [id: number, folderId: number]
}>()
const { t } = useI18n()

const open = ref(false)
const mode = ref<'menu' | 'rename' | 'move'>('menu')
const nameInput = ref('')

function start() {
  mode.value = 'menu'
  nameInput.value = props.file.name
  open.value = true
}

// 移动目标：根目录 + 全部文件夹（按全路径名展示），排除文件当前所在夹。
const moveTargets = computed(() => {
  const byId = new Map(props.folders.map(f => [f.id, f]))
  function pathOf(f: LibraryFolder): string {
    const parts = [f.name]
    let p = f.parent_id
    while (p) {
      const pf = byId.get(p)
      if (!pf) break
      parts.unshift(pf.name)
      p = pf.parent_id
    }
    return parts.join(' / ')
  }
  return props.folders
    .filter(f => f.id !== props.file.folder_id)
    .map(f => ({ id: f.id, label: pathOf(f) }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

function doRename() {
  const name = nameInput.value.trim()
  if (!name) return
  emit('rename', props.file.id, name)
  open.value = false
}
function doMove(folderId: number) {
  emit('move', props.file.id, folderId)
  open.value = false
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button size="icon" variant="ghost" class="size-7" :title="t('crud.more')" @click="start">
        <MoreHorizontal class="size-3.5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" class="w-56 p-1">
      <!-- 菜单 -->
      <div v-if="mode === 'menu'" class="flex flex-col">
        <button type="button" class="hover:bg-muted flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="mode = 'rename'">
          <Pencil class="size-3.5" /> {{ t('library.rename') }}
        </button>
        <button type="button" class="hover:bg-muted flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="mode = 'move'">
          <FolderInput class="size-3.5" /> {{ t('library.moveTo') }}
        </button>
      </div>

      <!-- 重命名 -->
      <form v-else-if="mode === 'rename'" class="flex flex-col gap-2 p-1" @submit.prevent="doRename">
        <Input v-model="nameInput" autofocus class="h-8" />
        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" size="sm" class="h-7 px-2.5" @click="mode = 'menu'">
            {{ t('crud.cancel') }}
          </Button>
          <Button type="submit" size="sm" class="h-7 px-2.5" :disabled="!nameInput.trim()">
            {{ t('crud.confirm') }}
          </Button>
        </div>
      </form>

      <!-- 移动到 -->
      <div v-else class="flex flex-col gap-1 p-1">
        <div class="text-muted-foreground px-1 pb-1 text-xs">
          {{ t('library.moveToHint') }}
        </div>
        <div class="max-h-56 overflow-y-auto">
          <button type="button" class="hover:bg-muted flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="doMove(0)">
            <Folder class="size-3.5 shrink-0" /> {{ t('library.folder.root') }}
          </button>
          <button
            v-for="tg in moveTargets"
            :key="tg.id"
            type="button"
            class="hover:bg-muted flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm"
            @click="doMove(tg.id)"
          >
            <Folder class="size-3.5 shrink-0" /> <span class="truncate">{{ tg.label }}</span>
          </button>
        </div>
        <Button type="button" variant="outline" size="sm" class="mt-1 h-7 px-2.5" @click="mode = 'menu'">
          {{ t('crud.cancel') }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
