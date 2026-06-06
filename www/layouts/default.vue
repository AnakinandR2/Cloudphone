<template>
  <div>
    <PromoBanner />
    <AppNavbar />
    <main>
      <slot />
    </main>
    <AppFooter />
    <ClientOnly>
      <TweakPanel />
    </ClientOnly>
    <ClientOnly>
      <CookieConsent />
    </ClientOnly>
  </div>
</template>
