<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { useMenuStore } from '@/stores/menu'

const route = useRoute()
const menuStore = useMenuStore()
const { t } = useI18n()

const crumbs = computed(() => menuStore.getBreadcrumb(route.path))
</script>

<template>
  <Breadcrumb v-if="crumbs.length">
    <BreadcrumbList>
      <template v-for="(crumb, index) in crumbs" :key="index">
        <BreadcrumbItem>
          <!-- 最后一项：当前页 -->
          <BreadcrumbPage v-if="index === crumbs.length - 1">
            {{ t(crumb.title) }}
          </BreadcrumbPage>
          <!-- 可点击的中间项（有 path） -->
          <BreadcrumbLink v-else-if="crumb.path" as-child>
            <RouterLink :to="crumb.path">
              {{ t(crumb.title) }}
            </RouterLink>
          </BreadcrumbLink>
          <!-- 不可点击的分组/中间项 -->
          <span v-else class="text-muted-foreground">{{ t(crumb.title) }}</span>
        </BreadcrumbItem>
        <BreadcrumbSeparator v-if="index < crumbs.length - 1" />
      </template>
    </BreadcrumbList>
  </Breadcrumb>
</template>
