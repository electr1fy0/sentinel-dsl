import * as React from "react"
import {
  Copy,
  DotsThree,
  DownloadSimple,
  Play,
  Plus,
  UploadSimple,
} from "@phosphor-icons/react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Textarea } from "@/components/ui/textarea"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
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
import { evaluatePolicyBatch } from "@/lib/api-client"
import type {
  DecisionResponse,
  RequestPayload,
  TestMatrixItem,
} from "@/lib/types"

const STORAGE_KEY = "sentinel:test-matrix:v2"
const SNAPSHOT_KEY = "sentinel:test-matrix:snapshot:v1"

const requestTemplate: RequestPayload = {
  method: "GET",
  path: "/api/users/42",
  host: "api.sentinel.local",
  scheme: "https",
  role: "support",
  headers: {},
  query: {},
  fields: {},
  remote_addr: "192.168.1.1",
  user_agent: "SentinelTest/1.0",
}

interface TestCase extends TestMatrixItem {
  expectedStatus?: number
  expectedDefault?: boolean
}

const DEFAULT_TEST_SUITE: TestCase[] = [
  {
    id: "t1",
    name: "Support Reading User by ID",
    description:
      "Support role reads numeric user endpoint /api/users/42 with GET.",
    request: { ...requestTemplate, headers: { "X-Request-ID": ["test-1"] } },
    expectedDecision: "ALLOW",
  },
  {
    id: "t2",
    name: "Support Deleting User",
    description:
      "Support role attempts DELETE on user resource (should be denied).",
    request: { ...requestTemplate, method: "DELETE", headers: {} },
    expectedDecision: "DENY",
  },
  {
    id: "t3",
    name: "Admin Accessing Admin Metrics",
    description: "Admin role requesting internal telemetry metrics.",
    request: {
      ...requestTemplate,
      path: "/metrics/cpu",
      role: "admin",
      remote_addr: "10.0.0.1",
    },
    expectedDecision: "ALLOW",
  },
  {
    id: "t4",
    name: "Guest Requesting Protected Endpoint",
    description: "Unauthenticated guest requesting a non-public endpoint.",
    request: {
      ...requestTemplate,
      path: "/api/billing/invoices",
      role: "guest",
      remote_addr: "203.0.113.1",
      user_agent: "Mozilla/5.0",
    },
    expectedDecision: "DENY",
  },
  {
    id: "t5",
    name: "Malformed User Path",
    description: "Non-numeric path parameter trying to bypass a path regex.",
    request: {
      ...requestTemplate,
      path: "/api/users/admin-bypass",
      headers: {},
    },
    expectedDecision: "DENY",
  },
  {
    id: "t6",
    name: "Localhost Internal Health Probe",
    description: "Health check from loopback 127.0.0.1.",
    request: {
      ...requestTemplate,
      path: "/healthz",
      host: "localhost",
      scheme: "http",
      role: "internal",
      remote_addr: "127.0.0.1",
      user_agent: "Kubelet/1.29",
    },
    expectedDecision: "ALLOW",
  },
]

interface TestResult {
  test: TestCase
  decision: DecisionResponse | null
  passed: boolean
  durationMs: number
  error?: string
  regressed?: boolean
  idle?: boolean
}

type Filter = "all" | "passed" | "failed" | "regressed"
type SortKey = "status" | "name" | "path" | "expected" | "actual" | "time"

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function loadTests(): TestCase[] {
  if (typeof window === "undefined") return clone(DEFAULT_TEST_SUITE)
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    if (!stored) return clone(DEFAULT_TEST_SUITE)
    const parsed = JSON.parse(stored) as TestCase[]
    return Array.isArray(parsed) && parsed.length > 0
      ? parsed
      : clone(DEFAULT_TEST_SUITE)
  } catch {
    return clone(DEFAULT_TEST_SUITE)
  }
}

function resultSignature(result: {
  decision: DecisionResponse | null
  error?: string
}): string {
  if (result.error) return `error:${result.error}`
  if (!result.decision) return "no-decision"
  return [
    result.decision.decision,
    result.decision.rule ?? "",
    result.decision.default ? "default" : "rule",
    result.decision.status ?? "",
  ].join("|")
}

function download(filename: string, content: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function makeTest(index: number): TestCase {
  return {
    id: `test-${Date.now()}-${index}`,
    name: `New scenario ${index + 1}`,
    description: "Describe what this request should prove.",
    request: clone(requestTemplate),
    expectedDecision: "ALLOW",
  }
}

function SortButton({
  label,
  sortKey,
  sort,
  onSort,
}: {
  label: string
  sortKey: SortKey
  sort: { key: SortKey; direction: "asc" | "desc" }
  onSort: (key: SortKey) => void
}) {
  const active = sort.key === sortKey
  return (
    <button
      type="button"
      aria-label={`Sort by ${label}`}
      aria-sort={
        active
          ? sort.direction === "asc"
            ? "ascending"
            : "descending"
          : "none"
      }
      onClick={() => onSort(sortKey)}
      className="inline-flex items-center gap-1 text-left hover:text-foreground"
    >
      {label}
      <span aria-hidden="true">
        {active ? (sort.direction === "asc" ? "↑" : "↓") : "↕"}
      </span>
    </button>
  )
}

export function TestMatrix({
  source,
  request,
}: {
  source: string
  request?: RequestPayload
}) {
  const [tests, setTests] = React.useState<TestCase[]>(loadTests)
  const [results, setResults] = React.useState<TestResult[]>([])
  const [snapshot, setSnapshot] = React.useState<Record<string, string>>({})
  const [running, setRunning] = React.useState(false)
  const [selectedIds, setSelectedIds] = React.useState<string[]>([])
  const [filter, setFilter] = React.useState<Filter>("all")
  const [search, setSearch] = React.useState("")
  const [editing, setEditing] = React.useState<TestCase | null>(null)
  const [requestJSON, setRequestJSON] = React.useState("")
  const [requestError, setRequestError] = React.useState("")
  const [suiteMessage, setSuiteMessage] = React.useState("")
  const fileInputRef = React.useRef<HTMLInputElement>(null)
  const runToken = React.useRef(0)
  const runController = React.useRef<AbortController | null>(null)
  const [sort, setSort] = React.useState<{
    key: SortKey
    direction: "asc" | "desc"
  }>({ key: "name", direction: "asc" })

  React.useEffect(() => {
    try {
      const stored = window.localStorage.getItem(SNAPSHOT_KEY)
      if (stored) setSnapshot(JSON.parse(stored) as Record<string, string>)
    } catch {
      // Local persistence is a convenience; unavailable storage must not break the matrix.
    }
  }, [])

  React.useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(tests))
    } catch {
      // Ignore private browsing/storage quota failures.
    }
  }, [tests])

  // A result belongs to the source it was run against. Require an explicit rerun
  // after edits instead of silently displaying an answer for stale policy text.
  React.useEffect(() => {
    runController.current?.abort()
    setResults([])
    setFilter("all")
  }, [source])

  const openEditor = (test: TestCase) => {
    setEditing(clone(test))
    setRequestJSON(JSON.stringify(test.request, null, 2))
    setRequestError("")
  }

  const updateEditing = (patch: Partial<TestCase>) => {
    setEditing((current) => (current ? { ...current, ...patch } : current))
  }

  const saveEditing = () => {
    if (!editing || !editing.name.trim()) return
    try {
      const parsed = JSON.parse(requestJSON) as RequestPayload
      if (!parsed.method || !parsed.path) {
        throw new Error("Request JSON needs at least method and path")
      }
      const next = { ...editing, name: editing.name.trim(), request: parsed }
      setTests((current) => {
        const existing = current.some((item) => item.id === next.id)
        return existing
          ? current.map((item) => (item.id === next.id ? next : item))
          : [...current, next]
      })
      setEditing(null)
      setRequestError("")
      setSuiteMessage("Scenario saved")
    } catch (error) {
      setRequestError(
        error instanceof Error ? error.message : "Invalid request JSON"
      )
    }
  }

  const addTest = (fromCurrentRequest = false) => {
    const test = makeTest(tests.length)
    if (fromCurrentRequest && request) test.request = clone(request)
    openEditor(test)
  }

  const duplicateTest = (test: TestCase) => {
    const copy = {
      ...clone(test),
      id: `test-${Date.now()}`,
      name: `${test.name} copy`,
    }
    setTests((current) => [...current, copy])
    setSuiteMessage("Scenario duplicated")
  }

  const deleteTest = (id: string) => {
    setTests((current) => current.filter((test) => test.id !== id))
    setSelectedIds((current) => current.filter((selected) => selected !== id))
    setResults((current) => current.filter((result) => result.test.id !== id))
  }

  const runTests = async (requested: TestCase[]) => {
    if (running || requested.length === 0) return
    const token = ++runToken.current
    const controller = new AbortController()
    runController.current = controller
    setRunning(true)
    setSuiteMessage("")
    const started = performance.now()
    let runResults: TestResult[] = []
    try {
      const response = await evaluatePolicyBatch(
        source,
        requested.map((test) => test.request),
        true,
        { signal: controller.signal }
      )
      const durationMs = Math.round(
        (performance.now() - started) / requested.length
      )
      const policyError = response.valid
        ? undefined
        : response.diagnostics.map((item) => item.message).join("; ") ||
          "Policy is invalid"
      runResults = requested.map((test, index) => {
        const decision = response.results?.[index]?.decision ?? null
        const passed = decision
          ? decision.decision === test.expectedDecision &&
            (test.expectedRule === undefined ||
              decision.rule === test.expectedRule) &&
            (test.expectedStatus === undefined ||
              decision.status === test.expectedStatus) &&
            (test.expectedDefault === undefined ||
              decision.default === test.expectedDefault)
          : false
        return {
          test,
          decision,
          passed,
          durationMs,
          error: policyError,
        }
      })
    } catch (error) {
      if (controller.signal.aborted) {
        setSuiteMessage("Test run cancelled")
        setRunning(false)
        return
      }
      runResults = requested.map((test) => ({
        test,
        decision: null,
        passed: false,
        durationMs: 0,
        error: error instanceof Error ? error.message : "Evaluation failed",
      }))
    }
    if (token !== runToken.current) return
    setResults(
      runResults.map((result) => ({
        ...result,
        regressed:
          Object.prototype.hasOwnProperty.call(snapshot, result.test.id) &&
          snapshot[result.test.id] !== resultSignature(result),
      }))
    )
    setRunning(false)
    runController.current = null
  }

  const runAllTests = () => void runTests(tests)
  const runSelected = () =>
    void runTests(tests.filter((test) => selectedIds.includes(test.id)))

  const cancelRun = () => {
    if (!running) return
    runToken.current += 1
    runController.current?.abort()
    runController.current = null
    setRunning(false)
    setSuiteMessage("Test run cancelled")
  }

  const saveSnapshot = () => {
    const next = Object.fromEntries(
      results.map((result) => [result.test.id, resultSignature(result)])
    )
    setSnapshot(next)
    try {
      window.localStorage.setItem(SNAPSHOT_KEY, JSON.stringify(next))
    } catch {
      /* ignore */
    }
    setResults((current) =>
      current.map((result) => ({ ...result, regressed: false }))
    )
    setSuiteMessage("Regression snapshot saved")
  }

  const exportSuite = () =>
    download(
      "sentinel-test-suite.json",
      JSON.stringify(tests, null, 2),
      "application/json"
    )

  const exportResults = () => {
    if (results.length === 0) return
    download(
      "sentinel-test-results.json",
      JSON.stringify(
        {
          source,
          generatedAt: new Date().toISOString(),
          results: results.map((result) => ({
            id: result.test.id,
            name: result.test.name,
            passed: result.passed,
            durationMs: result.durationMs,
            expected: {
              decision: result.test.expectedDecision,
              rule: result.test.expectedRule,
              status: result.test.expectedStatus,
              default: result.test.expectedDefault,
            },
            actual: result.decision,
            error: result.error,
          })),
        },
        null,
        2
      ),
      "application/json"
    )
  }

  const exportJUnit = () => {
    if (results.length === 0) return
    const xmlEscape = (value: string) =>
      value.replace(
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
    const failures = results
      .map((result) =>
        result.passed
          ? `<testcase name="${xmlEscape(result.test.name)}" time="${(result.durationMs / 1000).toFixed(3)}"/>`
          : `<testcase name="${xmlEscape(result.test.name)}" time="${(result.durationMs / 1000).toFixed(3)}"><failure message="${xmlEscape(result.error ?? `Expected ${result.test.expectedDecision}, got ${result.decision?.decision ?? "no decision"}`)}"/></testcase>`
      )
      .join("")
    download(
      "sentinel-test-results.xml",
      `<?xml version="1.0" encoding="UTF-8"?><testsuite name="Sentinel policy" tests="${results.length}" failures="${failedCount}" time="${(totalDuration / 1000).toFixed(3)}">${failures}</testsuite>`,
      "application/xml"
    )
  }

  const importSuite = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result)) as TestCase[]
        if (
          !Array.isArray(parsed) ||
          parsed.some((item) => !item.id || !item.name)
        )
          throw new Error("Expected an array of test scenarios")
        setTests(parsed)
        setResults([])
        setSuiteMessage(`${parsed.length} scenarios imported`)
      } catch (error) {
        setSuiteMessage(
          error instanceof Error ? error.message : "Could not import suite"
        )
      }
    }
    reader.readAsText(file)
  }

  const passedCount = results.filter((result) => result.passed).length
  const failedCount = results.filter((result) => !result.passed).length
  const regressedCount = results.filter((result) => result.regressed).length
  const totalDuration = results.reduce(
    (sum, result) => sum + result.durationMs,
    0
  )
  const ruleNames = Array.from(
    source.matchAll(/\brule\s+([A-Za-z_][\w-]*)/g),
    (match) => match[1]
  )
  const coveredRules = new Set(
    results
      .map((result) => result.decision?.rule)
      .filter((name): name is string => Boolean(name))
  )
  const resultById = new Map(results.map((result) => [result.test.id, result]))
  const showResultColumns = results.length > 0
  const visibleRows = tests.filter((test) => {
    const result = resultById.get(test.id)
    const match = `${test.name} ${test.description} ${test.request.path}`
      .toLowerCase()
      .includes(search.toLowerCase())
    if (!match) return false
    if (filter === "passed") return result?.passed === true
    if (filter === "failed") return result !== undefined && !result.passed
    if (filter === "regressed") return result?.regressed === true
    return true
  })
  const sortedRows = [...visibleRows].sort((left, right) => {
    const leftResult = resultById.get(left.id)
    const rightResult = resultById.get(right.id)
    const value = (
      test: TestCase,
      result: TestResult | undefined
    ): string | number => {
      if (sort.key === "name") return test.name.toLowerCase()
      if (sort.key === "path") return test.request.path.toLowerCase()
      if (sort.key === "expected") return test.expectedDecision
      if (sort.key === "actual") return result?.decision?.decision ?? ""
      if (sort.key === "time") return result?.durationMs ?? -1
      if (!result) return ""
      return result.passed ? 1 : 0
    }
    const a = value(left, leftResult)
    const b = value(right, rightResult)
    const compared = a < b ? -1 : a > b ? 1 : 0
    return sort.direction === "asc" ? compared : -compared
  })
  const setSortKey = (key: SortKey) =>
    setSort((current) =>
      current.key === key
        ? { key, direction: current.direction === "asc" ? "desc" : "asc" }
        : { key, direction: "asc" }
    )

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden bg-background">
      <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/30 px-4 py-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <div>
            <h1 className="text-sm font-medium text-foreground">Tests</h1>
            <p className="text-[11px] text-muted-foreground">
              Check that policy changes keep expected access decisions
            </p>
          </div>
          <Button
            aria-label="Add test scenario"
            variant="outline"
            size="sm"
            className="h-8 gap-1.5 px-2.5 text-xs"
            onClick={() => addTest()}
          >
            <Plus className="size-3.5" /> Add test
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept="application/json,.json"
            onChange={importSuite}
            className="hidden"
          />
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label="More test actions"
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground hover:text-foreground"
                >
                  <DotsThree className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="start" className="w-48">
              <DropdownMenuItem
                disabled={!request}
                onClick={() => addTest(true)}
              >
                From current request
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => fileInputRef.current?.click()}>
                <UploadSimple className="size-3.5" />
                Import test suite
              </DropdownMenuItem>
              <DropdownMenuItem onClick={exportSuite}>
                <DownloadSimple className="size-3.5" />
                Export test suite
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                disabled={results.length === 0}
                onClick={exportResults}
              >
                Export results as JSON
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={results.length === 0}
                onClick={exportJUnit}
              >
                Export results as JUnit
              </DropdownMenuItem>
              {results.length > 0 && (
                <DropdownMenuItem onClick={saveSnapshot}>
                  Save regression snapshot
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
        <div className="flex items-center gap-2">
          <Input
            aria-label="Search test scenarios"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Find a test"
            className="h-8 w-36 text-xs sm:w-44"
          />
          {selectedIds.length > 0 && (
            <Button
              size="sm"
              variant="outline"
              onClick={runSelected}
              disabled={running}
              className="hidden h-8 gap-1 px-2.5 text-xs sm:flex"
            >
              <Play weight="fill" className="size-2.5" /> Run selected
            </Button>
          )}
          <Button
            size="sm"
            onClick={runAllTests}
            disabled={running || tests.length === 0}
            className="h-8 gap-1.5 px-3 text-xs"
          >
            <Play weight="fill" className="size-2.5" />
            {running ? "Running…" : "Run all"}
          </Button>
          {running && (
            <Button
              size="sm"
              variant="ghost"
              onClick={cancelRun}
              className="h-8 px-2 text-xs text-destructive"
            >
              Cancel
            </Button>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 bg-muted/45 px-4 py-2.5 text-xs text-muted-foreground">
        <div className="flex flex-wrap items-center gap-2.5">
          <span className="font-medium text-foreground">
            {tests.length} {tests.length === 1 ? "test" : "tests"}
          </span>
          <button
            type="button"
            onClick={() => setFilter("all")}
            className={`rounded-full px-2 py-0.5 text-[11px] transition-colors ${filter === "all" ? "bg-background font-medium text-foreground" : "hover:bg-background/70"}`}
          >
            All
          </button>
          <button
            type="button"
            onClick={() => setFilter("passed")}
            className={`rounded-full px-2 py-0.5 text-[11px] transition-colors ${filter === "passed" ? "bg-emerald-500/10 font-medium text-emerald-600 dark:text-emerald-400" : "text-muted-foreground hover:bg-background/70"}`}
          >
            Passed {results.length > 0 ? passedCount : ""}
          </button>
          <button
            type="button"
            onClick={() => setFilter("failed")}
            className={`rounded-full px-2 py-0.5 text-[11px] transition-colors ${filter === "failed" ? "bg-destructive/10 font-medium text-destructive" : "text-muted-foreground hover:bg-background/70"}`}
          >
            Failed {results.length > 0 ? failedCount : ""}
          </button>
          {regressedCount > 0 && (
            <button
              type="button"
              onClick={() => setFilter("regressed")}
              className="text-amber-500"
            >
              Regressed {regressedCount}
            </button>
          )}
          <span className="text-[11px] text-muted-foreground/70">
            {results.length > 0
              ? `${coveredRules.size}/${ruleNames.length || "—"} rules covered`
              : "Not run yet"}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {results.length > 0 && (
            <span className="font-mono text-[11px]">{totalDuration}ms</span>
          )}
        </div>
      </div>

      {suiteMessage && (
        <div
          className="bg-muted/60 px-4 py-1.5 text-[11px] text-muted-foreground"
          role="status"
        >
          {suiteMessage}
        </div>
      )}

      {editing && (
        <Sheet
          open={Boolean(editing)}
          onOpenChange={(open) => {
            if (!open) setEditing(null)
          }}
        >
          <SheetContent
            side="right"
            className="w-full gap-0 overflow-y-auto p-0 sm:max-w-lg"
          >
            <SheetHeader className="bg-muted/45">
              <SheetTitle>
                {tests.some((test) => test.id === editing.id)
                  ? "Edit test"
                  : "New test"}
              </SheetTitle>
              <SheetDescription>
                Define the expected decision for this request.
              </SheetDescription>
            </SheetHeader>
            <div className="flex flex-col gap-4 p-4">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-foreground">
                  Expectations
                </span>
                <span className="text-[11px] text-muted-foreground">
                  Required
                </span>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="flex flex-col gap-1 sm:col-span-2">
                  <Label className="text-[11px] text-muted-foreground">
                    Name
                  </Label>
                  <Input
                    value={editing.name}
                    onChange={(event) =>
                      updateEditing({ name: event.target.value })
                    }
                    className="h-7 text-xs"
                  />
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-[11px] text-muted-foreground">
                    Expected decision
                  </Label>
                  <select
                    aria-label="Expected decision"
                    value={editing.expectedDecision}
                    onChange={(event) =>
                      updateEditing({
                        expectedDecision: event.target.value as
                          "ALLOW" | "DENY",
                      })
                    }
                    className="h-7 rounded-md bg-input/70 px-2 text-xs"
                  >
                    <option>ALLOW</option>
                    <option>DENY</option>
                  </select>
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-[11px] text-muted-foreground">
                    Expected status (optional)
                  </Label>
                  <Input
                    type="number"
                    value={editing.expectedStatus ?? ""}
                    onChange={(event) =>
                      updateEditing({
                        expectedStatus: event.target.value
                          ? Number(event.target.value)
                          : undefined,
                      })
                    }
                    className="h-7 text-xs"
                    placeholder="200"
                  />
                </div>
                <div className="flex flex-col gap-1 sm:col-span-2">
                  <Label className="text-[11px] text-muted-foreground">
                    Expected rule (optional)
                  </Label>
                  <Input
                    value={editing.expectedRule ?? ""}
                    onChange={(event) =>
                      updateEditing({
                        expectedRule: event.target.value || undefined,
                      })
                    }
                    className="h-7 font-mono text-xs"
                    placeholder="ReadUsers"
                  />
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-[11px] text-muted-foreground">
                    Default action?
                  </Label>
                  <select
                    aria-label="Expected default action"
                    value={
                      editing.expectedDefault === undefined
                        ? "any"
                        : String(editing.expectedDefault)
                    }
                    onChange={(event) =>
                      updateEditing({
                        expectedDefault:
                          event.target.value === "any"
                            ? undefined
                            : event.target.value === "true",
                      })
                    }
                    className="h-7 rounded-md bg-input/70 px-2 text-xs"
                  >
                    <option value="any">Any</option>
                    <option value="true">Yes</option>
                    <option value="false">No</option>
                  </select>
                </div>
                <div className="flex flex-col gap-1 sm:col-span-2">
                  <Label className="text-[11px] text-muted-foreground">
                    Description
                  </Label>
                  <Textarea
                    value={editing.description}
                    onChange={(event) =>
                      updateEditing({ description: event.target.value })
                    }
                    className="min-h-12 resize-y text-xs"
                  />
                </div>
                <div className="flex flex-col gap-1 sm:col-span-2">
                  <Label className="text-[11px] text-muted-foreground">
                    Request JSON
                  </Label>
                  <Textarea
                    value={requestJSON}
                    onChange={(event) => {
                      setRequestJSON(event.target.value)
                      setRequestError("")
                    }}
                    className="min-h-36 resize-y font-mono text-[11px]"
                    spellCheck={false}
                  />
                  {requestError && (
                    <span className="text-[11px] text-destructive" role="alert">
                      {requestError}
                    </span>
                  )}
                </div>
              </div>
            </div>
            <SheetFooter className="bg-muted/45">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setEditing(null)}
              >
                Cancel
              </Button>
              <Button size="sm" onClick={saveEditing}>
                Save test
              </Button>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      )}

      <ScrollArea className="flex-1 p-4">
        <div className="rounded-xl bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-9">
                  <input
                    aria-label="Select all visible tests"
                    type="checkbox"
                    checked={
                      visibleRows.length > 0 &&
                      visibleRows.every((test) => selectedIds.includes(test.id))
                    }
                    onChange={(event) =>
                      setSelectedIds(
                        event.target.checked
                          ? visibleRows.map((test) => test.id)
                          : []
                      )
                    }
                  />
                </TableHead>
                <TableHead className="w-20">
                  <SortButton
                    label="Status"
                    sortKey="status"
                    sort={sort}
                    onSort={setSortKey}
                  />
                </TableHead>
                <TableHead>
                  <SortButton
                    label="Scenario"
                    sortKey="name"
                    sort={sort}
                    onSort={setSortKey}
                  />
                </TableHead>
                <TableHead className="w-48">
                  <SortButton
                    label="Request target"
                    sortKey="path"
                    sort={sort}
                    onSort={setSortKey}
                  />
                </TableHead>
                <TableHead className="w-24">
                  <SortButton
                    label="Expected"
                    sortKey="expected"
                    sort={sort}
                    onSort={setSortKey}
                  />
                </TableHead>
                {showResultColumns && (
                  <>
                    <TableHead className="w-24">
                      <SortButton
                        label="Actual"
                        sortKey="actual"
                        sort={sort}
                        onSort={setSortKey}
                      />
                    </TableHead>
                    <TableHead className="w-32">Matched rule</TableHead>
                    <TableHead className="w-16 text-right">
                      <SortButton
                        label="Time"
                        sortKey="time"
                        sort={sort}
                        onSort={setSortKey}
                      />
                    </TableHead>
                  </>
                )}
                <TableHead className="w-28 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedRows.map((test) => {
                const result = resultById.get(test.id) ?? {
                  test,
                  decision: null,
                  passed: false,
                  durationMs: 0,
                  idle: true,
                }
                return (
                  <React.Fragment key={result.test.id}>
                    <TableRow
                      className={
                        result.regressed ? "bg-amber-500/5" : undefined
                      }
                    >
                      <TableCell>
                        <input
                          aria-label={`Select ${result.test.name}`}
                          type="checkbox"
                          checked={selectedIds.includes(result.test.id)}
                          onChange={(event) =>
                            setSelectedIds((current) =>
                              event.target.checked
                                ? [...current, result.test.id]
                                : current.filter((id) => id !== result.test.id)
                            )
                          }
                        />
                      </TableCell>
                      <TableCell>
                        {result.idle ? (
                          <span className="text-[11px] text-muted-foreground">
                            Not run
                          </span>
                        ) : (
                          <div className="flex items-center gap-1.5 font-mono text-[10px]">
                            <span
                              className={`size-1.5 rounded-full ${result.passed ? "bg-emerald-500" : "bg-destructive"}`}
                            />
                            <span
                              className={
                                result.passed
                                  ? "text-emerald-500"
                                  : "text-destructive"
                              }
                            >
                              {result.passed ? "PASS" : "FAIL"}
                            </span>
                            {result.regressed && (
                              <span className="text-amber-500">REG</span>
                            )}
                          </div>
                        )}
                      </TableCell>
                      <TableCell>
                        <button
                          type="button"
                          onClick={() => openEditor(result.test)}
                          className="flex flex-col text-left"
                        >
                          <span className="text-xs font-medium text-foreground hover:underline">
                            {result.test.name}
                          </span>
                          <span className="line-clamp-1 text-[11px] text-muted-foreground">
                            {result.error || result.test.description}
                          </span>
                        </button>
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        <span className="mr-1.5 font-medium text-foreground">
                          {result.test.request.method}
                        </span>
                        <span className="text-muted-foreground">
                          {result.test.request.path}
                        </span>
                        <span className="block text-[10px] text-muted-foreground/70">
                          role: {result.test.request.role || "-"}
                        </span>
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        <span
                          className={
                            result.test.expectedDecision === "ALLOW"
                              ? "text-emerald-500"
                              : "text-muted-foreground"
                          }
                        >
                          {result.test.expectedDecision}
                        </span>
                        {result.test.expectedStatus && (
                          <span className="block text-[10px] text-muted-foreground">
                            {result.test.expectedStatus}
                          </span>
                        )}
                      </TableCell>
                      {showResultColumns && (
                        <>
                          <TableCell className="font-mono text-xs">
                            {result.decision ? (
                              <span
                                className={
                                  result.decision.decision === "ALLOW"
                                    ? "text-emerald-500"
                                    : "text-muted-foreground"
                                }
                              >
                                {result.decision.decision}
                              </span>
                            ) : (
                              <span className="text-muted-foreground/60">
                                —
                              </span>
                            )}
                          </TableCell>
                          <TableCell className="font-mono text-xs text-muted-foreground">
                            {result.decision?.default
                              ? "default"
                              : result.decision?.rule || "—"}
                          </TableCell>
                          <TableCell className="text-right font-mono text-xs text-muted-foreground">
                            {result.durationMs}ms
                          </TableCell>
                        </>
                      )}
                      <TableCell>
                        <div className="flex justify-end">
                          <DropdownMenu>
                            <DropdownMenuTrigger
                              render={
                                <Button
                                  aria-label={`Actions for ${result.test.name}`}
                                  variant="ghost"
                                  size="icon-sm"
                                  className="text-muted-foreground hover:text-foreground"
                                >
                                  <DotsThree className="size-4" />
                                </Button>
                              }
                            />
                            <DropdownMenuContent align="end" className="w-40">
                              <DropdownMenuItem
                                disabled={running}
                                onClick={() => void runTests([result.test])}
                              >
                                <Play className="size-3.5" /> Run test
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={() => duplicateTest(result.test)}
                              >
                                <Copy className="size-3.5" /> Duplicate
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                variant="destructive"
                                onClick={() => deleteTest(result.test.id)}
                              >
                                Delete test
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </TableCell>
                    </TableRow>
                    {!result.idle &&
                    (result.error || result.decision?.trace?.length) ? (
                      <TableRow>
                        <TableCell
                          colSpan={showResultColumns ? 9 : 6}
                          className="bg-muted/10 py-2"
                        >
                          <details>
                            <summary className="cursor-pointer text-[11px] text-muted-foreground">
                              Inspect{" "}
                              {result.error
                                ? "failure details"
                                : "evaluation trace"}
                            </summary>
                            <div className="mt-2 flex flex-col gap-1 font-mono text-[11px]">
                              {result.error && (
                                <p className="text-destructive">
                                  {result.error}
                                </p>
                              )}
                              {result.decision?.trace?.map((trace, index) => (
                                <div
                                  key={`${trace.rule}-${index}`}
                                  className="rounded-lg bg-muted/55 px-2 py-1"
                                >
                                  <span
                                    className={
                                      trace.matched
                                        ? "text-emerald-500"
                                        : "text-muted-foreground"
                                    }
                                  >
                                    {trace.matched ? "✓" : "○"} {trace.rule}
                                  </span>
                                  <span className="ml-2 text-muted-foreground">
                                    {trace.detail}
                                  </span>
                                </div>
                              ))}
                            </div>
                          </details>
                        </TableCell>
                      </TableRow>
                    ) : null}
                  </React.Fragment>
                )
              })}
              {visibleRows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={showResultColumns ? 9 : 6}
                    className="h-28 text-center text-xs text-muted-foreground"
                  >
                    {results.length === 0
                      ? "No run yet. Run the suite to compare actual decisions."
                      : "No scenarios match this filter."}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
        {results.length > 0 && (
          <div className="mt-3 rounded-xl bg-muted/45 p-3 text-[11px] text-muted-foreground">
            <span className="font-medium text-foreground">Coverage</span>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {ruleNames.length ? (
                ruleNames.map((name) => (
                  <span
                    key={name}
                    className={`rounded-lg px-2 py-1 font-mono ${coveredRules.has(name) ? "bg-emerald-500/10 text-emerald-500" : "bg-amber-500/10 text-amber-500"}`}
                  >
                    {coveredRules.has(name) ? "✓" : "○"} {name}
                  </span>
                ))
              ) : (
                <span>
                  No rule declarations detected in the current source.
                </span>
              )}
            </div>
          </div>
        )}
      </ScrollArea>
    </div>
  )
}
