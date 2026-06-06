<script setup lang="ts">
const { t, locale } = useGp()
const { accent, accents, setAccent } = useThemeColor()
const colorMode = useColorMode()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

const modeLabel = computed(() => ({
  light: locale.value === 'zh' ? '明亮' : 'Light',
  dark: locale.value === 'zh' ? '暗黑' : 'Dark',
  system: locale.value === 'zh' ? '跟随系统' : 'System',
}))

function onDocClick(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => {
  document.addEventListener('mousedown', onDocClick)
  document.addEventListener('touchstart', onDocClick)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick)
  document.removeEventListener('touchstart', onDocClick)
})
</script>

<template>
  <div ref="root" class="tweak">
    <button class="tweak-fab" :class="{ open }" aria-label="Tweak appearance" :aria-expanded="open" @click="open = !open">
      <GpIcon name="palette" />
      <span class="tweak-fab__label">Tweak</span>
    </button>

    <div class="tweak-panel" :class="{ open }" role="dialog" :aria-hidden="!open">
      <div class="tweak-panel__head">
        <strong>{{ t.theme.appearance }}</strong>
        <button class="tweak-panel__close" aria-label="Close" @click="open = false"><GpIcon name="x" /></button>
      </div>

      <div class="tweak-panel__group">
        <div class="tweak-panel__label">{{ t.theme.accent }}</div>
        <div class="tweak-swatches">
          <button
            v-for="a in accents"
            :key="a.value"
            class="tweak-swatch"
            :class="{ active: accent === a.value }"
            :style="{ background: a.swatch }"
            :title="locale === 'zh' ? a.name.zh : a.name.en"
            :aria-label="locale === 'zh' ? a.name.zh : a.name.en"
            @click="setAccent(a.value)"
          >
            <GpIcon v-if="accent === a.value" name="check" />
          </button>
        </div>
      </div>

      <div class="tweak-panel__group">
        <div class="tweak-panel__label">{{ t.theme.mode }}</div>
        <div class="tweak-modes">
          <button
            v-for="m in ['light', 'dark', 'system']"
            :key="m"
            class="tweak-mode"
            :class="{ active: colorMode.preference === m }"
            @click="colorMode.preference = m"
          >
            <GpIcon v-if="m === 'light'" name="sun" />
            <GpIcon v-else-if="m === 'dark'" name="moon" />
            <GpIcon v-else name="system" />
            <span>{{ (modeLabel as any)[m] }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
