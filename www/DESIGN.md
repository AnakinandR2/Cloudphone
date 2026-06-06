# Gloryphone — Design System (DESIGN.md)

> Machine-readable design spec for the `www/` marketing site.
> Source of truth: ported 1:1 from the high-fidelity reference at
> `ref/sytle12-claude-design/`. When editing UI, **follow this file** — do not
> reintroduce shadcn/Tailwind-utility styling for page/section chrome.

---

## 1. Stack & where things live

| Concern | Location |
|---|---|
| Framework | Nuxt 3 (Vue 3 `<script setup>`, SSR) |
| Styling | Global CSS design system in [`assets/css/tailwind.css`](assets/css/tailwind.css) (Tailwind directives + ported `app.css`) |
| Dark mode | `@nuxtjs/color-mode`, `classSuffix: ''` → toggles `.dark` / `.light` class on `<html>` |
| Accent color | `<html data-accent="...">`, set from the `gp-accent` cookie ([`app.vue`](app.vue), [`composables/useThemeColor.ts`](composables/useThemeColor.ts)) |
| i18n | `@nuxtjs/i18n` (`en` default, `zh`), `no_prefix` strategy — drives locale switching only |
| Content/copy | [`composables/useGp.ts`](composables/useGp.ts) — full `GP_CONTENT` dict (`zh` + `en`), **not** the i18n JSON |
| Icons | [`components/GpIcon.vue`](components/GpIcon.vue) (`name` prop) + [`components/GpPartnerMark.vue`](components/GpPartnerMark.vue) |
| Scroll reveals | [`plugins/reveal.client.ts`](plugins/reveal.client.ts) |

**Rule of thumb:** style with the **design-system class names** below (e.g. `.section`, `.btn`, `.card`, `.eyebrow`), not ad-hoc Tailwind utilities. Inline `style="..."` is acceptable for one-off spacing, matching the reference.

---

## 2. Design tokens (CSS variables)

All tokens are defined in [`assets/css/tailwind.css`](assets/css/tailwind.css). Colors are stored as **space-separated RGB triplets** and consumed via `rgb(var(--token) / <alpha>)`.

### Neutral scale (light default; `.dark` overrides)
| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg` | `252 252 253` | `11 12 16` | page background |
| `--bg-elev` | `255 255 255` | `17 19 25` | cards, raised surfaces |
| `--bg-sunken` | `246 246 249` | `14 15 20` | alternating section backgrounds |
| `--bg-inset` | `240 240 245` | `21 23 30` | quiet buttons, insets |
| `--border` | `230 230 236` | `33 36 46` | default borders |
| `--border-strong` | `215 215 224` | `50 54 68` | hover/emphasis borders |
| `--fg` | `17 18 22` | `240 241 245` | primary text |
| `--fg-muted` | `86 90 102` | `168 172 186` | secondary text |
| `--fg-subtle` | `130 135 150` | `120 125 140` | tertiary/meta text |

### Accent (swappable — see §3)
| Token | Form | Use |
|---|---|---|
| `--accent` / `--accent-strong` | RGB triplet | alpha expressions: `rgb(var(--accent) / 0.1)` |
| `--accent-color` / `--accent-strong-color` | `#hex` | solid fills, text, borders |
| `--accent-fg` | `255 255 255` | text on accent fills |

### Other
- Radii: `--radius-xs:6 sm:10 md:14 lg:20 xl:28 full:999`px
- Shadows: `--shadow-sm | -md | -lg | -glow`
- Fonts: `--font-display` & `--font-body` = **Manrope**, `--font-mono` = **JetBrains Mono**
- Layout: `--container: 1240px`, `--nav-h: 68px`, `--promo-h: 44px`

---

## 3. Accent (theme) colors

8 presets, keyed off `[data-accent="<value>"]`. **Default: `green`.** Defined as CSS presets in [`assets/css/tailwind.css`](assets/css/tailwind.css) and listed in [`composables/useThemeColor.ts`](composables/useThemeColor.ts).

`purple` `blue` `teal` **`green`** `orange` `pink` `indigo` `rose`

- Read/set via `useThemeColor()` → `{ accent, accents, setAccent }`. `setAccent()` persists the `gp-accent` cookie and updates `<html data-accent>`.
- Accent does **not** change between light/dark; only the neutral scale does.
- The [`TweakPanel`](components/TweakPanel.vue) FAB (bottom-right) is the user-facing control for both accent and light/dark/system.

To add an accent: add a `[data-accent='x'] { --accent…; --accent-color… }` block in the CSS **and** an entry in `ACCENTS`.

---

## 4. Component / class vocabulary

Reusable primitives (full rules in [`assets/css/tailwind.css`](assets/css/tailwind.css)):

| Class | What it is |
|---|---|
| `.container` | max-width 1240px, centered, 24px gutter |
| `.section` / `#id` | 96px vertical rhythm; alternate bg with inline `style="background: rgb(var(--bg-sunken))"` |
| `.eyebrow` | small uppercase accent pill above a heading |
| `.section-title` / `.section-sub` | section heading + subtitle |
| `.btn` + `.btn-primary` / `.btn-ghost` / `.btn-quiet` + `.btn-sm` / `.btn-lg` | buttons |
| `.card` / `.feature-card` | bordered elevated card |
| `.icon-btn` | 36px square icon button (nav tools) |
| `.brand` + `.brand-mark` | logo lockup (CSS-drawn mark) |
| `.popover` / `.popover-wrap` | dropdown (language picker) |
| Section blocks | `.hero` `.hero-stats` `.logo-strip` `.features-grid` `.feature-spotlight` `.usecase-tabs/.usecase-panel` `.compare-table` `.perf-grid` `.pricing-grid/.price-card` `.testi-grid/.testi-card` `.dl-grid/.dl-card` `.blog-grid/.blog-card` `.faq-list/.faq-item` `.cta-banner` `.footer` |
| Reveal | add nothing — `.reveal`/`.in-view` are applied automatically by the plugin to known selectors |

### Icons
```vue
<GpIcon name="fingerprint" />          <!-- size comes from CSS context -->
<GpIcon name="arrow" style="width:14px;height:14px" />
```
Available: `moon sun globe palette check check-bold arrow arrow-sm burger x shield cpu fingerprint clock users code wifi win apple android plus system`. Icons carry **no** width/height attrs so surrounding CSS controls size (matches reference). Add new icons as a `v-else-if` branch in `GpIcon.vue`.

---

## 5. Content model

All copy lives in `GP_CONTENT` in [`composables/useGp.ts`](composables/useGp.ts), shaped `{ zh: {...}, en: {...} }`. Consume in any component:

```vue
<script setup lang="ts">
const { t } = useGp()          // t is a computed ref of the active locale's object
</script>
<template>{{ t.hero.title1 }}</template>   <!-- template auto-unwraps the ref -->
```
In `<script>` use `t.value.hero.title1`. Top-level keys: `promo nav hero stats logos features spotlight scenarios compare perf pricing testi download blog faq cta footer theme`.

**When adding/changing copy, edit both `zh` and `en`.** Keep the two locales structurally identical (same array lengths) — components index by position.

---

## 6. Page composition

| Route | File | Sections |
|---|---|---|
| `/` | [`pages/index.vue`](pages/index.vue) | Hero → Features → UseCases → Compare → Pricing → Testimonials → Downloads → Blog → FAQ → Cta |
| `/pricing` | [`pages/pricing.vue`](pages/pricing.vue) | Pricing → Compare → FAQ → Cta |
| `/faq` | [`pages/faq.vue`](pages/faq.vue) | FAQ → Cta |
| `/blog` | [`pages/blog/index.vue`](pages/blog/index.vue) | featured + grid + category tabs |
| `/blog/:slug` | [`pages/blog/[slug].vue`](pages/blog/%5Bslug%5D.vue) | article + related |

Chrome (`PromoBanner`, `AppNavbar`, `AppFooter`, `TweakPanel`) is mounted once in [`layouts/default.vue`](layouts/default.vue). Section components live in `components/sections/` and auto-import by filename.

---

## 7. Conventions for agents

1. **Reuse the class vocabulary** in §4; don't restyle sections with raw Tailwind utilities.
2. **Colors** → always `rgb(var(--token) / a)` (triplets) or `var(--accent-color)` (hex). Never hardcode brand colors.
3. **Dark mode** → never hardcode light values; rely on the `--bg/--fg/...` tokens which flip under `.dark`.
4. **Text** → from `useGp()` (`GP_CONTENT`), both locales, same structure.
5. **Icons** → `GpIcon` / `GpPartnerMark` only.
6. **New section** → make `components/sections/XxxSection.vue` wrapping `<section class="section" id="xxx">`, pull copy from `useGp`, then add to `pages/index.vue`. Reveal animation is automatic if it uses a known reveal selector (e.g. `.card`, `.feature-card`); otherwise extend the selector list in `plugins/reveal.client.ts`.
7. **Images** → put static assets in `public/images/` and reference as `/images/<file>`.
8. **Verify** → `pnpm build`, then `node .output/server/index.mjs` and check `/`.
