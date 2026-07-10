<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { ApiKey } from '@/types/apikey'
import type { McpToolGroup } from '@/types/mcp'
import { ArrowUpRight, Copy, Eye, KeyRound, Plug, Plus, TriangleAlert } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import apikeyApi from '@/api/modules/apikey'
import mcpApi from '@/api/modules/mcp'
import DataTable from '@/components/DataTable.vue'
import { copyText } from '@/utils/clipboard'
import Popconfirm from '@/components/Popconfirm.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { formatDateTime } from '@/utils/date'
import { activeStatusBadge } from '@/utils/statusBadge'

const { t } = useI18n()

const keys = ref<ApiKey[]>([])
const loading = ref(false)
const createOpen = ref(false)
const newName = ref('')
const creating = ref(false)
const fullKeyOpen = ref(false)
const fullKey = ref('')

const columns = computed<ColumnDef<ApiKey>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('apimcp.colName'), meta: { label: 'apimcp.colName' } },
  { id: 'key', header: t('apimcp.colKey'), meta: { label: 'apimcp.colKey' } },
  { accessorKey: 'createdAt', id: 'createdAt', header: t('apimcp.colCreated'), meta: { label: 'apimcp.colCreated' } },
  { accessorKey: 'lastUsedAt', id: 'lastUsedAt', header: t('apimcp.colLastUsed'), meta: { label: 'apimcp.colLastUsed' } },
  { accessorKey: 'status', id: 'status', header: t('apimcp.colStatus'), meta: { label: 'apimcp.colStatus' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

const baseUrl = computed(() => `${window.location.origin}/api/open/v1`)
const curlExample = computed(() =>
  `curl ${baseUrl.value}/phones \\\n  -H "Authorization: Bearer gp_live_xxxxxxxx"`,
)

// www 完整 API 文档（Scalar）地址：同源默认 /api-docs，跨域部署经 VITE_DOCS_URL 填完整地址。
const docsUrl = import.meta.env.VITE_DOCS_URL || '/api-docs'

// MCP 工具清单（按领域分组，仅工具 code），从后端 /mcp/tools 动态拉取。
const toolGroups = ref<McpToolGroup[]>([])
const toolsError = ref(false)
const toolCount = computed(() => toolGroups.value.reduce((n, g) => n + g.tools.length, 0))

const mcpUrl = computed(() => `${window.location.origin}/api/mcp`)
const mcpConfig = computed(() =>
  JSON.stringify(
    {
      mcpServers: {
        gloryphone: {
          type: 'streamable-http',
          url: mcpUrl.value,
          headers: { Authorization: 'Bearer gp_live_xxxxxxxx' },
        },
      },
    },
    null,
    2,
  ),
)

async function load() {
  loading.value = true
  try {
    const { data } = await apikeyApi.list()
    keys.value = data ?? []
  }
  finally {
    loading.value = false
  }
}

// 失败不阻塞页面其余功能：清单区不渲染，仅留一行降级提示（保留「与开放 API 一致」这条信息）。
async function loadTools() {
  toolsError.value = false
  try {
    const { data } = await mcpApi.getTools()
    toolGroups.value = data ?? []
  }
  catch {
    toolGroups.value = []
    toolsError.value = true
  }
}

onMounted(() => {
  load()
  loadTools()
})

function openCreate() {
  newName.value = ''
  createOpen.value = true
}
async function doCreate() {
  if (!newName.value.trim()) {
    toast.error(t('apimcp.errName'))
    return
  }
  creating.value = true
  try {
    const { data } = await apikeyApi.create(newName.value.trim())
    createOpen.value = false
    fullKey.value = data.fullKey
    fullKeyOpen.value = true
    load()
  }
  finally {
    creating.value = false
  }
}
async function revealKey(k: ApiKey) {
  try {
    const { data } = await apikeyApi.reveal(k.id)
    fullKey.value = data.fullKey
    fullKeyOpen.value = true
  }
  catch (e: any) {
    toast.error(e?.message ?? t('apimcp.revealFail'))
  }
}
async function revoke(k: ApiKey) {
  await apikeyApi.revoke(k.id)
  toast.success(t('apimcp.revokeOk'))
  load()
}

async function copy(text: string) {
  if (!text)
    return
  // 用兼容非 HTTPS 的 copyText（CP-0046 / #41）：内网 http 下 navigator.clipboard 不可用会回退 execCommand。
  if (await copyText(text))
    toast.success(t('apimcp.copied'))
  else
    toast.error(t('apimcp.copyFail'))
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('apimcp.title') }}</h1>
      <p class="text-muted-foreground mt-1 text-sm">{{ t('apimcp.desc') }}</p>
    </div>

    <!-- API 密钥 -->
    <Card>
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle class="flex items-center gap-2 text-base">
            <KeyRound class="size-4" /> {{ t('apimcp.keysTitle') }}
          </CardTitle>
          <CardDescription>{{ t('apimcp.keysDesc') }}</CardDescription>
        </div>
        <Button size="sm" @click="openCreate">
          <Plus class="size-4" /> {{ t('apimcp.newKey') }}
        </Button>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <DataTable
          pin-actions-column
          :columns="columns"
          :data="keys"
          :loading="loading"
          :search="false"
          :get-row-id="(k) => String(k.id)"
        >
          <template #cell-name="{ row }">
            <span class="font-medium" :class="row.status === 'revoked' && 'opacity-50'">{{ row.name }}</span>
          </template>
          <template #cell-key="{ row }">
            <span class="inline-flex items-center gap-1.5" :class="row.status === 'revoked' && 'opacity-50'">
              <code class="font-mono text-xs">{{ row.masked }}</code>
              <Button variant="ghost" size="icon" class="size-6" :title="t('apimcp.reveal')" @click="revealKey(row)">
                <Eye class="size-3.5" />
              </Button>
            </span>
          </template>
          <template #cell-createdAt="{ row }">
            <span class="text-muted-foreground tabular-nums" :class="row.status === 'revoked' && 'opacity-50'">{{ formatDateTime(row.createdAt) }}</span>
          </template>
          <template #cell-lastUsedAt="{ row }">
            <span class="text-muted-foreground tabular-nums" :class="row.status === 'revoked' && 'opacity-50'">{{ row.lastUsedAt ? formatDateTime(row.lastUsedAt) : '—' }}</span>
          </template>
          <template #cell-status="{ row }">
            <Badge v-bind="activeStatusBadge(row.status === 'active')" :class="row.status === 'revoked' && 'opacity-50'">
              {{ row.status === 'active' ? t('apimcp.statusActive') : t('apimcp.statusRevoked') }}
            </Badge>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-2">
              <Popconfirm v-if="row.status === 'active'" :title="t('apimcp.revokeConfirm', { name: row.name })" tone="danger" @confirm="revoke(row)">
                <Button variant="ghost" size="sm" class="text-destructive hover:text-destructive">{{ t('apimcp.revoke') }}</Button>
              </Popconfirm>
              <span v-else class="text-muted-foreground text-xs">{{ t('apimcp.revoked') }}</span>
            </div>
          </template>
        </DataTable>

        <!-- 如何调用 -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.howTo') }}</span>
            <Button variant="ghost" size="sm" @click="copy(curlExample)">
              <Copy class="size-3.5" /> {{ t('apimcp.copy') }}
            </Button>
          </div>
          <pre class="bg-muted/50 overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed">{{ curlExample }}</pre>
          <a
            :href="docsUrl" target="_blank" rel="noopener noreferrer"
            class="text-primary inline-flex w-fit items-center gap-1 text-xs hover:underline"
          >
            {{ t('apimcp.apiDocs') }} <ArrowUpRight class="size-3.5" />
          </a>
        </div>
      </CardContent>
    </Card>

    <!-- MCP 服务 -->
    <Card>
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-base">
          <Plug class="size-4" /> {{ t('apimcp.mcpTitle') }}
        </CardTitle>
        <CardDescription>{{ t('apimcp.mcpDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-3">
        <p class="text-muted-foreground text-xs">
          {{ t('apimcp.mcpEndpoint') }}：<code class="font-mono">{{ mcpUrl }}</code>
        </p>
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.mcpConfigTitle') }}</span>
            <Button variant="ghost" size="sm" @click="copy(mcpConfig)">
              <Copy class="size-3.5" /> {{ t('apimcp.copy') }}
            </Button>
          </div>
          <pre class="bg-muted/50 overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed">{{ mcpConfig }}</pre>
        </div>
        <p class="text-muted-foreground text-xs">{{ t('apimcp.mcpHint') }}</p>

        <!-- 可用工具：按领域分组，从后端动态拉取（仅工具 code，详情见 API 文档） -->
        <div v-if="toolGroups.length" class="flex flex-col gap-2">
          <span class="text-muted-foreground text-sm">{{ t('apimcp.toolsTitle', { count: toolCount }) }}</span>
          <div class="flex flex-col gap-2.5">
            <div v-for="g in toolGroups" :key="g.group" class="flex flex-col gap-1.5">
              <span class="text-muted-foreground text-xs font-medium">{{ t(`apimcp.group.${g.group}`) }}</span>
              <div class="flex flex-wrap gap-1.5">
                <code
                  v-for="name in g.tools" :key="name"
                  class="bg-muted rounded px-1.5 py-0.5 font-mono text-xs"
                >{{ name }}</code>
              </div>
            </div>
          </div>
        </div>
        <p v-else-if="toolsError" class="text-muted-foreground text-xs">{{ t('apimcp.toolsLoadFail') }}</p>
      </CardContent>
    </Card>

    <!-- 新建密钥 -->
    <Dialog v-model:open="createOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('apimcp.newKey') }}</DialogTitle>
          <DialogDescription>{{ t('apimcp.newKeyDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="grid gap-2 py-2">
          <Label>{{ t('apimcp.colName') }}</Label>
          <Input v-model="newName" :placeholder="t('apimcp.namePh')" @keyup.enter="doCreate" />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="createOpen = false">
            {{ t('apimcp.cancel') }}
          </Button>
          <Button :disabled="creating" @click="doCreate">
            {{ t('apimcp.create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 完整密钥一次性展示 -->
    <Dialog v-model:open="fullKeyOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ t('apimcp.fullKeyTitle') }}</DialogTitle>
          <DialogDescription>{{ t('apimcp.fullKeyDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3 py-2">
          <div class="flex items-center gap-2">
            <Input :model-value="fullKey" readonly class="h-9 font-mono text-xs" />
            <Button variant="outline" size="icon" class="size-9 shrink-0" @click="copy(fullKey)">
              <Copy class="size-4" />
            </Button>
          </div>
          <div class="text-destructive flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-xs">
            <TriangleAlert class="mt-0.5 size-4 shrink-0" />
            <span>{{ t('apimcp.fullKeyWarn') }}</span>
          </div>
        </div>
        <DialogFooter>
          <Button @click="fullKeyOpen = false">
            {{ t('apimcp.done') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
