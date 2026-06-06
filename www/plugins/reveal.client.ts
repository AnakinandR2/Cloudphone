// Scroll-reveal binder — ported from the reference design's setupReveals().
// Adds .reveal / .in-view to content blocks as they enter the viewport, plus
// per-section header stagger. Rebinds after every client-side navigation.
export default defineNuxtPlugin((nuxtApp) => {
  const REVEAL_SELECTORS = [
    '.feature-card', '.price-card', '.dl-card', '.blog-card', '.testi-card',
    '.perf-bar', '.usecase-panel', '.feature-spotlight', '.compare-row',
    '.cta-banner', '.faq-item',
  ].join(',')

  let observers: IntersectionObserver[] = []

  function cleanup() {
    observers.forEach((o) => o.disconnect())
    observers = []
  }

  function bind() {
    cleanup()
    if (typeof window === 'undefined' || !('IntersectionObserver' in window)) return

    const io = new IntersectionObserver((entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('in-view')
          io.unobserve(e.target)
        }
      })
    }, { threshold: 0.12, rootMargin: '0px 0px -6% 0px' })
    observers.push(io)

    document.querySelectorAll<HTMLElement>(REVEAL_SELECTORS).forEach((el) => {
      el.classList.add('reveal')
      const siblings = [...(el.parentElement?.children || [])].filter((c) => (c as Element).matches(REVEAL_SELECTORS))
      const idx = siblings.indexOf(el)
      el.style.setProperty('--reveal-i', String(Math.max(0, idx % 8)))
      io.observe(el)
    })

    document.querySelectorAll('.hero-stats').forEach((el) => io.observe(el))

    const headerIO = new IntersectionObserver((ents) => {
      ents.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('header-in')
          headerIO.unobserve(e.target)
        }
      })
    }, { threshold: 0.15 })
    observers.push(headerIO)
    document.querySelectorAll('.section').forEach((sec) => headerIO.observe(sec))
  }

  function rebind() {
    nextTick(() => requestAnimationFrame(bind))
  }

  nuxtApp.hook('app:mounted', rebind)
  nuxtApp.hook('page:finish', rebind)
})
