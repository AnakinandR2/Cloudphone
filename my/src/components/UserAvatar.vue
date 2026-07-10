<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { avatarFallbackClass, getUserIdInitial } from '@/lib/avatar'
import { cn } from '@/lib/utils'
import { useUserStore } from '@/stores/user'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()

const userStore = useUserStore()
const label = computed(() => getUserIdInitial(userStore.id))
</script>

<template>
  <Avatar :class="cn('size-8', props.class)">
    <AvatarFallback :class="avatarFallbackClass">
      {{ label }}
    </AvatarFallback>
  </Avatar>
</template>
