import * as React from "react"
import { HeadContent, Scripts, createRootRoute } from "@tanstack/react-router"
import type { ErrorComponentProps } from "@tanstack/react-router"
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools"
import { TanStackDevtools } from "@tanstack/react-devtools"
import { ArrowsClockwise, WarningCircle } from "@phosphor-icons/react"

import appCss from "../styles.css?url"

export const Route = createRootRoute({
  head: () => ({
    meta: [
      {
        charSet: "utf-8",
      },
      {
        name: "viewport",
        content: "width=device-width, initial-scale=1",
      },
      {
        title: "Sentinel Studio",
      },
    ],
    links: [
      {
        rel: "stylesheet",
        href: appCss,
      },
    ],
  }),
  errorComponent: RootErrorComponent,
  notFoundComponent: () => (
    <main className="container mx-auto p-4 pt-16">
      <h1>404</h1>
      <p>The requested page could not be found.</p>
    </main>
  ),
  shellComponent: RootDocument,
})

function isModuleImportError(error: unknown): boolean {
  if (!error) return false
  const msg =
    typeof error === "string"
      ? error
      : typeof error === "object" &&
          "message" in error &&
          typeof (error as any).message === "string"
        ? (error as any).message
        : String(error)

  const lower = msg.toLowerCase()
  return (
    lower.includes("importing a module script failed") ||
    lower.includes("failed to fetch dynamically imported module") ||
    lower.includes("error loading dynamically imported module") ||
    lower.includes("failed to load module script") ||
    lower.includes("unable to preload css")
  )
}

function RootErrorComponent({ error, reset }: ErrorComponentProps) {
  const isChunkError = isModuleImportError(error)

  React.useEffect(() => {
    if (typeof window === "undefined") return
    if (isChunkError) {
      const lastReload = sessionStorage.getItem("sentinel_chunk_reload_ts")
      const now = Date.now()
      // Auto-reload once within a 15-second window
      if (!lastReload || now - Number(lastReload) > 15000) {
        sessionStorage.setItem("sentinel_chunk_reload_ts", String(now))
        window.location.reload()
      }
    }
  }, [isChunkError])

  const errorMessage =
    error instanceof Error
      ? error.message
      : typeof error === "string"
        ? error
        : "An unexpected runtime error occurred."

  return (
    <main className="flex min-h-screen flex-col items-center justify-center bg-background p-6 text-foreground">
      <div className="w-full max-w-lg space-y-5 rounded-2xl bg-card p-6">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-lg bg-destructive/15 text-destructive">
            <WarningCircle size={24} weight="bold" />
          </div>
          <div>
            <h1 className="text-lg font-semibold tracking-tight text-foreground">
              {isChunkError
                ? "Application Update Detected"
                : "Something went wrong"}
            </h1>
            <p className="text-xs text-muted-foreground">
              {isChunkError
                ? "The application code was updated or a script chunk failed to load."
                : "An unexpected error occurred during rendering."}
            </p>
          </div>
        </div>

        <div className="rounded-lg bg-muted/70 p-3 font-mono text-xs break-words text-muted-foreground">
          {errorMessage}
        </div>

        <div className="flex items-center justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={() => reset()}
            className="inline-flex h-9 cursor-pointer items-center justify-center rounded-lg bg-muted px-4 py-2 text-xs font-medium text-foreground transition-colors hover:bg-accent"
          >
            Try again
          </button>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="inline-flex h-9 cursor-pointer items-center justify-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          >
            <ArrowsClockwise size={14} weight="bold" />
            Reload Application
          </button>
        </div>
      </div>
    </main>
  )
}

function RootDocument({ children }: { children: React.ReactNode }) {
  React.useEffect(() => {
    if (typeof window === "undefined") return
    const handlePreloadError = () => {
      const lastReload = sessionStorage.getItem("sentinel_chunk_reload_ts")
      const now = Date.now()
      if (!lastReload || now - Number(lastReload) > 15000) {
        sessionStorage.setItem("sentinel_chunk_reload_ts", String(now))
        window.location.reload()
      }
    }
    window.addEventListener("vite:preloadError", handlePreloadError)
    return () =>
      window.removeEventListener("vite:preloadError", handlePreloadError)
  }, [])

  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        {import.meta.env.DEV && (
          <TanStackDevtools
            config={{
              position: "bottom-right",
            }}
            plugins={[
              {
                name: "Tanstack Router",
                render: <TanStackRouterDevtoolsPanel />,
              },
            ]}
          />
        )}
        <Scripts />
      </body>
    </html>
  )
}
