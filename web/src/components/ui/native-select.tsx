import * as React from "react"
import { ChevronDownIcon } from "lucide-react"

import { cn } from "@/lib/utils"

type NativeSelectProps = Omit<React.ComponentProps<"select">, "size"> & {
  size?: "sm" | "default"
  wrapperClassName?: string
}

function NativeSelect({
  className,
  wrapperClassName,
  size = "default",
  multiple,
  ...props
}: NativeSelectProps) {
  return (
    <div
      className={cn("group/native-select relative w-full has-[select:disabled]:opacity-50", wrapperClassName)}
      data-slot="native-select-wrapper"
      data-size={size}
    >
      <select
        data-slot="native-select"
        data-size={size}
        multiple={multiple}
        className={cn(
          "h-8 w-full min-w-0 appearance-none rounded-md border border-input bg-surface py-1 pr-8 pl-2.5 text-sm transition-[border-color,box-shadow,background-color] duration-100 outline-none selection:bg-primary selection:text-primary-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/35 disabled:pointer-events-none disabled:cursor-not-allowed disabled:border-border-subtle disabled:bg-surface-subtle disabled:text-text-disabled aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20 data-[size=sm]:h-7 data-[size=sm]:rounded-md data-[size=sm]:py-0.5",
          multiple&&"h-auto min-h-24 appearance-auto py-2 pr-2",
          className
        )}
        {...props}
      />
      {!multiple&&<ChevronDownIcon className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-muted-foreground select-none" aria-hidden="true" data-slot="native-select-icon" />}
    </div>
  )
}

function NativeSelectOption({className,...props}:React.ComponentProps<"option">){
  return <option data-slot="native-select-option" className={cn("bg-[Canvas] text-[CanvasText]",className)} {...props}/>
}

function NativeSelectOptGroup({className,...props}:React.ComponentProps<"optgroup">){
  return <optgroup data-slot="native-select-optgroup" className={cn("bg-[Canvas] text-[CanvasText]",className)} {...props}/>
}

export { NativeSelect, NativeSelectOptGroup, NativeSelectOption }
