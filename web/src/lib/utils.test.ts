import { cva } from "class-variance-authority"
import { describe, expect, it } from "vitest"

import { cn } from "./utils"

const controlVariants = cva("p-3 text-sm bg-primary", {
  variants: {
    size: {
      large: "p-4 text-base",
    },
  },
})

const className = (...parts: string[]) => parts.join("")

describe("cn", () => {
  it("joins conditional, object, and nested array classes", () => {
    expect(cn("bean-control", false, null, undefined, { active: true, hidden: false }, ["nested", ["deep"]])).toBe(
      "bean-control active nested deep",
    )
  })

  it("resolves conflicting spacing classes", () => {
    expect(cn("p-3", "p-4")).toBe("p-4")
  })

  it("resolves conflicting text and background classes", () => {
    expect(cn("text-sm bg-primary", "text-base bg-muted")).toBe("text-base bg-muted")
  })

  it("scopes conflicts to responsive variants", () => {
    expect(cn("grid sm:grid-cols-2", "sm:grid-cols-3 lg:grid-cols-4")).toBe("grid sm:grid-cols-3 lg:grid-cols-4")
  })

  it("scopes conflicts to pseudo and state variants", () => {
    expect(cn("hover:bg-muted focus-visible:ring-2", "hover:bg-surface-hover focus-visible:ring-3")).toBe(
      "hover:bg-surface-hover focus-visible:ring-3",
    )
  })

  it("resolves arbitrary values and Tailwind v4 variable shorthand", () => {
    const rounded = className("rounded-", "[4px]")
    const roundedOverride = className("rounded-", "[5px]")
    const variableGap = className("gap-", "(--card-spacing)")

    expect(cn(`${rounded} ${variableGap} py-(--card-spacing)`, `${roundedOverride} gap-3`)).toBe(
      `py-(--card-spacing) ${roundedOverride} gap-3`,
    )
  })

  it("scopes conflicts to dark mode", () => {
    expect(cn("text-foreground dark:bg-input/30", "dark:bg-destructive/20")).toBe(
      "text-foreground dark:bg-destructive/20",
    )
  })

  it("preserves normal utilities when an important utility overrides them", () => {
    expect(cn("[&>svg]:size-4", "[&>svg]:size-3!")).toBe("[&>svg]:size-4 [&>svg]:size-3!")
  })

  it("resolves conflicts within matching arbitrary variants", () => {
    const iconSize = className("[&>svg]", ":", "size-4")
    const iconSizeOverride = className("[&>svg]", ":", "size-5")

    expect(cn(iconSize, iconSizeOverride)).toBe(iconSizeOverride)
  })

  it("merges CVA-produced classes with caller overrides", () => {
    expect(cn(controlVariants({ size: "large" }), "p-3 bg-muted")).toBe("text-base p-3 bg-muted")
  })
})
