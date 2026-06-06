import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export { default as Button } from "./Button.vue"

// Original shadcn geometry (rounded-md, h-9/8/10, text-sm) — restored.
// Kept from the Gloryphone pass: accent-tinted shadow + hover lift on the
// prominent (primary / destructive) buttons. Shadow values live in index.css
// (--btn-shadow / --btn-shadow-hover) and track the active theme color.
export const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all duration-200 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[var(--btn-shadow)] hover:bg-[var(--primary-strong)] hover:-translate-y-px hover:shadow-[var(--btn-shadow-hover)] active:translate-y-0",
        destructive:
          "bg-destructive text-white shadow-[var(--btn-shadow-destructive)] hover:bg-destructive/90 hover:-translate-y-px hover:shadow-[var(--btn-shadow-destructive-hover)] active:translate-y-0 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60",
        warning:
          "bg-amber-500 text-white shadow-xs hover:bg-amber-600 hover:-translate-y-px active:translate-y-0 focus-visible:border-amber-500 focus-visible:ring-amber-500/40 dark:bg-amber-500/90 dark:hover:bg-amber-500",
        outline:
          "border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground hover:-translate-y-px active:translate-y-0 dark:bg-input/30 dark:border-input dark:hover:bg-input/50",
        secondary:
          "bg-secondary text-secondary-foreground shadow-xs hover:bg-secondary/80 hover:-translate-y-px active:translate-y-0",
        ghost:
          "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        "default": "h-9 px-4 py-2 has-[>svg]:px-3",
        "sm": "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
        "lg": "h-10 rounded-md px-6 has-[>svg]:px-4",
        "icon": "size-9",
        "icon-sm": "size-8",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
)
export type ButtonVariants = VariantProps<typeof buttonVariants>
