import {
  ArrowClockwise,
  ArrowSquareOut,
  Keyboard,
  MagnifyingGlass,
  Moon,
  Question,
  Sun,
} from "@phosphor-icons/react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

interface HeaderProps {
  isBackendConnected: boolean
  backendChecking?: boolean
  backendVersion?: string
  backendError?: string
  onRetryHealth: () => void
  currentExampleName?: string
  onOpenCommandPalette: () => void
  darkMode: boolean
  onToggleDarkMode: () => void
}

export function Header({
  isBackendConnected,
  backendChecking = false,
  backendVersion,
  backendError,
  onRetryHealth,
  currentExampleName,
  onOpenCommandPalette,
  darkMode,
  onToggleDarkMode,
}: HeaderProps) {
  return (
    <TooltipProvider>
      <header className="flex h-14 w-full shrink-0 items-center justify-between overflow-hidden rounded-2xl bg-background/95 px-4 backdrop-blur sm:px-5">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-[15px] font-semibold tracking-tight text-foreground">
              Sentinel
            </span>
            <span className="text-xs text-muted-foreground">Studio</span>
          </div>

          <div className="hidden h-5 w-2 rounded-full bg-muted sm:block" />

          <div className="min-w-0 truncate text-xs text-muted-foreground">
            <span className="sr-only">Current policy: </span>
            {currentExampleName || "Untitled policy"}
          </div>
        </div>

        <div className="flex items-center gap-1.5 sm:gap-2">
          <Tooltip>
            <TooltipTrigger
              render={
                <button
                  type="button"
                  onClick={
                    !isBackendConnected && !backendChecking
                      ? onRetryHealth
                      : undefined
                  }
                  className="flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label={
                    backendChecking
                      ? "Connecting to the Sentinel API"
                      : isBackendConnected
                        ? `Sentinel API connected${backendVersion ? `, version ${backendVersion}` : ""}`
                        : "Sentinel API unavailable. Retry connection"
                  }
                >
                  <span
                    className={`size-1.5 rounded-full ${backendChecking ? "bg-amber-500" : isBackendConnected ? "bg-emerald-500" : "bg-muted-foreground/50"}`}
                  />
                  <span className="hidden sm:inline">
                    {backendChecking
                      ? "Connecting"
                      : isBackendConnected
                        ? "Connected"
                        : "Offline"}
                  </span>
                  {isBackendConnected && backendVersion && (
                    <span className="hidden font-mono text-[10px] text-muted-foreground/70 lg:inline">
                      v{backendVersion}
                    </span>
                  )}
                  {!isBackendConnected && !backendChecking && (
                    <ArrowClockwise className="size-3" aria-hidden="true" />
                  )}
                </button>
              }
            />
            <TooltipContent side="bottom">
              {backendChecking
                ? "Connecting to the Sentinel API…"
                : backendError
                  ? `API error: ${backendError}`
                  : isBackendConnected
                    ? "Connected to the Sentinel API."
                    : "The Sentinel API is unavailable. Click to retry."}
            </TooltipContent>
          </Tooltip>

          <Button
            variant="outline"
            size="sm"
            onClick={onOpenCommandPalette}
            className="hidden h-8 gap-2 bg-muted/30 px-2.5 text-xs font-normal text-muted-foreground hover:bg-muted hover:text-foreground sm:flex"
          >
            <MagnifyingGlass className="size-3.5" />
            <span>Search</span>
            <kbd className="rounded bg-background/70 px-1 py-0.5 font-mono text-[9px] text-muted-foreground">
              ⌘K
            </kbd>
          </Button>

          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground hover:text-foreground sm:hidden"
            onClick={onOpenCommandPalette}
            aria-label="Search commands"
          >
            <MagnifyingGlass className="size-3.5" />
          </Button>

          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground hover:text-foreground"
            onClick={onToggleDarkMode}
            aria-label="Toggle theme"
          >
            {darkMode ? (
              <Sun className="size-3.5" />
            ) : (
              <Moon className="size-3.5" />
            )}
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground hover:text-foreground"
                  aria-label="Open help menu"
                >
                  <Question className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuGroup>
                <DropdownMenuLabel>Help</DropdownMenuLabel>
                <DropdownMenuItem
                  onClick={() =>
                    window.open(
                      "https://github.com/ayush/sentinel-dsl",
                      "_blank"
                    )
                  }
                >
                  <ArrowSquareOut className="size-3.5" />
                  Documentation
                </DropdownMenuItem>
                <DropdownMenuItem onClick={onOpenCommandPalette}>
                  <Keyboard className="size-3.5" />
                  Keyboard shortcuts
                  <span className="ml-auto font-mono text-[10px] text-muted-foreground">
                    ⌘K
                  </span>
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuLabel className="font-normal text-muted-foreground/70">
                  Sentinel Studio
                </DropdownMenuLabel>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>
    </TooltipProvider>
  )
}
