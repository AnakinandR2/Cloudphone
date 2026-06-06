<script setup lang="ts">
import type { Locale } from '@/locales'

import { Check, Languages } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { localeOptions } from '@/locales'
import { useSettingsStore } from '@/stores/settings'

const store = useSettingsStore()

function pick(value: Locale) {
  store.settings.locale = value
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="ghost" size="icon" title="Language / 语言">
        <Languages class="size-4" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-36">
      <DropdownMenuItem
        v-for="opt in localeOptions"
        :key="opt.value"
        @click="pick(opt.value)"
      >
        <span class="flex-1">{{ opt.label }}</span>
        <Check v-if="store.settings.locale === opt.value" class="size-4" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
