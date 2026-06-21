<script setup lang="ts">
import { Info } from 'lucide-vue-next'

// 按 kind 展示须知文案（admin 可配，由 purchase-config 提供）。
defineProps<{
  notice?: string
  billingNote?: string
}>()
</script>

<template>
  <div v-if="notice || billingNote" class="bg-muted/40 flex gap-2.5 rounded-lg border px-3.5 py-3 text-sm">
    <Info class="text-muted-foreground mt-0.5 size-4 shrink-0" />
    <div class="text-muted-foreground space-y-1 leading-relaxed">
      <p v-if="notice" class="whitespace-pre-line">
        {{ notice }}
      </p>
      <p v-if="billingNote" class="whitespace-pre-line text-xs">
        {{ billingNote }}
      </p>
    </div>
  </div>
</template>
