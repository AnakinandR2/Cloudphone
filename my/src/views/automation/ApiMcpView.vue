<script setup lang="ts">
import type { ApiKey } from '@/types/apikey'
import { Copy, Eye, KeyRound, Plug, Plus, TriangleAlert } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import apikeyApi from '@/api/modules/apikey'
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
import { formatDateTime } from '@/utils/date'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const { t } = useI18n()

const keys = ref<ApiKey[]>([])
const loading = ref(false)
const createOpen = ref(false)
const newName = ref('')
const creating = ref(false)
const fullKeyOpen = ref(false)
const fullKey = ref('')

const baseUrl = computed(() => `${window.location.origin}/api/open/v1`)
const curlExample = computed(() =>
  `curl ${baseUrl.value}/phones \\\n  -H "Authorization: Bearer gp_live_xxxxxxxx"`,
)

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
onMounted(load)

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
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('apimcp.copied'))
  }
  catch {
    toast.error(t('apimcp.copyFail'))
  }
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
        <div class="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{{ t('apimcp.colName') }}</TableHead>
                <TableHead>{{ t('apimcp.colKey') }}</TableHead>
                <TableHead>{{ t('apimcp.colCreated') }}</TableHead>
                <TableHead>{{ t('apimcp.colLastUsed') }}</TableHead>
                <TableHead>{{ t('apimcp.colStatus') }}</TableHead>
                <TableHead class="text-right">{{ t('apimcp.colAction') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-if="!keys.length">
                <TableCell colspan="6" class="text-muted-foreground py-8 text-center text-sm">
                  {{ t('apimcp.empty') }}
                </TableCell>
              </TableRow>
              <TableRow v-for="k in keys" :key="k.id" :class="k.status === 'revoked' && 'opacity-50'">
                <TableCell class="font-medium">{{ k.name }}</TableCell>
                <TableCell>
                  <span class="inline-flex items-center gap-1.5">
                    <code class="font-mono text-xs">{{ k.masked }}</code>
                    <Button variant="ghost" size="icon" class="size-6" :title="t('apimcp.reveal')" @click="revealKey(k)">
                      <Eye class="size-3.5" />
                    </Button>
                  </span>
                </TableCell>
                <TableCell class="text-muted-foreground tabular-nums">{{ formatDateTime(k.createdAt) }}</TableCell>
                <TableCell class="text-muted-foreground tabular-nums">{{ k.lastUsedAt ? formatDateTime(k.lastUsedAt) : '—' }}</TableCell>
                <TableCell>
                  <Badge :variant="k.status === 'active' ? 'default' : 'secondary'">
                    {{ k.status === 'active' ? t('apimcp.statusActive') : t('apimcp.statusRevoked') }}
                  </Badge>
                </TableCell>
                <TableCell class="text-right">
                  <Popconfirm v-if="k.status === 'active'" :title="t('apimcp.revokeConfirm', { name: k.name })" tone="danger" @confirm="revoke(k)">
                    <Button variant="ghost" size="sm" class="text-destructive hover:text-destructive">{{ t('apimcp.revoke') }}</Button>
                  </Popconfirm>
                  <span v-else class="text-muted-foreground text-xs">{{ t('apimcp.revoked') }}</span>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- 如何调用 -->
        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.howTo') }}</span>
            <Button variant="ghost" size="sm" @click="copy(curlExample)">
              <Copy class="size-3.5" /> {{ t('apimcp.copy') }}
            </Button>
          </div>
          <pre class="bg-muted/50 overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed">{{ curlExample }}</pre>
          <p class="text-muted-foreground text-xs">{{ t('apimcp.baseUrl') }}：<code class="font-mono">{{ baseUrl }}</code></p>
          <p class="text-muted-foreground text-xs">{{ t('apimcp.howToHint') }}</p>
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
