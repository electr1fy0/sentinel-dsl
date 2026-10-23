import * as React from "react"
import {
  FileText,
  Graph,
  TreeStructure,
  Cpu,
  CheckCircle,
  Play,
  Check,
  Sun,
  Moon,
} from "@phosphor-icons/react"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command"
import type { ExampleScenario } from "@/lib/types"

interface CommandPaletteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  activeTab: string
  onSelectTab: (tab: string) => void
  examples: ExampleScenario[]
  onSelectExample: (example: ExampleScenario) => void
  onValidate: () => void
  onEvaluate: () => void
  darkMode: boolean
  onToggleDarkMode: () => void
}

export function CommandPalette({
  open,
  onOpenChange,
  onSelectTab,
  examples,
  onSelectExample,
  onValidate,
  onEvaluate,
  darkMode,
  onToggleDarkMode,
}: CommandPaletteProps) {
  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      const isEditable =
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA" ||
        target?.isContentEditable

      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        onOpenChange(!open)
        return
      }

      if (!(e.metaKey || e.ctrlKey) || e.shiftKey || e.altKey) return

      const tabByShortcut: Record<string, string> = {
        "1": "policy",
        "2": "tests",
        "3": "compiler",
        "4": "automata",
        "5": "theory",
      }
      if (tabByShortcut[e.key]) {
        e.preventDefault()
        onSelectTab(tabByShortcut[e.key])
        onOpenChange(false)
        return
      }

      // Keep normal paste behavior in the editor. Outside an editable field,
      // the advertised Cmd/Ctrl+V shortcut validates the current policy.
      if (e.key.toLowerCase() === "v" && !isEditable) {
        e.preventDefault()
        onValidate()
        return
      }

      if (e.key === "Enter" && !isEditable) {
        e.preventDefault()
        onEvaluate()
      }
    }
    document.addEventListener("keydown", down)
    return () => document.removeEventListener("keydown", down)
  }, [open, onOpenChange])

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="Search commands, views, examples..." />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>

        <CommandGroup heading="Workspace">
          <CommandItem
            onSelect={() => {
              onSelectTab("policy")
              onOpenChange(false)
            }}
          >
            <FileText className="mr-2 size-3.5 text-muted-foreground" />
            <span>Policy</span>
            <CommandShortcut>⌘1</CommandShortcut>
          </CommandItem>
          <CommandItem
            onSelect={() => {
              onSelectTab("tests")
              onOpenChange(false)
            }}
          >
            <CheckCircle className="mr-2 size-3.5 text-muted-foreground" />
            <span>Tests</span>
            <CommandShortcut>⌘2</CommandShortcut>
          </CommandItem>
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Learn">
          <CommandItem
            onSelect={() => {
              onSelectTab("compiler")
              onOpenChange(false)
            }}
          >
            <Cpu className="mr-2 size-3.5 text-muted-foreground" />
            <span>Compiler</span>
            <CommandShortcut>⌘3</CommandShortcut>
          </CommandItem>
          <CommandItem
            onSelect={() => {
              onSelectTab("automata")
              onOpenChange(false)
            }}
          >
            <Graph className="mr-2 size-3.5 text-muted-foreground" />
            <span>Automata</span>
            <CommandShortcut>⌘4</CommandShortcut>
          </CommandItem>
          <CommandItem
            onSelect={() => {
              onSelectTab("theory")
              onOpenChange(false)
            }}
          >
            <TreeStructure className="mr-2 size-3.5 text-muted-foreground" />
            <span>Theory</span>
            <CommandShortcut>⌘5</CommandShortcut>
          </CommandItem>
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Actions">
          <CommandItem
            onSelect={() => {
              onValidate()
              onOpenChange(false)
            }}
          >
            <Check className="mr-2 size-3.5 text-muted-foreground" />
            <span>Check policy</span>
            <CommandShortcut>⌘V</CommandShortcut>
          </CommandItem>
          <CommandItem
            onSelect={() => {
              onEvaluate()
              onOpenChange(false)
            }}
          >
            <Play className="mr-2 size-3.5 text-muted-foreground" />
            <span>Run policy</span>
            <CommandShortcut>⌘Enter</CommandShortcut>
          </CommandItem>
          <CommandItem
            onSelect={() => {
              onToggleDarkMode()
              onOpenChange(false)
            }}
          >
            {darkMode ? (
              <Sun className="mr-2 size-3.5 text-muted-foreground" />
            ) : (
              <Moon className="mr-2 size-3.5 text-muted-foreground" />
            )}
            <span>Toggle {darkMode ? "Light" : "Dark"} Mode</span>
          </CommandItem>
        </CommandGroup>

        <CommandSeparator />

        <CommandGroup heading="Examples">
          {examples.map((ex) => (
            <CommandItem
              key={ex.id}
              onSelect={() => {
                onSelectExample(ex)
                onOpenChange(false)
              }}
            >
              <FileText className="mr-2 size-3.5 text-muted-foreground" />
              <span>{ex.name}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
