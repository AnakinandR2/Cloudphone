<script setup lang="ts">
defineProps<{
  title: string
  iconSrc: string
  value: string | number
  suffix?: string
  actions: { label: string, onClick: () => void }[]
}>()
</script>

<template>
  <div class="kpi-card flex min-w-0 flex-1 flex-col gap-10 overflow-hidden rounded-[24px] p-6">
    <div class="relative z-[1] flex flex-col gap-6">
      <img :src="iconSrc" alt="" class="size-12 shrink-0">
      <div class="space-y-1">
        <p class="text-lg font-semibold text-white">
          {{ title }}
        </p>
        <div class="flex items-baseline gap-2 text-white">
          <span class="text-[40px] font-bold leading-10 tabular-nums">{{ value }}</span>
          <span v-if="suffix" class="text-base font-medium">{{ suffix }}</span>
        </div>
      </div>
    </div>
    <div class="relative z-[1] flex gap-2">
      <button
        v-for="action in actions"
        :key="action.label"
        type="button"
        class="h-11 min-w-[112px] flex-1 rounded-[12px] bg-black/20 px-7 text-base font-semibold text-white transition-colors hover:bg-black/30"
        @click="action.onClick"
      >
        {{ action.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.kpi-card {
  position: relative;
  background-color: rgb(0 0 0 / 20%);
  box-shadow: 0 18px 45px rgb(15 23 42 / 8%);
  transition:
    transform 0.4s cubic-bezier(0.22, 1, 0.36, 1),
    box-shadow 0.4s cubic-bezier(0.22, 1, 0.36, 1);
}

.kpi-card::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0;
  border-radius: inherit;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 400 252' xmlns='http://www.w3.org/2000/svg' preserveAspectRatio='none'%3E%3Crect width='100%25' height='100%25' fill='url(%23grad)'/%3E%3Cdefs%3E%3CradialGradient id='grad' gradientUnits='userSpaceOnUse' cx='0' cy='0' r='10' gradientTransform='matrix(-0.0000020862 25.2 -40 -0.0000041909 200 -0.0000090864)'%3E%3Cstop stop-color='rgba(0,0,0,1)' offset='0'/%3E%3Cstop stop-color='rgba(0,0,0,0)' offset='0.86197'/%3E%3C/radialGradient%3E%3C/defs%3E%3C/svg%3E");
  transition: opacity 0.4s cubic-bezier(0.22, 1, 0.36, 1);
}

.kpi-card::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 50%;
  width: 72%;
  height: 1px;
  pointer-events: none;
  opacity: 0;
  transform: translateX(-50%) scaleX(0.72);
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgb(255 255 255 / 85%) 50%,
    transparent 100%
  );
  transition:
    opacity 0.4s cubic-bezier(0.22, 1, 0.36, 1) 0.04s,
    transform 0.45s cubic-bezier(0.22, 1, 0.36, 1) 0.04s;
}

.kpi-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 24px 52px rgb(15 23 42 / 12%);
}

.kpi-card:hover::before {
  opacity: 1;
}

.kpi-card:hover::after {
  opacity: 1;
  transform: translateX(-50%) scaleX(1);
}
</style>
