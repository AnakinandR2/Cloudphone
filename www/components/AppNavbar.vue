<script setup lang="ts">
const { t } = useGp()
const { locale, locales, setLocale } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const colorMode = useColorMode()

const runtime = useRuntimeConfig()
const authUser = useAuthUser()
const isLoggedIn = computed(() => !!authUser.value)
const displayName = computed(() => authUser.value?.nickname || authUser.value?.phone || '')
const myAppPath = runtime.public.myAppPath
async function onLogout() {
  await logoutAuthUser()
}

const mobile = ref(false)
const langOpen = ref(false)
const langWrap = ref<HTMLElement | null>(null)

const isDark = computed(() => colorMode.value === 'dark')
function toggleDark() {
  colorMode.preference = isDark.value ? 'light' : 'dark'
}

interface NavLink {
  id: string
  href: string
  label: string
  children?: { id: string; href: string; label: string }[]
}

const links = computed<NavLink[]>(() => [
  { id: 'features', href: localePath('/') + '#features', label: t.value.nav.features },
  { id: 'scenarios', href: localePath('/') + '#scenarios', label: t.value.nav.scenarios },
  { id: 'pricing', href: localePath('/') + '#pricing', label: t.value.nav.pricing },
  { id: 'download', href: localePath('/') + '#download', label: t.value.nav.download },
  {
    id: 'resources',
    href: localePath('/blog'),
    label: t.value.nav.resources,
    children: [
      { id: 'blog', href: localePath('/blog'), label: t.value.nav.blog },
      { id: 'docs', href: localePath('/help'), label: t.value.nav.docs },
      { id: 'faq', href: localePath('/faq'), label: t.value.nav.faq },
      { id: 'apiDocs', href: localePath('/api-docs'), label: t.value.nav.apiDocs },
    ],
  },
])

const availableLocales = computed(() =>
  (locales.value as Array<{ code: string; name?: string }>).map((l) => ({ code: l.code, name: l.name ?? l.code })),
)

function pickLang(code: string) {
  setLocale(code as any)
  langOpen.value = false
}

function onDocClick(e: MouseEvent) {
  if (langOpen.value && langWrap.value && !langWrap.value.contains(e.target as Node)) langOpen.value = false
}
onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
watch(() => route.fullPath, () => (mobile.value = false))
</script>

<template>
  <header class="nav">
    <div class="container nav-inner">
      <NuxtLink :to="localePath('/')" class="brand">
        <span class="brand-mark" />
        <span>Gloryphone</span>
      </NuxtLink>

      <nav class="nav-links">
        <template v-for="l in links" :key="l.id">
          <div v-if="l.children" class="nav-dropdown">
            <NuxtLink :to="l.href" class="nav-link nav-dropdown__trigger">{{ l.label }}</NuxtLink>
            <div class="nav-dropdown__menu">
              <NuxtLink v-for="c in l.children" :key="c.id" :to="c.href" class="nav-dropdown__item">{{ c.label }}</NuxtLink>
            </div>
          </div>
          <NuxtLink v-else :to="l.href" class="nav-link">{{ l.label }}</NuxtLink>
        </template>
      </nav>

      <div class="nav-tools">
        <button class="icon-btn" :aria-label="t.theme.dark" @click="toggleDark">
          <GpIcon :name="isDark ? 'sun' : 'moon'" />
        </button>

        <div ref="langWrap" class="popover-wrap">
          <button class="icon-btn" aria-label="Language" @click="langOpen = !langOpen">
            <GpIcon name="globe" />
          </button>
          <div class="popover" :class="{ open: langOpen }">
            <button v-for="l in availableLocales" :key="l.code" @click="pickLang(l.code)">
              <span style="flex: 1">{{ l.name }}</span>
              <GpIcon v-if="locale === l.code" name="check" style="width: 14px; height: 14px" />
            </button>
          </div>
        </div>

        <template v-if="isLoggedIn">
          <a :href="myAppPath" class="btn btn-ghost btn-sm" style="margin-left: 4px" :title="displayName"><span class="nav-cta-text">{{ t.nav.console }}</span></a>
          <a href="#" class="btn btn-primary btn-sm" @click.prevent="onLogout">{{ t.nav.logout }}</a>
        </template>
        <template v-else>
          <a :href="`${myAppPath}login`" class="btn btn-ghost btn-sm" style="margin-left: 4px"><span class="nav-cta-text">{{ t.nav.login }}</span></a>
          <a :href="`${myAppPath}register`" class="btn btn-primary btn-sm">{{ t.nav.signup }}</a>
        </template>
        <button class="icon-btn nav-burger" aria-label="Menu" @click="mobile = !mobile">
          <GpIcon :name="mobile ? 'x' : 'burger'" />
        </button>
      </div>
    </div>

    <ClientOnly>
      <Teleport to="body">
        <div class="mobile-menu" :class="{ open: mobile }">
          <template v-for="l in links" :key="l.id">
            <NuxtLink :to="l.href" @click="mobile = false">{{ l.label }}</NuxtLink>
            <NuxtLink
              v-for="c in l.children || []"
              :key="c.id"
              :to="c.href"
              class="mobile-menu__sub"
              @click="mobile = false"
            >
              {{ c.label }}
            </NuxtLink>
          </template>
          <div class="mobile-menu__tools">
            <button class="icon-btn" :aria-label="t.theme.dark" @click="toggleDark">
              <GpIcon :name="isDark ? 'sun' : 'moon'" />
            </button>
          </div>
          <div class="mobile-menu__cta">
            <template v-if="isLoggedIn">
              <a :href="myAppPath" class="btn btn-ghost">{{ t.nav.console }}</a>
              <a href="#" class="btn btn-primary" @click.prevent="onLogout">{{ t.nav.logout }}</a>
            </template>
            <template v-else>
              <a :href="`${myAppPath}login`" class="btn btn-ghost">{{ t.nav.login }}</a>
              <a :href="`${myAppPath}register`" class="btn btn-primary">{{ t.nav.signup }}</a>
            </template>
          </div>
        </div>
      </Teleport>
    </ClientOnly>
  </header>
</template>
