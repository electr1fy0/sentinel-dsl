import * as React from "react"

import { cn } from "@/lib/utils"

const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.ComponentProps<"textarea">
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    data-slot="textarea"
    className={cn(
      "flex field-sizing-content min-h-16 w-full rounded-lg bg-input/70 px-2.5 py-2 text-base transition-colors outline-none placeholder:text-muted-foreground focus-visible:bg-accent focus-visible:ring-3 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:bg-destructive/10 aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/80 dark:disabled:bg-input/80 dark:aria-invalid:bg-destructive/15 dark:aria-invalid:ring-destructive/40",
      className
    )}
    {...props}
  />
))
Textarea.displayName = "Textarea"

export { Textarea }
