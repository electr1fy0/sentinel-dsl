import * as React from "react"
import { createFileRoute } from "@tanstack/react-router"
import {
  Binary,
  BookOpenText,
  CheckCircle as CheckCircle2,
  Cpu,
  MagnifyingGlass,
  Plus,
  Shield,
  Stack as Layers,
} from "@phosphor-icons/react"
import { Input } from "@/components/ui/input"
import { Header } from "@/components/studio/header"
import { CommandPalette } from "@/components/studio/command-palette"
import { PolicyLab } from "@/components/studio/policy-lab"
import type { ImportedPolicy } from "@/components/studio/policy-lab"
import {
  analyzeAutomata,
  ApiError,
  checkHealth,
  explainPolicy,
  fetchExamples,
  isApiCancellation,
  validatePolicy,
} from "@/lib/api-client"
import type {
  AutomataResponse,
  DecisionResponse,
  ExampleScenario,
  RequestPayload,
  ValidateResponse,
} from "@/lib/types"

export const Route = createFileRoute("/")({ component: SentinelStudioApp })

type StudioTab = "policy" | "automata" | "theory" | "compiler" | "tests"

const AutomataLab = React.lazy(() =>
  import("@/components/studio/automata-lab").then((module) => ({
    default: module.AutomataLab,
  }))
)
const TheoryLab = React.lazy(() =>
  import("@/components/studio/theory-lab").then((module) => ({
    default: module.TheoryLab,
  }))
)
const CompilerVisualizer = React.lazy(() =>
  import("@/components/studio/compiler-visualizer").then((module) => ({
    default: module.CompilerVisualizer,
  }))
)
const TestMatrix = React.lazy(() =>
  import("@/components/studio/test-matrix").then((module) => ({
    default: module.TestMatrix,
  }))
)

const EMPTY_REQUEST: RequestPayload = {
  method: "GET",
  path: "",
  host: "",
  scheme: "https",
  role: "",
  headers: {},
  query: {},
  fields: {},
  remote_addr: "",
  user_agent: "",
}

const EMPTY_POLICY = { source: "", request: EMPTY_REQUEST }
const NEW_POLICY_ID = "new"

function cloneRequest(request: RequestPayload): RequestPayload {
  return {
    ...request,
    headers: Object.fromEntries(
      Object.entries(request.headers).map(([key, values]) => [
        key,
        Array.isArray(values) ? values.map(String) : [String(values)],
      ])
    ),
    query: { ...request.query },
    fields: { ...request.fields },
  }
}

function clonePolicy(policy: ImportedPolicy): ImportedPolicy {
  return { source: policy.source, request: cloneRequest(policy.request) }
}

function policyName(source: string): string {
  return (
    source.match(/^\s*policy\s+([A-Za-z_][A-Za-z0-9_]*)/m)?.[1] ??
    "Untitled policy"
  )
}

function localPolicyExample(policy: ImportedPolicy): ExampleScenario {
  return {
    id: NEW_POLICY_ID,
    category: "Draft",
    name: policyName(policy.source),
    description: "Unsaved local policy",
    source: policy.source,
    request: cloneRequest(policy.request),
  }
}

function withLocalPolicy(
  examples: ExampleScenario[],
  policy: ImportedPolicy
): ExampleScenario[] {
  return [
    localPolicyExample(policy),
    ...examples.filter((example) => example.id !== NEW_POLICY_ID),
  ]
}

function samePolicy(a: ImportedPolicy, b: ImportedPolicy): boolean {
  return (
    a.source === b.source &&
    JSON.stringify(a.request) === JSON.stringify(b.request)
  )
}

function getInitialTab(): StudioTab {
  if (typeof window === "undefined") return "policy"
  const tab = new URLSearchParams(window.location.search).get("tab")
  return tab === "automata" ||
    tab === "theory" ||
    tab === "compiler" ||
    tab === "tests"
    ? tab
    : "policy"
}

function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  return error instanceof Error
    ? error.message
    : "An unexpected API error occurred."
}

function getStoredTheme(): boolean {
  if (typeof window === "undefined") return true
  const stored = window.localStorage.getItem("sentinel-theme")
  if (stored === "light") return false
  if (stored === "dark") return true
  return window.matchMedia("(prefers-color-scheme: dark)").matches
}

function readDraft(key: string): ImportedPolicy | null {
  if (typeof window === "undefined") return null
  try {
    const stored = window.localStorage.getItem(`sentinel:draft:${key}`)
    if (!stored) return null
    const value = JSON.parse(stored) as Partial<ImportedPolicy>
    if (typeof value.source !== "string" || !value.request) return null
    return clonePolicy({ source: value.source, request: value.request })
  } catch {
    return null
  }
}

function writeDraft(key: string, value: ImportedPolicy): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.setItem(`sentinel:draft:${key}`, JSON.stringify(value))
  } catch {
    // Storage can be disabled or full. The editor remains usable in that case.
  }
}

function clearDraft(key: string): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.removeItem(`sentinel:draft:${key}`)
  } catch {
    // Ignore storage failures while resetting the in-memory document.
  }
}

function SentinelStudioApp() {
  const [activeTab, setActiveTab] = React.useState<StudioTab>(getInitialTab)
  const [examples, setExamples] = React.useState<ExampleScenario[]>([])
  const [scenarioQuery, setScenarioQuery] = React.useState("")
  const [selectedExampleId, setSelectedExampleId] = React.useState<
    string | undefined
  >()
  const [source, setSource] = React.useState("")
  const [request, setRequest] = React.useState<RequestPayload>(
    cloneRequest(EMPTY_REQUEST)
  )
  const [baseline, setBaseline] = React.useState<ImportedPolicy>(EMPTY_POLICY)
  const [validation, setValidation] = React.useState<ValidateResponse | null>(
    null
  )
  const [decision, setDecision] = React.useState<DecisionResponse | null>(null)
  const [automataPattern, setAutomataPattern] =
    React.useState("/api/users/[0-9]+")
  const [automataInput, setAutomataInput] = React.useState("/api/users/42")
  const [automataData, setAutomataData] =
    React.useState<AutomataResponse | null>(null)
  const [isBackendConnected, setIsBackendConnected] = React.useState(false)
  const [isBackendChecking, setIsBackendChecking] = React.useState(true)
  const [backendVersion, setBackendVersion] = React.useState<string>()
  const [backendError, setBackendError] = React.useState<string>()
  const [commandPaletteOpen, setCommandPaletteOpen] = React.useState(false)
  const [policyLoading, setPolicyLoading] = React.useState(false)
  const [automataLoading, setAutomataLoading] = React.useState(false)
  const [policyError, setPolicyError] = React.useState<string>()
  const [evaluationMeta, setEvaluationMeta] = React.useState<{
    durationMs: number
    completedAt: number
  }>()
  const [darkMode, setDarkMode] = React.useState(getStoredTheme)
  const [draftHydrated, setDraftHydrated] = React.useState(false)
  const validationController = React.useRef<AbortController | null>(null)
  const validationVersion = React.useRef(0)
  const evaluationController = React.useRef<AbortController | null>(null)
  const automataController = React.useRef<AbortController | null>(null)
  const validatedSource = React.useRef<string | null>(null)

  const currentPolicy = React.useMemo(
    () => ({ source, request }),
    [source, request]
  )
  const isDirty = !samePolicy(currentPolicy, baseline)
  const draftKey = selectedExampleId || NEW_POLICY_ID

  React.useEffect(() => {
    document.documentElement.classList.toggle("dark", darkMode)
    window.localStorage.setItem("sentinel-theme", darkMode ? "dark" : "light")
  }, [darkMode])

  React.useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const onChange = (event: MediaQueryListEvent) => {
      if (!window.localStorage.getItem("sentinel-theme"))
        setDarkMode(event.matches)
    }
    media.addEventListener("change", onChange)
    return () => media.removeEventListener("change", onChange)
  }, [])

  React.useEffect(() => {
    let cancelled = false
    const load = async () => {
      const health = await checkHealth()
      if (cancelled) return
      setIsBackendConnected(health.ok)
      setIsBackendChecking(false)
      setBackendVersion(health.version)
      setBackendError(health.ok ? undefined : apiErrorMessage(health.error))
      try {
        const fetched = await fetchExamples()
        const localDraft = readDraft(NEW_POLICY_ID)
        const available = localDraft
          ? withLocalPolicy(fetched, localDraft)
          : fetched
        setExamples(available)
        if (available.length === 0) return
        const requested = new URLSearchParams(window.location.search).get(
          "example"
        )
        const selected =
          available.find((item) => item.id === requested) ?? available[0]
        const draft = readDraft(selected.id)
        const initial = draft ?? {
          source: selected.source,
          request: selected.request,
        }
        setSelectedExampleId(selected.id)
        setBaseline(
          selected.id === NEW_POLICY_ID
            ? clonePolicy(EMPTY_POLICY)
            : clonePolicy({
                source: selected.source,
                request: selected.request,
              })
        )
        setSource(initial.source)
        setRequest(cloneRequest(initial.request))
      } catch (error) {
        setBackendError(apiErrorMessage(error))
      } finally {
        setDraftHydrated(true)
      }
    }
    void load()
    return () => {
      cancelled = true
      validationController.current?.abort()
      evaluationController.current?.abort()
      automataController.current?.abort()
    }
  }, [])

  React.useEffect(() => {
    if (!draftHydrated) return
    const timer = window.setTimeout(
      () => writeDraft(draftKey, currentPolicy),
      500
    )
    return () => window.clearTimeout(timer)
  }, [currentPolicy, draftHydrated, draftKey])

  React.useEffect(() => {
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!isDirty) return
      event.preventDefault()
      event.returnValue = ""
    }
    window.addEventListener("beforeunload", handleBeforeUnload)
    return () => window.removeEventListener("beforeunload", handleBeforeUnload)
  }, [isDirty])

  React.useEffect(() => {
    if (typeof window === "undefined") return
    const params = new URLSearchParams(window.location.search)
    params.set("tab", activeTab)
    if (selectedExampleId) params.set("example", selectedExampleId)
    else params.delete("example")
    window.history.replaceState(
      null,
      "",
      `${window.location.pathname}?${params.toString()}`
    )
  }, [activeTab, selectedExampleId])

  const runValidation =
    React.useCallback(async (): Promise<ValidateResponse | null> => {
      const version = ++validationVersion.current
      validationController.current?.abort()
      const controller = new AbortController()
      validationController.current = controller
      setPolicyLoading(true)
      setPolicyError(undefined)
      try {
        const result = await validatePolicy(source, {
          signal: controller.signal,
        })
        if (version !== validationVersion.current) return null
        validatedSource.current = source
        setValidation(result)
        return result
      } catch (error) {
        if (isApiCancellation(error)) return null
        if (version !== validationVersion.current) return null
        validatedSource.current = null
        setValidation(null)
        setPolicyError(apiErrorMessage(error))
        return null
      } finally {
        if (version === validationVersion.current) setPolicyLoading(false)
      }
    }, [source])

  React.useEffect(() => {
    setDecision(null)
    setEvaluationMeta(undefined)
    setValidation(null)
    validatedSource.current = null
    const timer = window.setTimeout(() => void runValidation(), 350)
    return () => window.clearTimeout(timer)
  }, [runValidation])

  const handleSelectExample = React.useCallback(
    (example: ExampleScenario) => {
      if (example.id === selectedExampleId) return
      if (isDirty && !window.confirm("Discard the unsaved policy draft?"))
        return
      const isLocalPolicy = example.id === NEW_POLICY_ID
      const draft = isLocalPolicy ? null : readDraft(example.id)
      const initial = draft ?? {
        source: example.source,
        request: example.request,
      }
      setSelectedExampleId(example.id)
      setBaseline(
        isLocalPolicy
          ? clonePolicy(EMPTY_POLICY)
          : clonePolicy({ source: example.source, request: example.request })
      )
      setSource(initial.source)
      setRequest(cloneRequest(initial.request))
      setDecision(null)
      setValidation(null)
      setPolicyError(undefined)
    },
    [isDirty, selectedExampleId]
  )

  const handleSourceChange = (next: string) => {
    setSource(next)
    if (selectedExampleId === NEW_POLICY_ID) {
      setExamples((current) =>
        current.map((example) =>
          example.id === NEW_POLICY_ID
            ? { ...example, name: policyName(next), source: next }
            : example
        )
      )
    }
  }
  const handleRequestChange = (next: RequestPayload) => {
    const cloned = cloneRequest(next)
    setRequest(cloned)
    if (selectedExampleId === NEW_POLICY_ID) {
      setExamples((current) =>
        current.map((example) =>
          example.id === NEW_POLICY_ID
            ? { ...example, request: cloned }
            : example
        )
      )
    }
    setDecision(null)
    setEvaluationMeta(undefined)
  }

  const handleClearResult = () => {
    setDecision(null)
    setEvaluationMeta(undefined)
  }

  const handleEvaluate = React.useCallback(async () => {
    if (!validation?.valid || validatedSource.current !== source) {
      const result = await runValidation()
      if (!result?.valid) return
    }
    validationController.current?.abort()
    // Invalidate the debounced validator so its finally block cannot clear
    // the evaluation loading state after we intentionally cancelled it.
    validationVersion.current += 1
    evaluationController.current?.abort()
    const controller = new AbortController()
    evaluationController.current = controller
    setPolicyLoading(true)
    setPolicyError(undefined)
    setDecision(null)
    setEvaluationMeta(undefined)
    const startedAt = performance.now()
    try {
      const result = await explainPolicy(source, request, {
        signal: controller.signal,
      })
      setValidation({
        valid: result.valid,
        diagnostics: result.diagnostics,
        policy:
          validatedSource.current === source ? validation?.policy : undefined,
      })
      if (result.valid && result.decision) setDecision(result.decision)
      setEvaluationMeta({
        durationMs: Math.round(performance.now() - startedAt),
        completedAt: Date.now(),
      })
    } catch (error) {
      if (!isApiCancellation(error)) {
        setDecision(null)
        setValidation(null)
        setPolicyError(apiErrorMessage(error))
      }
    } finally {
      setPolicyLoading(false)
    }
  }, [request, runValidation, source, validation?.policy, validation?.valid])

  const handleAnalyzeAutomata = React.useCallback(async () => {
    automataController.current?.abort()
    const controller = new AbortController()
    automataController.current = controller
    setAutomataLoading(true)
    try {
      setAutomataData(
        await analyzeAutomata(automataPattern, automataInput, {
          signal: controller.signal,
        })
      )
    } catch (error) {
      if (!isApiCancellation(error)) {
        setAutomataData(null)
        setBackendError(apiErrorMessage(error))
      }
    } finally {
      setAutomataLoading(false)
    }
  }, [automataInput, automataPattern])

  const handleReset = () => {
    if (
      isDirty &&
      !window.confirm("Discard unsaved changes and reset this policy?")
    )
      return
    const next = clonePolicy(baseline)
    clearDraft(draftKey)
    if (selectedExampleId === NEW_POLICY_ID) {
      setExamples((current) => withLocalPolicy(current, next))
    }
    setSource(next.source)
    setRequest(next.request)
    setDecision(null)
    setValidation(null)
    setEvaluationMeta(undefined)
  }

  const handleNewPolicy = () => {
    if (
      isDirty &&
      !window.confirm("Discard unsaved changes and start a new policy?")
    )
      return
    clearDraft(draftKey)
    const next = clonePolicy(EMPTY_POLICY)
    setExamples((current) => withLocalPolicy(current, next))
    setSelectedExampleId(NEW_POLICY_ID)
    setBaseline(next)
    setSource(next.source)
    setRequest(next.request)
    setDecision(null)
    setValidation(null)
    setEvaluationMeta(undefined)
  }

  const handleImport = (imported: ImportedPolicy) => {
    if (
      isDirty &&
      !window.confirm("Discard unsaved changes and import this policy?")
    )
      return
    const next = clonePolicy(imported)
    setExamples((current) => withLocalPolicy(current, next))
    setSelectedExampleId(NEW_POLICY_ID)
    setBaseline(next)
    setSource(next.source)
    setRequest(next.request)
    setDecision(null)
    setValidation(null)
    setEvaluationMeta(undefined)
  }

  const primaryNavItems: {
    id: StudioTab
    label: string
    icon: typeof Shield
  }[] = [
    { id: "policy", label: "Policy", icon: Shield },
    { id: "tests", label: "Tests", icon: CheckCircle2 },
  ]
  const learnNavItems: {
    id: StudioTab
    label: string
    icon: typeof Shield
  }[] = [
    { id: "compiler", label: "Compiler", icon: Cpu },
    { id: "automata", label: "Automata", icon: Binary },
    { id: "theory", label: "Theory", icon: Layers },
  ]
  const currentExample = examples.find(
    (example) => example.id === selectedExampleId
  )
  const visibleExamples = examples.filter((example) =>
    `${example.name} ${example.description}`
      .toLowerCase()
      .includes(scenarioQuery.trim().toLowerCase())
  )
  const isLearnTab = learnNavItems.some((item) => item.id === activeTab)

  const setLearnTab = (tab: StudioTab) => {
    setActiveTab(tab)
  }

  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-muted/60 text-foreground selection:bg-primary/20">
      <div className="shrink-0 px-2 pt-2">
        <Header
          isBackendConnected={isBackendConnected}
          backendVersion={backendVersion}
          backendError={backendError}
          backendChecking={isBackendChecking}
          onRetryHealth={() => window.location.reload()}
          currentExampleName={currentExample?.name}
          onOpenCommandPalette={() => setCommandPaletteOpen(true)}
          darkMode={darkMode}
          onToggleDarkMode={() => setDarkMode((value) => !value)}
        />
      </div>

      <nav
        aria-label="Primary navigation"
        className="mx-2 mt-2 flex shrink-0 gap-1 overflow-x-auto rounded-xl bg-sidebar px-3 py-2 md:hidden"
      >
        {primaryNavItems.map((item) => {
          const Icon = item.icon
          const isActive = activeTab === item.id
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => setActiveTab(item.id)}
              aria-current={isActive ? "page" : undefined}
              className={`flex h-8 shrink-0 items-center gap-1.5 rounded-md px-3 text-xs ${isActive ? "bg-background font-medium text-foreground" : "text-muted-foreground"}`}
            >
              <Icon className="size-3.5" />
              {item.label}
            </button>
          )
        })}
        <button
          type="button"
          onClick={() => setLearnTab(isLearnTab ? activeTab : "compiler")}
          aria-current={isLearnTab ? "page" : undefined}
          className={`flex h-8 shrink-0 items-center gap-1.5 rounded-md px-3 text-xs ${isLearnTab ? "bg-background font-medium text-foreground" : "text-muted-foreground"}`}
        >
          <BookOpenText className="size-3.5" />
          Learn
        </button>
      </nav>

      {isLearnTab && (
        <nav
          aria-label="Learn navigation"
          className="mx-2 mt-1 flex shrink-0 gap-1 overflow-x-auto rounded-xl bg-background px-3 py-1.5 md:hidden"
        >
          {learnNavItems.map((item) => {
            const Icon = item.icon
            const isActive = activeTab === item.id
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setLearnTab(item.id)}
                aria-current={isActive ? "page" : undefined}
                className={`flex h-7 shrink-0 items-center gap-1.5 rounded-md px-2.5 text-[11px] ${isActive ? "bg-muted font-medium text-foreground" : "text-muted-foreground"}`}
              >
                <Icon className="size-3" />
                {item.label}
              </button>
            )
          })}
        </nav>
      )}

      <div className="flex min-h-0 w-full flex-1 gap-2 overflow-hidden p-2">
        <aside className="hidden w-60 shrink-0 flex-col overflow-hidden rounded-2xl bg-sidebar select-none md:flex">
          <div className="p-3">
            <button
              type="button"
              onClick={handleNewPolicy}
              className="flex h-9 w-full items-center justify-center gap-2 rounded-lg bg-primary px-3 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
            >
              <Plus className="size-3.5" />
              New policy
            </button>
          </div>

          <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-2 pb-3">
            <div className="flex flex-col gap-1">
              <span className="px-2 text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                Workspace
              </span>
              {primaryNavItems.map((item) => {
                const Icon = item.icon
                const isActive = activeTab === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setActiveTab(item.id)}
                    className={`flex h-8 w-full cursor-pointer items-center gap-2 rounded-md px-2.5 text-left text-xs transition-colors ${isActive ? "bg-background font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"}`}
                  >
                    <Icon
                      className={`size-3.5 ${isActive ? "text-foreground" : "text-muted-foreground"}`}
                    />
                    <span>{item.label}</span>
                  </button>
                )
              })}
            </div>

            <div className="flex flex-col gap-1">
              <span className="px-2 text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                Learn
              </span>
              {learnNavItems.map((item) => {
                const Icon = item.icon
                const isActive = activeTab === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setLearnTab(item.id)}
                    className={`flex h-8 w-full cursor-pointer items-center gap-2 rounded-md px-2.5 text-left text-xs transition-colors ${isActive ? "bg-background font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"}`}
                  >
                    <Icon className="size-3.5" />
                    <span>{item.label}</span>
                  </button>
                )
              })}
            </div>

            <div className="h-2" aria-hidden="true" />

            <div className="flex min-h-0 flex-1 flex-col gap-2">
              <div className="flex items-center justify-between px-2">
                <span className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                  Policies
                </span>
                <span className="font-mono text-[10px] text-muted-foreground/60">
                  {examples.length}
                </span>
              </div>
              <div className="relative px-1">
                <MagnifyingGlass className="pointer-events-none absolute top-1/2 left-2.5 size-3 -translate-y-1/2 text-muted-foreground/60" />
                <Input
                  value={scenarioQuery}
                  onChange={(event) => setScenarioQuery(event.target.value)}
                  placeholder="Find a policy"
                  aria-label="Find a policy"
                  className="h-7 bg-background/70 pl-7 text-[11px]"
                />
              </div>
              <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto">
                {!draftHydrated ? (
                  <span className="px-2 text-[11px] text-muted-foreground">
                    Loading policies…
                  </span>
                ) : examples.length === 0 ? (
                  <span className="px-2 text-[11px] leading-relaxed text-muted-foreground">
                    Connect to load policies.
                  </span>
                ) : visibleExamples.length === 0 ? (
                  <span className="px-2 text-[11px] text-muted-foreground">
                    No matching policies.
                  </span>
                ) : (
                  visibleExamples.map((example) => (
                    <button
                      key={example.id}
                      type="button"
                      onClick={() => handleSelectExample(example)}
                      className={`flex min-h-8 w-full cursor-pointer items-center truncate rounded-md px-2.5 text-left text-xs transition-colors ${selectedExampleId === example.id ? "bg-background font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"}`}
                    >
                      <span className="truncate">{example.name}</span>
                    </button>
                  ))
                )}
              </div>
            </div>
          </div>
        </aside>

        <main className="flex min-w-0 flex-1 flex-col overflow-hidden rounded-2xl bg-background">
          <React.Suspense
            fallback={
              <div
                className="flex h-full items-center justify-center text-sm text-muted-foreground"
                role="status"
              >
                Loading workspace…
              </div>
            }
          >
            {activeTab === "policy" && (
              <PolicyLab
                source={source}
                onSourceChange={handleSourceChange}
                request={request}
                onRequestChange={handleRequestChange}
                validation={validation}
                decision={decision}
                loading={policyLoading}
                error={policyError}
                isDirty={isDirty}
                lastRun={evaluationMeta}
                onValidate={runValidation}
                onEvaluate={handleEvaluate}
                onReset={handleReset}
                onNewPolicy={handleNewPolicy}
                onImport={handleImport}
                onClearResult={handleClearResult}
              />
            )}
            {activeTab === "automata" && (
              <AutomataLab
                pattern={automataPattern}
                onPatternChange={setAutomataPattern}
                input={automataInput}
                onInputChange={setAutomataInput}
                data={automataData}
                loading={automataLoading}
                onAnalyze={handleAnalyzeAutomata}
              />
            )}
            {activeTab === "theory" && <TheoryLab />}
            {activeTab === "compiler" && (
              <CompilerVisualizer source={source} policy={validation?.policy} />
            )}
            {activeTab === "tests" && (
              <TestMatrix source={source} request={request} />
            )}
          </React.Suspense>
        </main>
      </div>

      <CommandPalette
        open={commandPaletteOpen}
        onOpenChange={setCommandPaletteOpen}
        activeTab={activeTab}
        onSelectTab={(tab) => setActiveTab(tab as StudioTab)}
        examples={examples}
        onSelectExample={handleSelectExample}
        onValidate={() => void runValidation()}
        onEvaluate={() => void handleEvaluate()}
        darkMode={darkMode}
        onToggleDarkMode={() => setDarkMode((value) => !value)}
      />
    </div>
  )
}
