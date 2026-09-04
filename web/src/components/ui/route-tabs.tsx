import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const routeTabsListVariants = cva(
  "inline-flex w-fit items-center justify-center text-muted-foreground",
  {
    variants: {
      variant: {
        default: "rounded-md border border-border bg-surface-subtle p-0.5",
        line: "gap-1 border-b border-border-subtle bg-transparent",
      },
      orientation: {
        horizontal: "h-9 max-w-full overflow-x-auto",
        vertical: "h-auto w-full flex-col items-stretch justify-start",
      },
    },
    defaultVariants: {
      variant: "default",
      orientation: "horizontal",
    },
  }
)

const routeTabsLinkVariants = cva(
  "relative inline-flex min-w-0 items-center justify-center gap-1.5 whitespace-nowrap border border-transparent px-2 py-1 text-sm font-medium text-foreground/60 transition-[color,box-shadow,background-color] outline-none hover:text-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:text-foreground",
  {
    variants: {
      variant: {
        default:
          "h-[calc(100%-1px)] rounded-sm data-[state=active]:bg-surface data-[state=active]:shadow-[0_1px_2px_rgb(0_0_0/0.08)]",
        line:
          "rounded-none after:absolute after:bg-primary after:opacity-0 after:transition-opacity data-[state=active]:after:opacity-100",
      },
      orientation: {
        horizontal: "after:inset-x-1 after:-bottom-1 after:h-0.5",
        vertical:
          "w-full justify-start after:inset-y-1 after:-right-1 after:w-0.5",
      },
    },
    defaultVariants: {
      variant: "default",
      orientation: "horizontal",
    },
  }
)

type RouteTabsVariant = NonNullable<VariantProps<typeof routeTabsListVariants>["variant"]>
type RouteTabsOrientation = NonNullable<VariantProps<typeof routeTabsListVariants>["orientation"]>

function RouteTabsList({
  className,
  variant = "default",
  orientation = "horizontal",
  ...props
}: React.ComponentProps<"nav"> & {
  variant?: RouteTabsVariant
  orientation?: RouteTabsOrientation
}) {
  return (
    <nav
      data-slot="route-tabs-list"
      data-variant={variant}
      data-orientation={orientation}
      className={cn(routeTabsListVariants({ variant, orientation }), className)}
      {...props}
    />
  )
}

function RouteTabsLink({
  className,
  variant = "default",
  orientation = "horizontal",
  active = false,
  asChild = false,
  ...props
}: React.ComponentProps<"a"> & {
  variant?: RouteTabsVariant
  orientation?: RouteTabsOrientation
  active?: boolean
  asChild?: boolean
}) {
  const Comp = asChild ? Slot.Root : "a"
  return (
    <Comp
      data-slot="route-tabs-link"
      data-variant={variant}
      data-orientation={orientation}
      data-state={active ? "active" : "inactive"}
      className={cn(routeTabsLinkVariants({ variant, orientation }), className)}
      {...props}
    />
  )
}

export {
  RouteTabsLink,
  RouteTabsList,
  routeTabsLinkVariants,
  routeTabsListVariants,
  type RouteTabsOrientation,
  type RouteTabsVariant,
}
