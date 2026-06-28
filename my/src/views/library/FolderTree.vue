<script setup lang="ts">
import type { LibraryFolder } from '@/types/library'
import { HardDrive } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import FolderTreeNode from './FolderTreeNode.vue'

// 文件夹树。扁平 folders 由父级传入，递归子组件渲染。
const props = defineProps<{
  folders: LibraryFolder[]
  selected: number // 当前选中文件夹 id（0=根）
}>()
const emit = defineEmits<{
  'update:selected': [id: number]
  'rename': [id: number, name: string]
  'move': [id: number, parentId: number]
  'delete': [id: number]
}>()
const { t } = useI18n()

const roots = computed(() =>
  props.folders
    .filter(f => f.parent_id === 0)
    .sort((a, b) => a.name.localeCompare(b.name)))

const expandedIds = ref<number[]>([])
function toggle(id: number) {
  const i = expandedIds.value.indexOf(id)
  if (i >= 0) expandedIds.value.splice(i, 1)
  else expandedIds.value.push(id)
}
</script>

<template>
  <div class="flex flex-col gap-0.5 text-sm">
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-md px-2 py-1.5 text-left transition-colors"
      :class="selected === 0 ? 'bg-primary/10 text-primary font-medium' : 'hover:bg-muted/60'"
      @click="emit('update:selected', 0)"
    >
      <HardDrive class="size-4 shrink-0" />
      <span class="truncate">{{ t('library.folder.root') }}</span>
    </button>

    <FolderTreeNode
      v-for="node in roots"
      :key="node.id"
      :folder="node"
      :all-folders="folders"
      :depth="0"
      :selected="selected"
      :expanded-ids="expandedIds"
      @select="emit('update:selected', $event)"
      @toggle="toggle"
      @rename="(id, name) => emit('rename', id, name)"
      @move="(id, parentId) => emit('move', id, parentId)"
      @delete="emit('delete', $event)"
    />
  </div>
</template>
