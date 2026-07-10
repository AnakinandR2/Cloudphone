<script setup lang="ts">
import { ArrowUpRight } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'

export interface ResourceAction {
  label: string
  variant: 'default' | 'outline'
  onClick: () => void
}

withDefaults(defineProps<{
  title: string
  value: string | number
  /** 大数字右侧单位/总量，如 "/ 100"、"分钟" */
  valueSuffix?: string
  description: string
  illustration: string
  actions: ResourceAction[]
  showProductLink?: boolean
}>(), {
  showProductLink: true,
})

const emit = defineEmits<{ productLink: [] }>()
const { t } = useI18n()
</script>

<template>
  <!--
    描边/背景/阴影都在同一层，避免内外双圆角叠出白边；
    仅插画层 overflow 裁切，不影响外阴影。
  -->
  <article class="resource-card group/card relative min-h-[206px] w-full max-w-[600px] rounded-2xl border border-[#d7dbe2]">
    <div class="pointer-events-none absolute inset-0 overflow-hidden rounded-[inherit]">
      <!-- 插画略放大裁切：PNG 上下自带 1～2px 白边，贴齐高度时会露出白条 -->
      <img
        :src="illustration"
        alt=""
        width="600"
        height="206"
        class="resource-card__art absolute top-1/2 right-0 h-[214px] w-[620px] max-w-none -translate-y-1/2 object-cover object-right transition-transform duration-[450ms] ease-[cubic-bezier(0.22,1,0.36,1)] group-hover/card:scale-[1.02]"
      >
      <!-- 下半部分渐进高斯模糊，减轻与「产品介绍」等文案的重叠干扰 -->
      <img
        :src="illustration"
        alt=""
        aria-hidden="true"
        width="600"
        height="206"
        class="resource-card__art resource-card__art--blur absolute top-1/2 right-0 h-[214px] w-[620px] max-w-none -translate-y-1/2 object-cover object-right transition-transform duration-[450ms] ease-[cubic-bezier(0.22,1,0.36,1)] group-hover/card:scale-[1.02]"
      >
    </div>

    <div class="relative z-[1] flex h-full min-h-[206px] flex-col justify-between gap-8 p-[25px]">
      <div class="flex max-w-[58%] flex-col gap-4">
        <p class="text-base font-medium leading-6 tracking-[0.048px] text-[#1f2329]">
          {{ title }}
        </p>
        <div class="flex flex-col">
          <div class="flex flex-wrap items-baseline gap-2 text-[#1f2329]">
            <span class="text-[28px] font-semibold leading-9 tracking-[0.084px] tabular-nums">{{ value }}</span>
            <span v-if="valueSuffix" class="text-sm leading-5 tracking-[0.042px] tabular-nums">{{ valueSuffix }}</span>
          </div>
          <p class="text-sm leading-5 tracking-[0.042px] text-[#8f959e]">
            {{ description }}
          </p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <Button
            v-for="action in actions"
            :key="action.label"
            size="sm"
            :variant="action.variant"
            class="min-w-[72px] rounded-[8px] px-6"
            @click="action.onClick"
          >
            {{ action.label }}
          </Button>
        </div>
        <Button
          v-if="showProductLink"
          variant="link"
          size="sm"
          class="h-8 gap-1 px-1"
          @click="emit('productLink')"
        >
          {{ t('billing.purchase2.productIntro') }}
          <ArrowUpRight class="size-3.5" />
        </Button>
      </div>
    </div>
  </article>
</template>

<style scoped>
.resource-card {
  /* 与插画底色一致，避免裁切缝隙露出纯白 */
  background-color: #eefcf8;
  background-image: radial-gradient(
    ellipse 95% 110% at 100% 100%,
    #e7fcf6 0%,
    #e7fcf6 28%,
    rgb(231 252 246 / 0.55) 48%,
    transparent 72%
  );
  box-shadow: 0 1px 1px rgb(15 17 26 / 0.05);
  transition:
    transform 0.4s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.4s cubic-bezier(0.22, 1, 0.36, 1),
    border-color 0.4s cubic-bezier(0.22, 1, 0.36, 1),
    background-image 0.4s cubic-bezier(0.22, 1, 0.36, 1);
}

.resource-card__art--blur {
  filter: blur(10px);
  transform-origin: right center;
  /* 上半透明 → 下半实显，形成渐进模糊 */
  -webkit-mask-image: linear-gradient(
    to bottom,
    transparent 38%,
    rgb(0 0 0 / 0.45) 62%,
    black 100%
  );
  mask-image: linear-gradient(
    to bottom,
    transparent 38%,
    rgb(0 0 0 / 0.45) 62%,
    black 100%
  );
}

.resource-card:hover {
  transform: translateY(-4px);
  border-color: color-mix(in oklab, var(--primary) 22%, #d7dbe2);
  background-image: radial-gradient(
    ellipse 105% 120% at 100% 100%,
    #e7fcf6 0%,
    #e7fcf6 32%,
    rgb(231 252 246 / 0.65) 52%,
    transparent 75%
  );
  box-shadow:
    0 14px 28px -10px rgb(15 23 42 / 14%),
    0 6px 14px -6px rgb(20 184 166 / 16%);
}
</style>
