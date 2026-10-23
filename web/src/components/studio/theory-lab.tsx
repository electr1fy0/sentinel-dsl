import * as React from "react"
import {
  ArrowClockwise,
  CaretLeft,
  CaretRight,
  DownloadSimple,
  Pause,
  Play,
  UploadSimple,
} from "@phosphor-icons/react"
import {
  analyzeCFG,
  analyzeCSG,
  simulateLBA,
  simulatePDA,
  simulateTM,
} from "@/lib/api-client"
import type {
  CFGResponse,
  CSGResponse,
  LBAResponse,
  PDAResponse,
  TMConfiguration,
  TMResponse,
} from "@/lib/types"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Textarea } from "@/components/ui/textarea"

type TheoryTab = "cfg" | "pda" | "tm" | "lba" | "csg"
type JSONRecord = Record<string, unknown>

const PDA_PRESET = JSON.stringify(
  {
    states: ["q0", "q1", "q_accept"],
    input_alphabet: ["0", "1"],
    stack_alphabet: ["Z", "0"],
    start: "q0",
    initial_stack: "Z",
    accept_states: ["q_accept"],
    accept_by_final_state: true,
    accept_by_empty_stack: false,
    transitions: [
      {
        from_state: "q0",
        input: "0",
        stack_top: "Z",
        to_state: "q0",
        push: ["Z", "0"],
      },
      {
        from_state: "q0",
        input: "0",
        stack_top: "0",
        to_state: "q0",
        push: ["0", "0"],
      },
      {
        from_state: "q0",
        input: "1",
        stack_top: "0",
        to_state: "q1",
        push: [],
      },
      {
        from_state: "q1",
        input: "1",
        stack_top: "0",
        to_state: "q1",
        push: [],
      },
      {
        from_state: "q1",
        input: "",
        stack_top: "Z",
        to_state: "q_accept",
        push: ["Z"],
      },
    ],
  },
  null,
  2
)

const TM_PRESET = JSON.stringify(
  {
    states: ["q0", "q_carry", "q_accept"],
    alphabet: ["0", "1"],
    tape_alphabet: ["0", "1", "_"],
    blank: "_",
    start: "q0",
    accept_states: ["q_accept"],
    reject_states: [],
    transitions: [
      { from_state: "q0", read: "0", to_state: "q0", write: "0", move: 1 },
      { from_state: "q0", read: "1", to_state: "q0", write: "1", move: 1 },
      {
        from_state: "q0",
        read: "_",
        to_state: "q_carry",
        write: "_",
        move: -1,
      },
      {
        from_state: "q_carry",
        read: "0",
        to_state: "q_accept",
        write: "1",
        move: 0,
      },
      {
        from_state: "q_carry",
        read: "1",
        to_state: "q_carry",
        write: "0",
        move: -1,
      },
      {
        from_state: "q_carry",
        read: "_",
        to_state: "q_accept",
        write: "1",
        move: 0,
      },
    ],
  },
  null,
  2
)

const LBA_PRESET = JSON.stringify(
  {
    states: ["q0", "q_accept", "q_reject"],
    alphabet: ["a", "b"],
    tape_alphabet: ["a", "b", "_"],
    blank: "_",
    start: "q0",
    accept_states: ["q_accept"],
    reject_states: ["q_reject"],
    transitions: [
      {
        from_state: "q0",
        read: "a",
        to_state: "q_accept",
        write: "a",
        move: 0,
      },
    ],
  },
  null,
  2
)

const CSG_PRESET = JSON.stringify(
  {
    variables: ["S"],
    terminals: ["a", "b"],
    start: "S",
    productions: [
      { lhs: ["S"], rhs: ["a", "S", "b"] },
      { lhs: ["S"], rhs: ["a", "b"] },
    ],
  },
  null,
  2
)

function parseList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseJSON<T>(value: string): T {
  const parsed: unknown = JSON.parse(value)
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
    throw new Error("Definition must be a JSON object")
  return parsed as T
}

function downloadJSON(filename: string, value: unknown) {
  const blob = new Blob([JSON.stringify(value, null, 2)], {
    type: "application/json",
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function ImportExport({
  filename,
  value,
  onImport,
}: {
  filename: string
  value: string
  onImport: (value: string) => void
}) {
  const inputRef = React.useRef<HTMLInputElement>(null)
  return (
    <div className="flex items-center gap-1">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 gap-1 px-1.5 text-[11px] text-muted-foreground"
        onClick={() => {
          try {
            downloadJSON(filename, parseJSON<JSONRecord>(value))
          } catch {
            downloadJSON(filename, value)
          }
        }}
      >
        <DownloadSimple className="size-3" /> Export
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 gap-1 px-1.5 text-[11px] text-muted-foreground"
        onClick={() => inputRef.current?.click()}
      >
        <UploadSimple className="size-3" /> Import
      </Button>
      <input
        ref={inputRef}
        type="file"
        accept="application/json,.json"
        className="hidden"
        aria-label={`Import ${filename}`}
        onChange={(event) => {
          const file = event.target.files?.[0]
          if (!file) return
          void file.text().then(onImport)
          event.target.value = ""
        }}
      />
    </div>
  )
}

function ErrorState({ message }: { message: string }) {
  return (
    <Alert variant="destructive">
      <AlertTitle>Could not run this definition</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  )
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="rounded-xl bg-muted/35 p-8 text-center text-xs text-muted-foreground">
      {message}
    </div>
  )
}

function Playback({
  step,
  length,
  onStep,
}: {
  step: number
  length: number
  onStep: (step: number) => void
}) {
  const [playing, setPlaying] = React.useState(false)
  const maxStep = Math.max(0, length - 1)
  React.useEffect(() => {
    setPlaying(false)
  }, [length])
  React.useEffect(() => {
    if (!playing) return
    const timer = window.setInterval(() => {
      onStep(Math.min(maxStep, step + 1))
      if (step >= maxStep) setPlaying(false)
    }, 650)
    return () => window.clearInterval(timer)
  }, [maxStep, onStep, playing, step])
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (
        target?.isContentEditable ||
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA"
      )
        return
      if (event.key === "ArrowLeft") {
        event.preventDefault()
        onStep(Math.max(0, step - 1))
      }
      if (event.key === "ArrowRight") {
        event.preventDefault()
        onStep(Math.min(maxStep, step + 1))
      }
      if (event.key === " ") {
        event.preventDefault()
        setPlaying((current) => !current)
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [maxStep, onStep, step])
  return (
    <div className="flex items-center gap-1" aria-label="Trace playback">
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label="Reset trace"
        disabled={step === 0}
        onClick={() => onStep(0)}
      >
        <ArrowClockwise className="size-3" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label="Previous trace step"
        disabled={step === 0}
        onClick={() => onStep(Math.max(0, step - 1))}
      >
        <CaretLeft className="size-3" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label={playing ? "Pause trace" : "Play trace"}
        disabled={maxStep === 0}
        onClick={() => setPlaying((current) => !current)}
      >
        {playing ? (
          <Pause className="size-3" />
        ) : (
          <Play weight="fill" className="size-3" />
        )}
      </Button>
      <span className="min-w-16 text-center font-mono text-[11px] text-muted-foreground">
        {step} / {maxStep}
      </span>
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label="Next trace step"
        disabled={step >= maxStep}
        onClick={() => onStep(Math.min(maxStep, step + 1))}
      >
        <CaretRight className="size-3" />
      </Button>
    </div>
  )
}

export function TheoryLab() {
  const [hierarchyTab, setHierarchyTab] = React.useState<TheoryTab>("cfg")
  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden bg-background">
      <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/30 px-4 py-3">
        <div>
          <h1 className="text-sm font-medium text-foreground">Theory</h1>
          <p className="text-[11px] text-muted-foreground">
            Explore bounded, inspectable runs across the Chomsky hierarchy
          </p>
        </div>
        <label className="flex items-center gap-2 text-xs text-muted-foreground">
          Machine
          <select
            aria-label="Choose theory machine"
            value={hierarchyTab}
            onChange={(event) =>
              setHierarchyTab(event.target.value as TheoryTab)
            }
            className="h-8 rounded-md bg-input/70 px-2.5 font-mono text-xs text-foreground outline-none focus:bg-accent focus:ring-2 focus:ring-ring/30"
          >
            <option value="cfg">CFG · Context-free grammar</option>
            <option value="pda">PDA · Pushdown automaton</option>
            <option value="tm">TM · Turing machine</option>
            <option value="lba">LBA · Linear bounded automaton</option>
            <option value="csg">CSG · Context-sensitive grammar</option>
          </select>
        </label>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        {hierarchyTab === "cfg" && <CFGPlayground />}
        {hierarchyTab === "pda" && <MachinePlayground kind="pda" />}
        {hierarchyTab === "tm" && <MachinePlayground kind="tm" />}
        {hierarchyTab === "lba" && <MachinePlayground kind="lba" />}
        {hierarchyTab === "csg" && <CSGPlayground />}
      </div>
    </div>
  )
}

function Field({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <Label className="text-[11px] text-muted-foreground">{label}</Label>
      {children}
    </div>
  )
}

function CFGPlayground() {
  const [variables, setVariables] = React.useState("S, A, B")
  const [terminals, setTerminals] = React.useState("a, b, c")
  const [start, setStart] = React.useState("S")
  const [productionsRaw, setProductionsRaw] = React.useState(
    "S -> a S b | A\nA -> c"
  )
  const [maxSteps, setMaxSteps] = React.useState("8")
  const [result, setResult] = React.useState<CFGResponse | null>(null)
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  const handleCompute = async () => {
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const productions: { lhs: string; rhs: string[] }[] = []
      for (const [index, line] of productionsRaw.split("\n").entries()) {
        if (!line.trim()) continue
        if (!line.includes("->"))
          throw new Error(`Production line ${index + 1} must contain →`)
        const [lhsRaw, rhsRaw] = line.split("->")
        const lhs = lhsRaw.trim()
        if (!lhs)
          throw new Error(`Production line ${index + 1} has no left-hand side`)
        for (const option of rhsRaw.split("|")) {
          const rhs = option.trim()
          productions.push({
            lhs,
            rhs: rhs === "ε" || rhs === "" ? [] : rhs.split(/\s+/),
          })
        }
      }
      const response = await analyzeCFG({
        variables: parseList(variables),
        terminals: parseList(terminals),
        start: start.trim(),
        productions,
        max_steps: Math.max(1, Math.min(20, Number(maxSteps) || 8)),
      })
      if (!response.valid)
        throw new Error(response.error || "The grammar is invalid")
      setResult(response)
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The CFG service is unavailable"
      )
    } finally {
      setLoading(false)
    }
  }

  const exportValue = JSON.stringify(
    {
      variables: parseList(variables),
      terminals: parseList(terminals),
      start,
      productions: productionsRaw,
    },
    null,
    2
  )
  const importGrammar = (value: string) => {
    try {
      const parsed = parseJSON<JSONRecord>(value)
      if (Array.isArray(parsed.variables))
        setVariables(parsed.variables.join(", "))
      if (Array.isArray(parsed.terminals))
        setTerminals(parsed.terminals.join(", "))
      if (typeof parsed.start === "string") setStart(parsed.start)
      if (typeof parsed.productions === "string")
        setProductionsRaw(parsed.productions)
      else if (Array.isArray(parsed.productions))
        setProductionsRaw(
          parsed.productions
            .map((item) => {
              const production = item as JSONRecord
              const lhs = String(production.lhs || "")
              const rhs = Array.isArray(production.rhs)
                ? production.rhs.join(" ")
                : "ε"
              return `${lhs} -> ${rhs}`
            })
            .join("\n")
        )
    } catch {
      setError("Import must be a valid CFG JSON definition")
    }
  }

  return (
    <div className="grid h-full grid-cols-1 gap-2 overflow-hidden bg-muted/55 p-2 lg:grid-cols-[360px_1fr]">
      <div className="flex flex-col gap-3 overflow-y-auto rounded-xl bg-background p-4">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium">Grammar definition</span>
          <ImportExport
            filename="grammar.cfg.json"
            value={exportValue}
            onImport={importGrammar}
          />
        </div>
        <Field label="Variables (non-terminals)">
          <Input
            value={variables}
            onChange={(event) => setVariables(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <Field label="Terminals">
          <Input
            value={terminals}
            onChange={(event) => setTerminals(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <Field label="Start variable">
          <Input
            value={start}
            onChange={(event) => setStart(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <Field label="Productions (one per line; ε is empty)">
          <Textarea
            value={productionsRaw}
            onChange={(event) => setProductionsRaw(event.target.value)}
            className="min-h-28 font-mono text-xs leading-relaxed"
          />
        </Field>
        <Field label="Derivation step limit">
          <Input
            type="number"
            min={1}
            max={20}
            value={maxSteps}
            onChange={(event) => setMaxSteps(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <Button
          size="sm"
          onClick={() => void handleCompute()}
          disabled={loading}
          className="h-7 gap-1.5 text-xs"
        >
          <Play weight="fill" className="size-2.5" />
          {loading ? "Analyzing…" : "Analyze grammar"}
        </Button>
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          Runs are bounded by the step limit. Increase it for recursive
          grammars, but expect more work.
        </p>
      </div>
      <ScrollArea className="h-full rounded-xl bg-background p-4">
        <div className="flex max-w-4xl flex-col gap-4">
          {error && <ErrorState message={error} />}
          {loading && <EmptyState message="Analyzing grammar…" />}
          {!loading && !error && !result && (
            <EmptyState message="Edit the grammar and run an analysis to inspect FIRST, FOLLOW, and derivations." />
          )}
          {result && (
            <>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <SetCard
                  title="FIRST"
                  values={result.first}
                  empty="No FIRST sets returned."
                />
                <SetCard
                  title="FOLLOW"
                  values={result.follow}
                  empty="No FOLLOW sets returned."
                />
              </div>
              <div className="rounded-xl bg-card p-3">
                <span className="mb-2 block text-xs font-medium">
                  Generated terminal sentences
                </span>
                {result.derivations?.length ? (
                  <div className="flex flex-wrap gap-1.5">
                    {result.derivations.map((derivation, index) => (
                      <span
                        key={index}
                        className="rounded-lg bg-muted/55 px-2 py-0.5 font-mono text-xs"
                      >
                        {derivation.join("") || "ε"}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground">
                    No finite derivations found within the bound.
                  </p>
                )}
              </div>
              {result.simplified && (
                <div className="rounded-xl bg-card p-3">
                  <span className="mb-2 block text-xs font-medium">
                    Simplified grammar
                  </span>
                  <div className="flex flex-col gap-1 font-mono text-xs">
                    {result.simplified.productions.map((production, index) => (
                      <div key={index}>
                        <span className="font-medium">{production.lhs}</span> →{" "}
                        {production.rhs.length ? production.rhs.join(" ") : "ε"}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}

function SetCard({
  title,
  values,
  empty,
}: {
  title: string
  values?: Record<string, string[]>
  empty: string
}) {
  return (
    <div className="rounded-xl bg-card p-3">
      <span className="mb-2 block text-xs font-medium">{title} sets</span>
      {values && Object.keys(values).length ? (
        <div className="flex flex-col gap-1 font-mono text-xs">
          {Object.entries(values).map(([name, set]) => (
            <div
              key={name}
              className="flex items-center justify-between rounded-lg bg-muted/45 px-2 py-1"
            >
              <span className="font-medium">
                {title}({name}) =
              </span>
              <span className="text-muted-foreground">{`{ ${set.map((item) => item || "ε").join(", ")} }`}</span>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">{empty}</p>
      )}
    </div>
  )
}

type MachineKind = "pda" | "tm" | "lba"
type MachineResponse = PDAResponse | TMResponse | LBAResponse

function MachinePlayground({ kind }: { kind: MachineKind }) {
  const defaults =
    kind === "pda"
      ? { definition: PDA_PRESET, input: "0 0 1 1" }
      : kind === "tm"
        ? { definition: TM_PRESET, input: "1 1 0 1" }
        : { definition: LBA_PRESET, input: "a" }
  const [definition, setDefinition] = React.useState(defaults.definition)
  const [input, setInput] = React.useState(defaults.input)
  const [maxSteps, setMaxSteps] = React.useState(kind === "pda" ? "50" : "100")
  const [result, setResult] = React.useState<MachineResponse | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(false)
  const [step, setStep] = React.useState(0)
  const title =
    kind === "pda"
      ? "Pushdown automaton"
      : kind === "tm"
        ? "Turing machine"
        : "Linear-bounded automaton"

  const run = async () => {
    setLoading(true)
    setError(null)
    setResult(null)
    setStep(0)
    try {
      const parsed = parseJSON<JSONRecord>(definition)
      const payload = {
        ...parsed,
        input: parseList(input),
        max_steps: Math.max(
          1,
          Math.min(kind === "pda" ? 100 : 200, Number(maxSteps) || 50)
        ),
      }
      const response =
        kind === "pda"
          ? await simulatePDA(payload as Parameters<typeof simulatePDA>[0])
          : kind === "tm"
            ? await simulateTM(payload as Parameters<typeof simulateTM>[0])
            : await simulateLBA(payload as Parameters<typeof simulateLBA>[0])
      if (!response.valid)
        throw new Error(response.error || `${title} definition is invalid`)
      setResult(response)
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : `${title} service is unavailable`
      )
    } finally {
      setLoading(false)
    }
  }
  const trace = result?.trace ?? []
  const current = trace[Math.min(step, Math.max(trace.length - 1, 0))]
  const preset = () => {
    setDefinition(defaults.definition)
    setInput(defaults.input)
    setError(null)
  }
  const renderConfiguration = () => {
    if (trace.length === 0)
      return (
        <EmptyState message="Run the machine to inspect its configuration trace." />
      )
    if (kind === "pda") {
      const pda = current as {
        state: string
        position: number
        stack: string[]
      }
      return (
        <div className="grid grid-cols-2 gap-2 text-xs">
          <Metric label="State" value={pda.state} />
          <Metric label="Input position" value={`${pda.position}`} />
          <div className="col-span-2 rounded-lg bg-muted/45 p-2">
            <span className="text-muted-foreground">Stack (bottom → top)</span>
            <div className="mt-2 flex flex-wrap gap-1">
              {pda.stack.length ? (
                pda.stack.map((symbol, index) => (
                  <span
                    key={index}
                    className="rounded-lg bg-muted/55 px-2 py-1 font-mono"
                  >
                    {symbol}
                  </span>
                ))
              ) : (
                <span className="font-mono">ε</span>
              )}
            </div>
          </div>
        </div>
      )
    }
    if (kind === "lba") {
      const lba = current as { state: string; head: number; tape: string[] }
      return (
        <div className="flex flex-col gap-2 text-xs">
          <Metric label="State" value={lba.state} />
          <div className="flex flex-wrap gap-1">
            {lba.tape.map((symbol, index) => (
              <span
                key={index}
                className={`rounded-lg px-2 py-1 font-mono ${index === lba.head ? "bg-primary text-primary-foreground" : "bg-muted/55"}`}
              >
                {symbol}
              </span>
            ))}
          </div>
          <span className="font-mono text-muted-foreground">
            head → {lba.head}
          </span>
        </div>
      )
    }
    const tm = current as TMConfiguration
    const cells: number[] = []
    for (let index = tm.min_index; index <= tm.max_index; index += 1)
      cells.push(index)
    return (
      <div className="flex flex-col gap-2 text-xs">
        <Metric label="State" value={tm.state} />
        <div className="flex flex-wrap gap-1">
          {cells.map((index) => (
            <span
              key={index}
              className={`rounded-lg px-2 py-1 font-mono ${index === tm.head ? "bg-primary text-primary-foreground" : "bg-muted/55"}`}
            >
              {tm.tape[index] || "_"}
            </span>
          ))}
        </div>
        <span className="font-mono text-muted-foreground">
          head → {tm.head}
        </span>
      </div>
    )
  }

  return (
    <div className="grid h-full grid-cols-1 gap-2 overflow-hidden bg-muted/55 p-2 lg:grid-cols-[390px_1fr]">
      <div className="flex flex-col gap-3 overflow-y-auto rounded-xl bg-background p-4">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium">{title} definition</span>
          <ImportExport
            filename={`${kind}.machine.json`}
            value={definition}
            onImport={setDefinition}
          />
        </div>
        <Field label="Definition JSON">
          <Textarea
            value={definition}
            onChange={(event) => setDefinition(event.target.value)}
            className="min-h-[300px] font-mono text-[11px] leading-relaxed"
            spellCheck={false}
          />
        </Field>
        <Field label="Input symbols (space-separated)">
          <Input
            value={input}
            onChange={(event) => setInput(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <Field label="Step limit">
          <Input
            type="number"
            min={1}
            max={kind === "pda" ? 100 : 200}
            value={maxSteps}
            onChange={(event) => setMaxSteps(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <div className="flex gap-1.5">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={preset}
          >
            Load preset
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => setDefinition("{}")}
          >
            Clear definition
          </Button>
        </div>
        <Button
          size="sm"
          onClick={() => void run()}
          disabled={loading}
          className="h-7 gap-1.5 text-xs"
        >
          <Play weight="fill" className="size-2.5" />
          {loading ? "Running…" : "Run machine"}
        </Button>
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          Definitions are sent to the real bounded simulator. A timeout is a
          valid execution outcome, not an acceptance.
        </p>
      </div>
      <ScrollArea className="h-full rounded-xl bg-background p-4">
        <div className="flex max-w-4xl flex-col gap-4">
          {error && <ErrorState message={error} />}
          {loading && (
            <EmptyState message={`Running ${title.toLowerCase()}…`} />
          )}
          {!loading && !error && !result && (
            <EmptyState message="Edit or import a definition, then run it to inspect the trace." />
          )}
          {result && (
            <>
              <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-card p-3">
                <div className="flex items-center gap-2 text-xs">
                  <span
                    className={`size-2 rounded-full ${result.accepted ? "bg-emerald-500" : "timed_out" in result && result.timed_out ? "bg-amber-500" : "bg-destructive"}`}
                  />
                  <span className="font-medium">
                    {result.accepted
                      ? "Accepted"
                      : "timed_out" in result && result.timed_out
                        ? "Timed out"
                        : "rejected" in result && result.rejected
                          ? "Rejected"
                          : "Halted"}
                  </span>
                  <span className="font-mono text-muted-foreground">
                    ({result.steps ?? 0} steps)
                  </span>
                </div>
                <Playback step={step} length={trace.length} onStep={setStep} />
              </div>
              <div className="rounded-xl bg-card p-3">
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-xs font-medium">
                    Configuration at step {step}
                  </span>
                  <span className="font-mono text-[11px] text-muted-foreground">
                    ←/→ or Space to play
                  </span>
                </div>
                {renderConfiguration()}
              </div>
              <div className="rounded-xl bg-card p-3">
                <span className="mb-2 block text-xs font-medium">
                  Trace history
                </span>
                {trace.length ? (
                  <div className="flex flex-wrap gap-1">
                    {trace.map((item, index) => (
                      <Button
                        key={index}
                        type="button"
                        variant={index === step ? "secondary" : "ghost"}
                        size="sm"
                        className="h-6 px-2 font-mono text-[11px]"
                        onClick={() => setStep(index)}
                      >
                        #{index} {item.state}
                      </Button>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground">
                    The simulator returned no trace.
                  </p>
                )}
              </div>
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/45 p-2">
      <span className="block text-[11px] text-muted-foreground">{label}</span>
      <span className="font-mono text-xs">{value}</span>
    </div>
  )
}

function CSGPlayground() {
  const [definition, setDefinition] = React.useState(CSG_PRESET)
  const [input, setInput] = React.useState("a a b b")
  const [maxSteps, setMaxSteps] = React.useState("20")
  const [maxLength, setMaxLength] = React.useState("8")
  const [result, setResult] = React.useState<CSGResponse | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(false)
  const run = async () => {
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const parsed = parseJSON<JSONRecord>(definition)
      const response = await analyzeCSG({
        ...(parsed as Parameters<typeof analyzeCSG>[0]),
        input: parseList(input),
        max_steps: Math.max(1, Math.min(50, Number(maxSteps) || 20)),
        max_length: Math.max(1, Math.min(50, Number(maxLength) || 8)),
      })
      if (!response.valid)
        throw new Error(response.error || "The CSG definition is invalid")
      setResult(response)
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The CSG service is unavailable"
      )
    } finally {
      setLoading(false)
    }
  }
  return (
    <div className="grid h-full grid-cols-1 gap-2 overflow-hidden bg-muted/55 p-2 lg:grid-cols-[390px_1fr]">
      <div className="flex flex-col gap-3 overflow-y-auto rounded-xl bg-background p-4">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium">Context-sensitive grammar</span>
          <ImportExport
            filename="grammar.csg.json"
            value={definition}
            onImport={setDefinition}
          />
        </div>
        <Field label="Definition JSON">
          <Textarea
            value={definition}
            onChange={(event) => setDefinition(event.target.value)}
            className="min-h-[260px] font-mono text-[11px] leading-relaxed"
            spellCheck={false}
          />
        </Field>
        <Field label="Input symbols (space-separated)">
          <Input
            value={input}
            onChange={(event) => setInput(event.target.value)}
            className="h-7 font-mono text-xs"
          />
        </Field>
        <div className="grid grid-cols-2 gap-2">
          <Field label="Step limit">
            <Input
              type="number"
              min={1}
              max={50}
              value={maxSteps}
              onChange={(event) => setMaxSteps(event.target.value)}
              className="h-7 font-mono text-xs"
            />
          </Field>
          <Field label="Form length limit">
            <Input
              type="number"
              min={1}
              max={50}
              value={maxLength}
              onChange={(event) => setMaxLength(event.target.value)}
              className="h-7 font-mono text-xs"
            />
          </Field>
        </div>
        <div className="flex gap-1.5">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => {
              setDefinition(CSG_PRESET)
              setInput("a a b b")
            }}
          >
            Load aⁿbⁿ preset
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => setDefinition("{}")}
          >
            Clear definition
          </Button>
        </div>
        <Button
          size="sm"
          onClick={() => void run()}
          disabled={loading}
          className="h-7 gap-1.5 text-xs"
        >
          <Play weight="fill" className="size-2.5" />
          {loading ? "Recognizing…" : "Recognize input"}
        </Button>
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          CSG recognition is breadth-first and bounded by both steps and
          sentential-form length.
        </p>
      </div>
      <ScrollArea className="h-full rounded-xl bg-background p-4">
        <div className="flex max-w-4xl flex-col gap-4">
          {error && <ErrorState message={error} />}
          {loading && <EmptyState message="Recognizing input…" />}
          {!loading && !error && !result && (
            <EmptyState message="Edit or import a grammar, then recognize an input to inspect its derivation." />
          )}
          {result && (
            <>
              <div className="flex items-center justify-between rounded-xl bg-card p-3 text-xs">
                <div className="flex items-center gap-2">
                  <span
                    className={`size-2 rounded-full ${result.accepted ? "bg-emerald-500" : "bg-destructive"}`}
                  />
                  <span className="font-medium">
                    {result.accepted ? "Accepted" : "Not derived within bounds"}
                  </span>
                </div>
                <span className="font-mono text-muted-foreground">
                  {result.steps ?? 0} steps
                </span>
              </div>
              <div className="rounded-xl bg-card p-3">
                <span className="mb-2 block text-xs font-medium">
                  Derivation
                </span>
                {result.derivation?.length ? (
                  <div className="flex flex-col gap-1">
                    {result.derivation.map((form, index) => (
                      <div
                        key={index}
                        className="flex items-center gap-2 font-mono text-xs"
                      >
                        <span className="w-7 text-right text-[10px] text-muted-foreground">
                          {index}
                        </span>
                        <span className="rounded-lg bg-muted/55 px-2 py-1">
                          {form.join(" ") || "ε"}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground">
                    No derivation found before the configured limits.
                  </p>
                )}
              </div>
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
