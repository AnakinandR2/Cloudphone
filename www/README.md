# GloryPhone — SaaS Marketing Website

A modern SaaS company website built with **Nuxt 3 + Vue 3 + shadcn-vue + Tailwind CSS**.

## Features

- 🎨 **Light / Dark / System** theme — via `@nuxtjs/color-mode`
- 🌈 **Swappable theme color** — 5 accent presets (Violet, Blue, Emerald, Rose, Orange), persisted in a cookie
- 🌍 **Multi-language** — English + 简体中文 via `@nuxtjs/i18n` (cookie-based, no URL prefix)
- 📱 **Fully responsive** — tested at 375 / 768 / 1024 / 1440px
- ♿ **Accessible** — focus states, ARIA labels, reduced-motion support, 4.5:1 contrast
- 🧩 **shadcn-vue components** — Button, Card, Badge, Accordion, Dropdown Menu, Input

## Content sections

Hero · Logo cloud · Features · **Pricing** (monthly/yearly toggle) · **Blog** (list + article) · **FAQ** (accordion) · CTA · Footer (newsletter)

## Design system

| Token | Choice |
| --- | --- |
| Style | Clean minimal SaaS with subtle gradients |
| Headings | Space Grotesk |
| Body | DM Sans |
| Default accent | Violet `hsl(262 83% 58%)` |

## Project structure

```
assets/css/tailwind.css     # design tokens + accent presets
components/ui/              # shadcn-vue primitives
components/                 # AppNavbar, AppFooter, ThemeToggle, ColorPicker, LangSwitcher, BlogCard
components/sections/        # Hero, Features, Pricing, Blog, Faq, Cta, ...
composables/useThemeColor.ts# swappable accent logic
composables/useContent.ts   # pricing plans + blog metadata
i18n/locales/{en,zh}.json   # translations
pages/                      # index, pricing, faq, blog/index, blog/[slug]
```

## Develop

```bash
pnpm install
pnpm dev          # http://localhost:3000
```

## Build

```bash
pnpm build        # SSR build
pnpm generate     # static site
pnpm preview
```
