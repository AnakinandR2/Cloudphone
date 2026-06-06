<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { Code2, Download, FileCode, Pencil, Play, Plus, Star, Trash2 } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

interface Script { id: number, name: string, lang: string, desc: string, updated: string, status: string }
const { t } = useI18n()

const search = ref('')
const myScripts: Script[] = [
  { id: 1, name: '每日签到', lang: 'JavaScript', desc: '自动完成 App 每日签到任务', updated: '2026-06-04 10:22', status: 'enabled' },
  { id: 2, name: '养号脚本', lang: 'Python', desc: '模拟真人操作维持账号活跃', updated: '2026-06-02 18:40', status: 'enabled' },
  { id: 3, name: '批量点赞', lang: 'JavaScript', desc: '对指定视频批量点赞', updated: '2026-05-28 09:05', status: 'disabled' },
  { id: 4, name: '关键词采集', lang: 'Python', desc: '按关键词采集商品信息并导出', updated: '2026-05-20 14:12', status: 'enabled' },
]
const columns: ColumnDef<Script>[] = [
  { accessorKey: 'name', id: 'name', header: t('script.colName'), meta: { label: 'script.colName' } },
  { accessorKey: 'lang', id: 'lang', header: t('script.colLang'), meta: { label: 'script.colLang' } },
  { accessorKey: 'desc', id: 'desc', header: t('script.colDesc'), meta: { label: 'script.colDesc' } },
  { accessorKey: 'updated', id: 'updated', header: t('script.colUpdated'), meta: { label: 'script.colUpdated' } },
  { accessorKey: 'status', id: 'status', header: t('script.colStatus'), meta: { label: 'script.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
]

const categories = ['all', 'social', 'ecommerce', 'game', 'tool']
const activeCat = ref('all')
const storeScripts = [
  { id: 101, name: '抖音养号大师', author: '官方', cat: 'social', desc: '多账号自动浏览、点赞、关注，智能模拟真人行为', installs: '12.3k', rating: 4.9, free: true },
  { id: 102, name: '电商自动下单', author: '云象工作室', cat: 'ecommerce', desc: '秒杀抢购、自动下单付款，支持多平台', installs: '8.7k', rating: 4.6, free: false },
  { id: 103, name: '游戏日常托管', author: '官方', cat: 'game', desc: '主流手游每日任务一键托管', installs: '21.1k', rating: 4.8, free: true },
  { id: 104, name: '批量改机助手', author: 'DevTools', cat: 'tool', desc: '一键随机设备信息，配合养号使用', installs: '5.2k', rating: 4.4, free: true },
  { id: 105, name: '私信群发', author: '增长黑客', cat: 'social', desc: '按人群批量私信，内置话术模板', installs: '3.9k', rating: 4.2, free: false },
  { id: 106, name: '评论区监控', author: '云象工作室', cat: 'tool', desc: '关键词监控评论并自动回复', installs: '2.1k', rating: 4.5, free: true },
]
const filteredStore = computed(() =>
  activeCat.value === 'all' ? storeScripts : storeScripts.filter(s => s.cat === activeCat.value),
)

function soon() {
  toast.info(t('script.comingSoon'))
}
</script>

<template>
  <Card>
    <CardHeader>
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1.5">
          <CardTitle>{{ t('script.title') }}</CardTitle>
          <CardDescription>{{ t('script.desc') }}</CardDescription>
        </div>
        <Button size="sm" class="shrink-0" @click="soon">
          <Plus class="size-4" /> {{ t('script.newScript') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <Tabs default-value="mine">
        <TabsList>
          <TabsTrigger value="mine">
            {{ t('script.tabMine') }}
          </TabsTrigger>
          <TabsTrigger value="store">
            {{ t('script.tabStore') }}
          </TabsTrigger>
        </TabsList>

        <!-- 我的脚本 -->
        <TabsContent value="mine" class="mt-4">
          <DataTable
            v-model:search-value="search"
            :columns="columns"
            :data="myScripts"
            :get-row-id="(r) => String(r.id)"
            :search-placeholder="t('script.searchMine')"
          >
            <template #cell-name="{ row }">
              <span class="flex items-center gap-2 font-medium"><FileCode class="size-4 text-muted-foreground" />{{ row.name }}</span>
            </template>
            <template #cell-lang="{ row }">
              <Badge variant="secondary" class="font-normal">
                <Code2 class="mr-1 size-3" />{{ row.lang }}
              </Badge>
            </template>
            <template #cell-desc="{ row }">
              <span class="text-muted-foreground">{{ row.desc }}</span>
            </template>
            <template #cell-updated="{ row }">
              <span class="tabular-nums text-muted-foreground">{{ row.updated }}</span>
            </template>
            <template #cell-status="{ row }">
              <Badge :variant="row.status === 'enabled' ? 'default' : 'secondary'">
                {{ row.status === 'enabled' ? t('script.enabled') : t('script.disabled') }}
              </Badge>
            </template>
            <template #cell-actions>
              <Button variant="ghost" size="icon" class="size-7" :title="t('script.run')" @click="soon">
                <Play class="size-3.5" />
              </Button>
              <Button variant="ghost" size="icon" class="size-7" :title="t('crud.edit')" @click="soon">
                <Pencil class="size-3.5" />
              </Button>
              <Button variant="ghost" size="icon" class="size-7 text-destructive" :title="t('crud.delete')" @click="soon">
                <Trash2 class="size-3.5" />
              </Button>
            </template>
          </DataTable>
        </TabsContent>

        <!-- 脚本商店 -->
        <TabsContent value="store" class="mt-4">
          <div class="mb-4 flex flex-wrap items-center gap-1.5">
            <Button
              v-for="c in categories" :key="c"
              :variant="activeCat === c ? 'default' : 'outline'"
              size="sm" class="h-7"
              @click="activeCat = c"
            >
              {{ t(`script.cat_${c}`) }}
            </Button>
          </div>
          <div class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
            <div v-for="s in filteredStore" :key="s.id" class="flex flex-col gap-3 rounded-lg border p-4">
              <div class="flex items-start gap-3">
                <div class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <FileCode class="size-5" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate font-medium">{{ s.name }}</span>
                    <Badge v-if="s.author === '官方'" class="h-4 px-1 text-[10px]">
                      {{ t('script.official') }}
                    </Badge>
                  </div>
                  <div class="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{{ s.author }}</span>
                    <span class="flex items-center gap-0.5"><Star class="size-3 fill-amber-400 text-amber-400" />{{ s.rating }}</span>
                  </div>
                </div>
              </div>
              <p class="line-clamp-2 text-xs text-muted-foreground">
                {{ s.desc }}
              </p>
              <div class="mt-auto flex items-center justify-between">
                <span class="text-xs text-muted-foreground">{{ t('script.installs', { n: s.installs }) }}</span>
                <Button size="sm" class="h-7" :variant="s.free ? 'default' : 'outline'" @click="soon">
                  <Download class="size-3.5" /> {{ s.free ? t('script.get') : t('script.buy') }}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </CardContent>
  </Card>
</template>
