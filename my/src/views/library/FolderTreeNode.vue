<script setup lang="ts">
import type { LibraryFolder } from '@/types/library'
import { ChevronRight, Folder, FolderInput, FolderOpen, MoreHorizontal, Pencil, Trash2 } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

// 递归文件夹节点。自引用渲染子树。
const props = defineProps<{
  folder: LibraryFolder
  allFolders: LibraryFolder[]
  depth: number
  selected: number
  expandedIds: number[]
}>()
const emit = defineEmits<{
  'select': [id: number]
  'toggle': [id: number]
  'rename': [id: number, name: string]
  'move': [id: number, parentId: number]
  'delete': [id: number]
}>()
const { t } = useI18n()

const children = computed(() =>
  props.allFolders
    .filter(f => f.parent_id === props.folder.id)
    .sort((a, b) => a.name.localeCompare(b.name)))

const isOpen = computed(() => props.expandedIds.includes(props.folder.id))
const isSelected = computed(() => props.selected === props.folder.id)
const hasChildren = computed(() => children.value.length > 0)

// ---- 操作 popover（就地）----
const menuOpen = ref(false)
const mode = ref<'menu' | 'rename' | 'move' | 'delete'>('menu')
const renameInput = ref('')

function openMenu() {
  mode.value = 'menu'
  renameInput.value = props.folder.name
  menuOpen.value = true
}

// 自身 + 全部子孙 id（移动目标需排除，防环）。
const blockedIds = computed(() => {
  const childMap = new Map<number, number[]>()
  for (const f of props.allFolders) {
    const arr = childMap.get(f.parent_id) ?? []
    arr.push(f.id)
    childMap.set(f.parent_id, arr)
  }
  const set = new Set<number>([props.folder.id])
  const stack = [props.folder.id]
  while (stack.length) {
    const cur = stack.pop()!
    for (const cid of childMap.get(cur) ?? []) {
      if (!set.has(cid)) {
        set.add(cid)
        stack.push(cid)
      }
    }
  }
  return set
})

// 移动目标：排除自身+子孙+当前父级；按全路径名展示便于区分。
const moveTargets = computed(() => {
  const byId = new Map(props.allFolders.map(f => [f.id, f]))
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
  return props.allFolders
    .filter(f => !blockedIds.value.has(f.id) && f.id !== props.folder.parent_id)
    .map(f => ({ id: f.id, label: pathOf(f) }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

function doRename() {
  const name = renameInput.value.trim()
  if (!name) return
  emit('rename', props.folder.id, name)
  menuOpen.value = false
}
function doMove(parentId: number) {
  emit('move', props.folder.id, parentId)
  menuOpen.value = false
}
function doDelete() {
  emit('delete', props.folder.id)
  menuOpen.value = false
}
</script>

<template>
  <div>
    <div
      class="group flex cursor-pointer items-center gap-1 rounded-md py-1.5 pr-1 transition-colors"
      :class="isSelected ? 'bg-primary/10 text-primary font-medium' : 'hover:bg-muted/60'"
      :style="{ paddingLeft: `${8 + depth * 14}px` }"
      @click="emit('select', folder.id)"
    >
      <ChevronRight
        v-if="hasChildren"
        class="size-3.5 shrink-0 transition-transform"
        :class="isOpen ? 'rotate-90' : ''"
        @click.stop="emit('toggle', folder.id)"
      />
      <span v-else class="size-3.5 shrink-0" />
      <FolderOpen v-if="isOpen && hasChildren" class="size-4 shrink-0" />
      <Folder v-else class="size-4 shrink-0" />
      <span class="flex-1 truncate text-sm">{{ folder.name }}</span>

      <Popover v-model:open="menuOpen">
        <PopoverTrigger as-child>
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground shrink-0 rounded p-0.5 opacity-0 transition-opacity group-hover:opacity-100 data-[state=open]:opacity-100"
            :title="t('crud.more')"
            @click.stop="openMenu"
          >
            <MoreHorizontal class="size-3.5" />
          </button>
        </PopoverTrigger>
        <PopoverContent align="end" class="w-56 p-1">
          <!-- 菜单 -->
          <div v-if="mode === 'menu'" class="flex flex-col">
            <button type="button" class="hover:bg-muted flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="mode = 'rename'">
              <Pencil class="size-3.5" /> {{ t('library.folder.rename') }}
            </button>
            <button type="button" class="hover:bg-muted flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="mode = 'move'">
              <FolderInput class="size-3.5" /> {{ t('library.folder.moveTo') }}
            </button>
            <button type="button" class="hover:bg-destructive/10 text-destructive flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="mode = 'delete'">
              <Trash2 class="size-3.5" /> {{ t('crud.delete') }}
            </button>
          </div>

          <!-- 重命名 -->
          <form v-else-if="mode === 'rename'" class="flex flex-col gap-2 p-1" @submit.prevent="doRename">
            <Input v-model="renameInput" autofocus class="h-8" />
            <div class="flex justify-end gap-2">
              <Button type="button" variant="outline" size="sm" class="h-7 px-2.5" @click="mode = 'menu'">
                {{ t('crud.cancel') }}
              </Button>
              <Button type="submit" size="sm" class="h-7 px-2.5" :disabled="!renameInput.trim()">
                {{ t('crud.confirm') }}
              </Button>
            </div>
          </form>

          <!-- 移动到 -->
          <div v-else-if="mode === 'move'" class="flex flex-col gap-1 p-1">
            <div class="text-muted-foreground px-1 pb-1 text-xs">
              {{ t('library.folder.moveToHint') }}
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

          <!-- 删除确认 -->
          <div v-else class="flex flex-col gap-2 p-1">
            <p class="text-sm">
              {{ t('library.folder.deleteConfirm', { name: folder.name }) }}
            </p>
            <div class="flex justify-end gap-2">
              <Button type="button" variant="outline" size="sm" class="h-7 px-2.5" @click="mode = 'menu'">
                {{ t('crud.cancel') }}
              </Button>
              <Button type="button" variant="destructive" size="sm" class="h-7 px-2.5" @click="doDelete">
                {{ t('crud.delete') }}
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>

    <template v-if="isOpen && hasChildren">
      <FolderTreeNode
        v-for="child in children"
        :key="child.id"
        :folder="child"
        :all-folders="allFolders"
        :depth="depth + 1"
        :selected="selected"
        :expanded-ids="expandedIds"
        @select="emit('select', $event)"
        @toggle="emit('toggle', $event)"
        @rename="(id, name) => emit('rename', id, name)"
        @move="(id, parentId) => emit('move', id, parentId)"
        @delete="emit('delete', $event)"
      />
    </template>
  </div>
</template>
