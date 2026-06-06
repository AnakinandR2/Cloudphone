<script setup lang="ts">
import { Copy, KeyRound, Plug, Plus } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const { t } = useI18n()

// 原型静态数据。
const keys = [
  { id: 1, name: '默认密钥', key: 'gp_live_••••••3f2a', created: '2026-05-12 09:20', lastUsed: '2026-06-05 14:02' },
  { id: 2, name: 'CI 自动化', key: 'gp_live_••••••9b07', created: '2026-04-28 16:41', lastUsed: '—' },
]
const mcpUrl = 'https://mcp.gloryphone.com/sse'
const mcpToken = 'gp_mcp_••••••a17c'
const configJson = `{
  "mcpServers": {
    "gloryphone": {
      "url": "${mcpUrl}",
      "headers": { "Authorization": "Bearer gp_mcp_xxxxxxxx" }
    }
  }
}`
const tools = ['listPhones', 'powerOnOff', 'installApp', 'runScript', 'screenshot', 'sendKeys']

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('apimcp.copied'))
  }
  catch {
    toast.error(t('apimcp.copyFail'))
  }
}
function soon() {
  toast.info(t('apimcp.comingSoon'))
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
        <Button size="sm" @click="soon">
          <Plus class="size-4" /> {{ t('apimcp.newKey') }}
        </Button>
      </CardHeader>
      <CardContent>
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
              <TableRow v-for="k in keys" :key="k.id">
                <TableCell class="font-medium">{{ k.name }}</TableCell>
                <TableCell>
                  <span class="inline-flex items-center gap-1.5">
                    <code class="font-mono text-xs">{{ k.key }}</code>
                    <Button variant="ghost" size="icon" class="size-6" @click="copy(k.key)">
                      <Copy class="size-3.5" />
                    </Button>
                  </span>
                </TableCell>
                <TableCell class="text-muted-foreground tabular-nums">{{ k.created }}</TableCell>
                <TableCell class="text-muted-foreground tabular-nums">{{ k.lastUsed }}</TableCell>
                <TableCell><Badge>{{ t('apimcp.statusActive') }}</Badge></TableCell>
                <TableCell class="text-right">
                  <Button variant="ghost" size="sm" class="text-destructive" @click="soon">{{ t('apimcp.revoke') }}</Button>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
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
      <CardContent class="flex flex-col gap-5">
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-1.5">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.mcpUrl') }}</span>
            <div class="flex items-center gap-2">
              <Input :model-value="mcpUrl" readonly class="h-9 font-mono text-xs" />
              <Button variant="outline" size="icon" class="size-9 shrink-0" @click="copy(mcpUrl)">
                <Copy class="size-4" />
              </Button>
            </div>
          </div>
          <div class="flex flex-col gap-1.5">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.mcpToken') }}</span>
            <div class="flex items-center gap-2">
              <Input :model-value="mcpToken" readonly class="h-9 font-mono text-xs" />
              <Button variant="outline" size="icon" class="size-9 shrink-0" @click="copy(mcpToken)">
                <Copy class="size-4" />
              </Button>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-1.5">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground text-sm">{{ t('apimcp.mcpConfig') }}</span>
            <Button variant="ghost" size="sm" @click="copy(configJson)">
              <Copy class="size-3.5" /> {{ t('apimcp.copy') }}
            </Button>
          </div>
          <pre class="bg-muted/50 overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed">{{ configJson }}</pre>
        </div>

        <div class="flex flex-col gap-2">
          <span class="text-muted-foreground text-sm">{{ t('apimcp.capsTitle') }}</span>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="tool in tools" :key="tool" variant="secondary" class="font-normal">
              <code class="mr-1 font-mono text-[11px]">{{ tool }}</code>{{ t(`apimcp.tool_${tool}`) }}
            </Badge>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
