<script setup lang="ts">
import {
  AlertTriangle,
  CheckCircle2,
  ChevronsUpDown,
  Copy,
  Info,
  MoreHorizontal,
  PanelRight,
  Pencil,
  RefreshCw,
  Trash2,
  XCircle,
} from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const { t } = useI18n()
const open = ref(false)
const loadingDemo = ref(true)

function reloadSkeleton() {
  loadingDemo.value = true
  setTimeout(() => (loadingDemo.value = false), 1800)
}
reloadSkeleton()

function promiseToast() {
  toast.promise(new Promise(resolve => setTimeout(resolve, 1500)), {
    loading: t('comp.toastLoading'),
    success: t('comp.toastSuccess'),
    error: t('comp.toastError'),
  })
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">
          {{ t('comp.feedbackTitle') }}
        </h1>
        <p class="text-muted-foreground text-sm">
          {{ t('comp.feedbackDesc') }}
        </p>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-2">
      <!-- Toast -->
      <Card class="transition-shadow duration-200 hover:shadow-md">
        <CardHeader>
          <CardTitle>{{ t('comp.toasts') }}</CardTitle>
          <CardDescription>{{ t('comp.toastsDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-wrap gap-2">
          <Button variant="outline" @click="toast.success(t('comp.toastSuccess'))">
            <CheckCircle2 class="text-emerald-500" />{{ t('comp.success') }}
          </Button>
          <Button variant="outline" @click="toast.info(t('comp.toastInfo'))">
            <Info class="text-sky-500" />{{ t('comp.info') }}
          </Button>
          <Button variant="outline" @click="toast.warning(t('comp.toastWarning'))">
            <AlertTriangle class="text-amber-500" />{{ t('comp.warning') }}
          </Button>
          <Button variant="outline" @click="toast.error(t('comp.toastErrorMsg'))">
            <XCircle class="text-rose-500" />{{ t('comp.error') }}
          </Button>
          <Button
            variant="outline"
            @click="toast(t('comp.toastAction'), {
              action: { label: t('common.confirm'), onClick: () => {} },
            })"
          >
            {{ t('comp.withAction') }}
          </Button>
          <Button variant="outline" @click="promiseToast">
            <RefreshCw />{{ t('comp.promise') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Tooltip + Dropdown -->
      <Card class="transition-shadow duration-200 hover:shadow-md">
        <CardHeader>
          <CardTitle>{{ t('comp.tooltipDropdown') }}</CardTitle>
          <CardDescription>{{ t('comp.tooltipDropdownDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <div class="flex flex-wrap items-center gap-2">
            <TooltipProvider :delay-duration="150">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="outline">
                    {{ t('comp.hoverMe') }}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{{ t('comp.tooltipText') }}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="outline" size="icon">
                    <Info />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right">
                  {{ t('comp.tooltipSide') }}
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <Separator />

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline">
                <MoreHorizontal />{{ t('comp.openMenu') }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" class="w-44">
              <DropdownMenuLabel>{{ t('comp.actions') }}</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem><Pencil class="size-4" />{{ t('common.edit') }}</DropdownMenuItem>
              <DropdownMenuItem><Copy class="size-4" />{{ t('comp.duplicate') }}</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem class="text-destructive focus:text-destructive">
                <Trash2 class="size-4" />{{ t('common.delete') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </CardContent>
      </Card>

      <!-- Sheet + Collapsible -->
      <Card class="transition-shadow duration-200 hover:shadow-md">
        <CardHeader>
          <CardTitle>{{ t('comp.sheetCollapsible') }}</CardTitle>
          <CardDescription>{{ t('comp.sheetCollapsibleDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <Sheet v-model:open="open">
            <SheetTrigger as-child>
              <Button variant="outline">
                <PanelRight />{{ t('comp.openSheet') }}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{{ t('comp.sheetTitle') }}</SheetTitle>
                <SheetDescription>{{ t('comp.sheetDesc') }}</SheetDescription>
              </SheetHeader>
              <div class="text-muted-foreground px-4 text-sm">
                {{ t('comp.sheetBody') }}
              </div>
              <SheetFooter>
                <Button @click="open = false">
                  {{ t('common.confirm') }}
                </Button>
                <SheetClose as-child>
                  <Button variant="outline">
                    {{ t('common.cancel') }}
                  </Button>
                </SheetClose>
              </SheetFooter>
            </SheetContent>
          </Sheet>

          <Separator />

          <Collapsible class="space-y-2">
            <CollapsibleTrigger as-child>
              <Button variant="ghost" class="w-full justify-between">
                {{ t('comp.collapsibleTitle') }}
                <ChevronsUpDown class="size-4" />
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent class="space-y-2">
              <div
                v-for="i in 3"
                :key="i"
                class="bg-muted/50 text-muted-foreground rounded-md px-3 py-2 text-sm"
              >
                {{ t('comp.collapsibleItem', { n: i }) }}
              </div>
            </CollapsibleContent>
          </Collapsible>
        </CardContent>
      </Card>

      <!-- Skeleton -->
      <Card class="transition-shadow duration-200 hover:shadow-md">
        <CardHeader>
          <CardTitle class="flex items-center justify-between">
            {{ t('comp.skeleton') }}
            <Button variant="ghost" size="icon-sm" @click="reloadSkeleton">
              <RefreshCw class="size-4" />
            </Button>
          </CardTitle>
          <CardDescription>{{ t('comp.skeletonDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div v-if="loadingDemo" class="space-y-4">
            <div class="flex items-center gap-3">
              <Skeleton class="size-12 rounded-full" />
              <div class="flex-1 space-y-2">
                <Skeleton class="h-4 w-1/3" />
                <Skeleton class="h-3 w-1/2" />
              </div>
            </div>
            <Skeleton class="h-24 w-full rounded-lg" />
            <div class="flex gap-2">
              <Skeleton class="h-9 w-24" />
              <Skeleton class="h-9 w-24" />
            </div>
          </div>
          <div v-else class="space-y-4">
            <div class="flex items-center gap-3">
              <div class="bg-primary/10 text-primary flex size-12 items-center justify-center rounded-full font-medium">
                GP
              </div>
              <div>
                <p class="font-medium">
                  Glory Phone
                </p>
                <p class="text-muted-foreground text-sm">
                  {{ t('comp.skeletonLoaded') }}
                </p>
              </div>
            </div>
            <div class="bg-muted/40 rounded-lg p-4 text-sm">
              {{ t('comp.skeletonContent') }}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
