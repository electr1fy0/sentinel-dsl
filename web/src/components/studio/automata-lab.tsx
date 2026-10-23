import * as React from "react"
import {
  DotsThree,
  Play,
  Pause,
  CaretRight,
  CaretLeft,
  ArrowClockwise,
  ArrowRight,
  DownloadSimple,
  Minus,
  Plus,
} from "@phosphor-icons/react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { AutomataResponse, GraphNode, GraphResponse } from "@/lib/types"

interface AutomataLabProps {
  pattern: string
  onPatternChange: (p: string) => void
  input: string
  onInputChange: (i: string) => void
  data: AutomataResponse | null
  loading: boolean
  onAnalyze: () => void
}

const REGEX_PRESETS = [
  {
    name: "Path ID",
    pattern: "/api/users/[0-9]+",
    input: "/api/users/42",
  },
  {
    name: "API Versions",
    pattern: "/v(1|2)/(users|teams)/[a-z]+",
    input: "/v1/users/alice",
  },
  { name: "Alternation", pattern: "a(b|c)*d+", input: "abbccd" },
  {
    name: "Actions",
    pattern: "(read|write)_(item|user)",
    input: "read_item",
  },
  {
    name: "HTTPS Url",
    pattern: "https?://[a-z]+\\.sentinel",
    input: "https://api.sentinel",
  },
]

function downloadFile(filename: string, content: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function graphToDot(graph: GraphResponse): string {
  const lines = [
    "digraph Automaton {",
    "  rankdir=LR;",
    "  __start [shape=point];",
  ]
  graph.nodes.forEach((node) => {
    const shape = node.accept ? "doublecircle" : "circle"
    lines.push(
      `  ${JSON.stringify(node.id)} [label=${JSON.stringify(node.label)}, shape=${shape}];`
    )
    if (node.start) lines.push(`  __start -> ${JSON.stringify(node.id)};`)
  })
  graph.edges.forEach((edge) =>
    lines.push(
      `  ${JSON.stringify(edge.from)} -> ${JSON.stringify(edge.to)} [label=${JSON.stringify(edge.label)}];`
    )
  )
  lines.push("}")
  return lines.join("\n")
}

function graphToSvg(graph: GraphResponse): string {
  const width = 600
  const height = 480
  const radius = Math.min(220, Math.max(140, graph.nodes.length * 28))
  const points = graph.nodes.map((node, index) => ({
    node,
    x:
      300 +
      radius *
        Math.cos((2 * Math.PI * index) / graph.nodes.length - Math.PI / 2),
    y:
      240 +
      radius *
        Math.sin((2 * Math.PI * index) / graph.nodes.length - Math.PI / 2),
  }))
  const lookup = new Map(points.map((point) => [point.node.id, point]))
  const edges = graph.edges
    .map((edge) => {
      const from = lookup.get(edge.from)
      const to = lookup.get(edge.to)
      if (!from || !to) return ""
      return `<line x1="${from.x}" y1="${from.y}" x2="${to.x}" y2="${to.y}" stroke="#94a3b8" marker-end="url(#arrow)"/><text x="${(from.x + to.x) / 2}" y="${(from.y + to.y) / 2 - 6}" font-family="monospace" font-size="11" text-anchor="middle">${escapeXml(edge.label)}</text>`
    })
    .join("")
  const nodes = points
    .map(
      ({ node, x, y }) =>
        `${node.start ? `<line x1="${x - 46}" y1="${y}" x2="${x - 25}" y2="${y}" stroke="#64748b" marker-end="url(#arrow)"/>` : ""}<circle cx="${x}" cy="${y}" r="22" fill="white" stroke="#64748b"/>${node.accept ? `<circle cx="${x}" cy="${y}" r="26" fill="none" stroke="#64748b"/>` : ""}<text x="${x}" y="${y + 4}" font-family="monospace" font-size="12" text-anchor="middle">${escapeXml(node.label)}</text>`
    )
    .join("")
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}"><defs><marker id="arrow" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto"><path d="M0,0 L8,3 L0,6 Z" fill="#64748b"/></marker></defs>${edges}${nodes}</svg>`
}

function escapeXml(value: string): string {
  return value.replace(
    /[<>&'"]/g,
    (char) =>
      ({
        "<": "&lt;",
        ">": "&gt;",
        "&": "&amp;",
        "'": "&apos;",
        '"': "&quot;",
      })[char] ?? char
  )
}

function graphToCsv(graph: GraphResponse): string {
  const symbols = graph.alphabet
  const rows = [
    ["state", "kind", ...symbols],
    ...graph.nodes.map((node) => [
      node.label,
      node.start && node.accept
        ? "start/accept"
        : node.start
          ? "start"
          : node.accept
            ? "accept"
            : "normal",
      ...symbols.map(
        (symbol) =>
          graph.edges
            .filter((edge) => edge.from === node.id && edge.label === symbol)
            .map((edge) => edge.to)
            .join(" | ") || ""
      ),
    ]),
  ]
  return rows
    .map((row) =>
      row.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(",")
    )
    .join("\n")
}

function AutomataDiagnostics({ data }: { data: AutomataResponse }) {
  return (
    <div className="bg-destructive/10 px-4 py-2 text-xs" role="alert">
      <div className="font-medium text-destructive">
        Pattern could not be compiled
      </div>
      {data.diagnostics && data.diagnostics.length > 0 ? (
        <ul className="mt-1 list-inside list-disc text-[11px] text-destructive/90">
          {data.diagnostics.map((diagnostic, index) => (
            <li key={`${diagnostic.message}-${index}`}>
              {diagnostic.message}{" "}
              <span className="font-mono opacity-70">
                ({diagnostic.start.line}:{diagnostic.start.column})
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-1 text-[11px] text-muted-foreground">
          Fix the pattern and compile again.
        </p>
      )}
    </div>
  )
}

export function AutomataLab({
  pattern,
  onPatternChange,
  input,
  onInputChange,
  data,
  loading,
  onAnalyze,
}: AutomataLabProps) {
  const [activeTab, setActiveTab] = React.useState<
    "minimized" | "dfa" | "nfa" | "table"
  >("minimized")
  const [currentStep, setCurrentStep] = React.useState(0)
  const [isPlaying, setIsPlaying] = React.useState(false)
  const [playSpeed, setPlaySpeed] = React.useState(800)

  const simSteps =
    activeTab === "nfa" ? data?.nfa_sim_steps : data?.dfa_sim_steps
  const maxSteps = (simSteps?.length ?? 1) - 1

  React.useEffect(() => {
    setCurrentStep(0)
    setIsPlaying(false)
  }, [pattern, input, activeTab])

  React.useEffect(() => {
    if (!isPlaying) return
    const timer = setInterval(() => {
      setCurrentStep((prev) => {
        if (prev >= maxSteps) {
          setIsPlaying(false)
          return prev
        }
        return prev + 1
      })
    }, playSpeed)
    return () => clearInterval(timer)
  }, [isPlaying, maxSteps, playSpeed])

  // Arrow keys make the simulator useful for keyboard-driven walkthroughs.
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.matches("input, textarea, select, [contenteditable=true]"))
        return
      if (event.key === "ArrowRight") {
        event.preventDefault()
        setCurrentStep((step) => Math.min(maxSteps, step + 1))
      } else if (event.key === "ArrowLeft") {
        event.preventDefault()
        setCurrentStep((step) => Math.max(0, step - 1))
      } else if (event.key === "Home") {
        event.preventDefault()
        setCurrentStep(0)
      } else if (event.key === "End") {
        event.preventDefault()
        setCurrentStep(maxSteps)
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [maxSteps])

  const activeStepData = simSteps ? simSteps[currentStep] : null
  const activeStateNames = activeStepData ? activeStepData.active_states : []
  const previousStateNames =
    currentStep > 0 && simSteps ? simSteps[currentStep - 1].active_states : []
  const displayedGraph =
    activeTab === "minimized"
      ? data?.minimized
      : activeTab === "dfa"
        ? data?.dfa
        : data?.nfa

  const nfaStates = data?.nfa?.states ?? 0
  const dfaStates = data?.dfa?.states ?? 0
  const minStates = data?.minimized?.states ?? 0
  const reduction =
    dfaStates > 0 ? Math.round(((dfaStates - minStates) / dfaStates) * 100) : 0

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden bg-background">
      {/* One input, one action. Presets and exports stay available in More. */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/30 px-4 py-3">
        <div className="min-w-44">
          <h1 className="text-sm font-medium text-foreground">Automata</h1>
          <p className="text-[11px] text-muted-foreground">
            Compile a regular expression and inspect its state machine
          </p>
        </div>

        <div className="flex min-w-0 flex-1 items-center justify-end gap-1.5 sm:max-w-2xl">
          <Input
            aria-label="Regular expression pattern"
            value={pattern}
            onChange={(e) => onPatternChange(e.target.value)}
            className="h-8 min-w-0 flex-1 bg-background font-mono text-xs"
            placeholder="e.g. a(b|c)*d"
          />
          <Button
            size="sm"
            onClick={onAnalyze}
            disabled={loading}
            className="h-8 gap-1.5 px-3 text-xs font-medium"
          >
            <Play weight="fill" className="size-2.5" />
            <span>{loading ? "Compiling…" : "Compile"}</span>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground hover:text-foreground"
                  aria-label="More automata actions"
                >
                  <DotsThree className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem
                disabled={!displayedGraph}
                onClick={() =>
                  displayedGraph &&
                  downloadFile(
                    "automaton.dot",
                    graphToDot(displayedGraph),
                    "text/vnd.graphviz"
                  )
                }
              >
                Export DOT
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!displayedGraph}
                onClick={() =>
                  displayedGraph &&
                  downloadFile(
                    "automaton.svg",
                    graphToSvg(displayedGraph),
                    "image/svg+xml"
                  )
                }
              >
                <DownloadSimple className="size-3.5" /> Export SVG
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!data?.minimized}
                onClick={() =>
                  data?.minimized &&
                  downloadFile(
                    "automaton-transitions.csv",
                    graphToCsv(data.minimized),
                    "text/csv"
                  )
                }
              >
                Export transition table
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {REGEX_PRESETS.map((preset) => (
                <DropdownMenuItem
                  key={preset.name}
                  onClick={() => {
                    onPatternChange(preset.pattern)
                    onInputChange(preset.input)
                  }}
                >
                  Use {preset.name} preset
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {data && !data.valid && <AutomataDiagnostics data={data} />}

      {/* Main Content Area */}
      <div className="grid flex-1 grid-cols-1 gap-2 overflow-hidden bg-muted/55 p-2 lg:grid-cols-[1fr_340px]">
        {/* Left: Graphs & Minimization */}
        <div className="flex flex-col overflow-hidden rounded-xl bg-background">
          {/* Refined Inline Metrics Bar */}
          <div className="flex flex-wrap items-center justify-between gap-2 bg-muted/45 px-4 py-2.5 font-mono text-[11px] text-muted-foreground">
            <div className="flex items-center gap-4">
              <span>
                NFA:{" "}
                <strong className="text-foreground">
                  {data?.valid ? nfaStates : "—"}
                </strong>{" "}
                states
              </span>
              <span>
                DFA:{" "}
                <strong className="text-foreground">
                  {data?.valid ? dfaStates : "—"}
                </strong>{" "}
                states
              </span>
              <span>
                Minimized:{" "}
                <strong className="text-foreground">
                  {data?.valid ? minStates : "—"}
                </strong>{" "}
                states
              </span>
            </div>

            {reduction > 0 && (
              <span className="font-medium text-emerald-600 dark:text-emerald-400">
                -{reduction}% states reduced
              </span>
            )}
          </div>

          {/* Graph Visualization Area */}
          <div className="flex flex-1 flex-col overflow-hidden">
            <div className="flex h-8 items-center justify-between bg-muted/60 px-3">
              <Tabs
                value={activeTab}
                onValueChange={(val) => {
                  if (
                    val === "minimized" ||
                    val === "dfa" ||
                    val === "nfa" ||
                    val === "table"
                  ) {
                    setActiveTab(val)
                  }
                }}
                className="w-auto"
              >
                <TabsList className="h-7 gap-1 bg-transparent p-0">
                  <TabsTrigger
                    value="minimized"
                    className="h-7 rounded-md px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                  >
                    Minimized
                  </TabsTrigger>
                  <TabsTrigger
                    value="dfa"
                    className="h-7 rounded-md px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                  >
                    DFA
                  </TabsTrigger>
                  <TabsTrigger
                    value="nfa"
                    className="h-7 rounded-md px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                  >
                    NFA
                  </TabsTrigger>
                  <TabsTrigger
                    value="table"
                    className="h-7 rounded-md px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                  >
                    Table
                  </TabsTrigger>
                </TabsList>
              </Tabs>

              <span className="font-mono text-[11px] text-muted-foreground">
                alphabet: [
                {data?.valid ? data.minimized?.alphabet.join(", ") || "-" : "—"}
                ]
              </span>
            </div>

            <div className="relative flex-1 overflow-hidden bg-background">
              {activeTab === "table" ? (
                <ScrollArea className="h-full p-4">
                  {data?.minimized && (
                    <TransitionTableView graph={data.minimized} />
                  )}
                </ScrollArea>
              ) : (
                <AutomataGraphViewer
                  graph={
                    activeTab === "minimized"
                      ? data?.minimized
                      : activeTab === "dfa"
                        ? data?.dfa
                        : data?.nfa
                  }
                  activeStates={activeStateNames}
                  previousActiveStates={previousStateNames}
                  activeSymbol={activeStepData?.symbol}
                />
              )}
            </div>
          </div>
        </div>

        {/* Right: Step-by-Step Input Simulator */}
        <div className="flex flex-col overflow-hidden rounded-xl bg-background">
          <div className="flex h-9 items-center justify-between bg-muted/60 px-3.5">
            <span className="text-xs font-medium text-foreground">
              Simulator
            </span>
            {data?.valid && (
              <div className="flex items-center gap-1.5">
                <span
                  className={`size-1.5 rounded-full ${
                    data.accepted ? "bg-emerald-500" : "bg-destructive"
                  }`}
                />
                <span
                  className={`font-mono text-[11px] font-medium ${
                    data.accepted ? "text-emerald-500" : "text-destructive"
                  }`}
                >
                  {data.accepted ? "Accepted" : "Rejected"}
                </span>
              </div>
            )}
          </div>

          <ScrollArea className="flex-1 p-3">
            <div className="flex flex-col gap-3">
              {/* Input String Field */}
              <div className="flex flex-col gap-1">
                <Label className="text-xs text-muted-foreground">
                  Input string
                </Label>
                <div className="flex gap-2">
                  <Input
                    aria-label="Automaton test string"
                    value={input}
                    onChange={(e) => onInputChange(e.target.value)}
                    className="h-7 font-mono text-xs"
                    placeholder="Input string to simulate..."
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={onAnalyze}
                    className="h-7 px-2.5 text-xs font-normal"
                  >
                    Run
                  </Button>
                </div>
              </div>

              {/* String Tape Ribbon with Character Cursor */}
              <div className="rounded-xl bg-muted/55 p-2.5">
                <div className="mb-1.5 flex items-center justify-between font-mono text-[10px] text-muted-foreground">
                  <span>
                    Tape Position: {currentStep} / {input.length}
                  </span>
                </div>
                <div className="flex items-center gap-1 overflow-x-auto py-1">
                  <div
                    className={`flex h-7 items-center justify-center rounded-lg px-2 font-mono text-xs transition-all ${
                      currentStep === 0
                        ? "bg-primary font-medium text-primary-foreground"
                        : "bg-card text-muted-foreground"
                    }`}
                  >
                    START
                  </div>
                  {input.split("").map((char, i) => {
                    const isActive = currentStep === i + 1
                    return (
                      <div
                        key={i}
                        className={`flex size-7 items-center justify-center rounded-lg font-mono text-xs font-medium transition-all ${
                          isActive
                            ? "bg-emerald-500 text-white"
                            : i < currentStep
                              ? "bg-muted/70 text-foreground"
                              : "bg-background text-muted-foreground"
                        }`}
                      >
                        {char}
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Playback Controls */}
              <div className="flex items-center justify-between rounded-xl bg-muted/35 p-2">
                <div className="flex items-center gap-1">
                  <Button
                    aria-label="Reset simulator to start"
                    variant="ghost"
                    size="icon"
                    className="size-6 text-muted-foreground hover:text-foreground"
                    disabled={currentStep === 0}
                    onClick={() => setCurrentStep(0)}
                  >
                    <ArrowClockwise className="size-3" />
                  </Button>
                  <Button
                    aria-label="Step simulator backward"
                    variant="ghost"
                    size="icon"
                    className="size-6 text-muted-foreground hover:text-foreground"
                    disabled={currentStep === 0}
                    onClick={() => setCurrentStep((p) => Math.max(0, p - 1))}
                  >
                    <CaretLeft className="size-3" />
                  </Button>
                  <Button
                    aria-label={
                      isPlaying ? "Pause simulator" : "Play simulator"
                    }
                    variant="ghost"
                    size="icon"
                    className="size-6 text-foreground"
                    onClick={() => setIsPlaying(!isPlaying)}
                  >
                    {isPlaying ? (
                      <Pause className="size-3" />
                    ) : (
                      <Play weight="fill" className="size-3" />
                    )}
                  </Button>
                  <Button
                    aria-label="Step simulator forward"
                    variant="ghost"
                    size="icon"
                    className="size-6 text-muted-foreground hover:text-foreground"
                    disabled={currentStep >= maxSteps}
                    onClick={() =>
                      setCurrentStep((p) => Math.min(maxSteps, p + 1))
                    }
                  >
                    <CaretRight className="size-3" />
                  </Button>
                </div>

                <div className="flex items-center gap-1">
                  {[
                    { label: "1x", ms: 800 },
                    { label: "2x", ms: 400 },
                  ].map((s) => (
                    <Button
                      key={s.label}
                      variant={playSpeed === s.ms ? "secondary" : "ghost"}
                      size="sm"
                      className="h-5 px-1.5 font-mono text-[10px]"
                      onClick={() => setPlaySpeed(s.ms)}
                    >
                      {s.label}
                    </Button>
                  ))}
                </div>
              </div>

              {/* Active Step State Card */}
              {activeStepData && (
                <div className="rounded-xl bg-card p-2.5 text-xs">
                  <div className="mb-1 flex items-center justify-between">
                    <span className="font-medium text-foreground">
                      Step {activeStepData.step_index}: &apos;
                      {activeStepData.symbol}&apos;
                    </span>
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {activeStepData.active_states.length} active
                    </span>
                  </div>
                  <p className="mb-2 font-mono text-[11px] text-muted-foreground">
                    {activeStepData.description}
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {activeStepData.active_states.map((s, idx) => (
                      <span
                        key={idx}
                        className="rounded bg-muted/60 px-1.5 py-0.5 font-mono text-[11px] text-foreground"
                      >
                        {s}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Step Transition Log */}
              {simSteps && (
                <div className="flex flex-col gap-1">
                  <span className="text-xs font-medium text-foreground">
                    Step log
                  </span>
                  <div className="flex flex-col gap-1">
                    {simSteps.map((step, idx) => (
                      <div
                        key={idx}
                        onClick={() => setCurrentStep(idx)}
                        onKeyDown={(event) => {
                          if (event.key === "Enter" || event.key === " ")
                            setCurrentStep(idx)
                        }}
                        role="button"
                        tabIndex={0}
                        className={`flex cursor-pointer items-center justify-between rounded-lg p-1.5 font-mono text-xs transition-all ${
                          currentStep === idx
                            ? "bg-muted/70 font-medium text-foreground"
                            : "bg-card text-muted-foreground hover:bg-muted/40"
                        }`}
                      >
                        <div className="flex items-center gap-1.5">
                          <span className="text-[10px] opacity-60">
                            #{step.step_index}
                          </span>
                          <span>[{step.symbol}]</span>
                          <ArrowRight className="size-2.5 opacity-40" />
                          <span className="max-w-[130px] truncate">
                            {step.active_states.join(", ")}
                          </span>
                        </div>
                        {currentStep === idx && (
                          <span className="size-1.5 rounded-full bg-foreground" />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </ScrollArea>
        </div>
      </div>
    </div>
  )
}

function AutomataGraphViewer({
  graph,
  activeStates,
  previousActiveStates,
  activeSymbol,
}: {
  graph?: GraphResponse
  activeStates: string[]
  previousActiveStates: string[]
  activeSymbol?: string
}) {
  const [zoom, setZoom] = React.useState(1)
  const [pan, setPan] = React.useState({ x: 0, y: 0 })
  const drag = React.useRef<{
    x: number
    y: number
    panX: number
    panY: number
  } | null>(null)

  React.useEffect(() => {
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }, [graph])

  if (!graph || graph.nodes.length === 0) {
    return (
      <div className="flex h-full items-center justify-center px-6 text-center">
        <div>
          <p className="text-sm font-medium text-foreground">
            Compile the pattern to see the automaton
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            The graph and transition states will appear here.
          </p>
        </div>
      </div>
    )
  }

  const nodeCount = graph.nodes.length
  const radius = Math.min(220, Math.max(140, nodeCount * 28))
  const centerX = 300
  const centerY = 240

  const positionedNodes: (GraphNode & { cx: number; cy: number })[] =
    graph.nodes.map((node, i) => {
      const angle = (2 * Math.PI * i) / nodeCount - Math.PI / 2
      return {
        ...node,
        cx: centerX + radius * Math.cos(angle),
        cy: centerY + radius * Math.sin(angle),
      }
    })

  const nodeMap = new Map(positionedNodes.map((n) => [n.id, n]))

  const matchesState = (states: string[], node: GraphNode) =>
    states.includes(node.id) ||
    states.includes(node.label) ||
    states.includes(`q${node.id}`)
  return (
    <div
      className="relative flex size-full items-center justify-center overflow-hidden p-4"
      onPointerMove={(event) => {
        if (!drag.current) return
        setPan({
          x: drag.current.panX + event.clientX - drag.current.x,
          y: drag.current.panY + event.clientY - drag.current.y,
        })
      }}
      onPointerUp={() => {
        drag.current = null
      }}
      onPointerLeave={() => {
        drag.current = null
      }}
    >
      <div className="absolute top-3 right-3 z-10 flex items-center gap-1 rounded-xl bg-muted/80 p-1">
        <Button
          aria-label="Zoom out graph"
          variant="ghost"
          size="icon"
          className="size-6"
          onClick={() =>
            setZoom((value) =>
              Math.max(0.5, Math.round((value - 0.1) * 10) / 10)
            )
          }
        >
          <Minus className="size-3" />
        </Button>
        <span className="w-10 text-center font-mono text-[10px] text-muted-foreground">
          {Math.round(zoom * 100)}%
        </span>
        <Button
          aria-label="Zoom in graph"
          variant="ghost"
          size="icon"
          className="size-6"
          onClick={() =>
            setZoom((value) => Math.min(2, Math.round((value + 0.1) * 10) / 10))
          }
        >
          <Plus className="size-3" />
        </Button>
        <Button
          aria-label="Fit graph to view"
          variant="ghost"
          size="sm"
          className="h-6 px-1.5 text-[10px]"
          onClick={() => {
            setZoom(1)
            setPan({ x: 0, y: 0 })
          }}
        >
          Fit
        </Button>
      </div>
      <svg
        viewBox="0 0 600 480"
        className="h-auto w-full max-w-[650px] cursor-grab select-none active:cursor-grabbing"
        role="img"
        aria-label="Automaton state graph; drag to pan"
        onPointerDown={(event) => {
          drag.current = {
            x: event.clientX,
            y: event.clientY,
            panX: pan.x,
            panY: pan.y,
          }
          event.currentTarget.setPointerCapture(event.pointerId)
        }}
      >
        <defs>
          <marker
            id="arrowhead"
            markerWidth="8"
            markerHeight="6"
            refX="7"
            refY="3"
            orient="auto"
          >
            <polygon
              points="0 0, 8 3, 0 6"
              className="fill-muted-foreground/70"
            />
          </marker>
        </defs>

        <g transform={`translate(${pan.x} ${pan.y}) scale(${zoom})`}>
          {/* Edges */}
          {graph.edges.map((edge, i) => {
            const fromNode = nodeMap.get(edge.from)
            const toNode = nodeMap.get(edge.to)
            if (!fromNode || !toNode) return null

            const isSelfLoop = edge.from === edge.to
            if (isSelfLoop) {
              const loopRadius = 18
              const path = `M ${fromNode.cx - 10} ${fromNode.cy - 20} A ${loopRadius} ${loopRadius} 0 1 1 ${
                fromNode.cx + 10
              } ${fromNode.cy - 20}`
              return (
                <g key={i}>
                  <path
                    d={path}
                    fill="none"
                    className={
                      activeSymbol !== undefined &&
                      previousActiveStates.some(
                        (state) =>
                          state === edge.from || state === `q${edge.from}`
                      ) &&
                      edge.label === activeSymbol
                        ? "stroke-emerald-500 stroke-[3]"
                        : "stroke-muted-foreground/30 stroke-[1.5]"
                    }
                    markerEnd="url(#arrowhead)"
                  />
                  <text
                    x={fromNode.cx}
                    y={fromNode.cy - 42}
                    textAnchor="middle"
                    className="fill-muted-foreground font-mono text-[11px] font-medium"
                  >
                    {edge.label}
                  </text>
                </g>
              )
            }

            const dx = toNode.cx - fromNode.cx
            const dy = toNode.cy - fromNode.cy
            const dist = Math.hypot(dx, dy)
            const offsetRadius = 24
            const startX = fromNode.cx + (dx / dist) * offsetRadius
            const startY = fromNode.cy + (dy / dist) * offsetRadius
            const endX = toNode.cx - (dx / dist) * offsetRadius
            const endY = toNode.cy - (dy / dist) * offsetRadius

            const midX = (startX + endX) / 2
            const midY = (startY + endY) / 2

            return (
              <g key={i}>
                <line
                  x1={startX}
                  y1={startY}
                  x2={endX}
                  y2={endY}
                  className={
                    activeSymbol !== undefined &&
                    previousActiveStates.some(
                      (state) =>
                        state === edge.from || state === `q${edge.from}`
                    ) &&
                    edge.label === activeSymbol
                      ? "stroke-emerald-500 stroke-[3]"
                      : "stroke-muted-foreground/30 stroke-[1.5]"
                  }
                  markerEnd="url(#arrowhead)"
                />
                <rect
                  x={midX - 10}
                  y={midY - 8}
                  width="20"
                  height="16"
                  rx="3"
                  className="fill-background stroke-muted-foreground/25 stroke-[0.5]"
                />
                <text
                  x={midX}
                  y={midY + 4}
                  textAnchor="middle"
                  className="fill-muted-foreground font-mono text-[11px] font-medium"
                >
                  {edge.label}
                </text>
              </g>
            )
          })}

          {/* Nodes */}
          {positionedNodes.map((node) => {
            const isActive = matchesState(activeStates, node)
            return (
              <g key={node.id} className="cursor-pointer transition-all">
                {/* Start State Indicator Arrow */}
                {node.start && (
                  <path
                    d={`M ${node.cx - 45} ${node.cy} L ${node.cx - 26} ${node.cy}`}
                    className="stroke-muted-foreground stroke-[1.5]"
                    markerEnd="url(#arrowhead)"
                  />
                )}

                {/* Outer Accept Circle */}
                {node.accept && (
                  <circle
                    cx={node.cx}
                    cy={node.cy}
                    r="26"
                    fill="none"
                    className={
                      isActive
                        ? "stroke-emerald-500 stroke-2"
                        : "stroke-muted-foreground/30 stroke-[1.5]"
                    }
                  />
                )}

                {/* Main Node Circle */}
                <circle
                  cx={node.cx}
                  cy={node.cy}
                  r="22"
                  className={`transition-all ${
                    isActive
                      ? "fill-emerald-500 stroke-emerald-600 stroke-2"
                      : "fill-card stroke-muted-foreground/30 stroke-[1.5]"
                  }`}
                />

                {/* Label */}
                <text
                  x={node.cx}
                  y={node.cy + 4}
                  textAnchor="middle"
                  className={`font-mono text-xs font-medium select-none ${
                    isActive ? "fill-white" : "fill-foreground"
                  }`}
                >
                  {node.label}
                </text>
              </g>
            )
          })}
        </g>
      </svg>
    </div>
  )
}

function TransitionTableView({ graph }: { graph: GraphResponse }) {
  return (
    <div className="rounded-xl bg-card">
      <div className="bg-muted/60 px-3 py-2 text-xs font-medium text-foreground">
        Transition Table ({graph.states} States)
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[120px]">State</TableHead>
            <TableHead>Type</TableHead>
            {graph.alphabet.map((sym) => (
              <TableHead key={sym} className="text-center font-mono">
                &apos;{sym}&apos;
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {graph.nodes.map((node) => (
            <TableRow key={node.id}>
              <TableCell className="font-mono font-medium">
                {node.start ? "→ " : ""}
                {node.label}
                {node.accept ? " *" : ""}
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {node.start && node.accept
                  ? "Start / Accept"
                  : node.start
                    ? "Start"
                    : node.accept
                      ? "Accept"
                      : "Normal"}
              </TableCell>
              {graph.alphabet.map((sym) => {
                const edge = graph.edges.find(
                  (e) => e.from === node.id && e.label === sym
                )
                return (
                  <TableCell
                    key={sym}
                    className="text-center font-mono text-xs text-muted-foreground"
                  >
                    {edge ? edge.to : "—"}
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
