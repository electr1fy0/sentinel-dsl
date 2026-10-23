import * as React from "react"
import {
  CaretDown,
  DotsThree,
  Play,
  Check,
  Copy,
  Plus,
  X,
  ArrowClockwise,
} from "@phosphor-icons/react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { formatPolicy, PolicyEditor } from "@/components/studio/policy-editor"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable"
import type {
  ASTNode,
  DecisionResponse,
  PolicySummary,
  RequestPayload,
  ValidateResponse,
} from "@/lib/types"

function shellTokens(command: string): string[] {
  const tokens: string[] = []
  const pattern = /"([^"\\]*(?:\\.[^"\\]*)*)"|'([^']*)'|([^\s]+)/g
  let match: RegExpExecArray | null
  while ((match = pattern.exec(command)) !== null) {
    tokens.push(match[1] || match[2] || match[3] || "")
  }
  return tokens
}

function requestToCurl(request: RequestPayload): string {
  const origin = `${request.scheme || "https"}://${request.host || "localhost"}`
  const query = new URLSearchParams(request.query).toString()
  const url = `${origin}${request.path || "/"}${query ? `?${query}` : ""}`
  const parts = ["curl", "-X", request.method || "GET", shellQuote(url)]
  Object.entries(request.headers).forEach(([key, values]) => {
    values.forEach((value) => parts.push("-H", shellQuote(`${key}: ${value}`)))
  })
  if (request.user_agent) parts.push("-A", shellQuote(request.user_agent))
  return parts.join(" ")
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'\\''`)}'`
}

function parseCurl(command: string): RequestPayload {
  const tokens = shellTokens(command.trim())
  if (tokens[0]?.toLowerCase() !== "curl")
    throw new Error("Command must start with curl.")
  let method = "GET"
  let userAgent = ""
  const headers: Record<string, string[]> = {}
  let urlValue = ""
  for (let index = 1; index < tokens.length; index += 1) {
    const token = tokens[index]
    if (!token) continue
    if (token === "-X" || token === "--request") {
      method = tokens[++index] ?? method
    } else if (token === "-H" || token === "--header") {
      const header = tokens[++index] ?? ""
      const separator = header.indexOf(":")
      if (separator > 0) {
        const key = header.slice(0, separator).trim()
        const value = header.slice(separator + 1).trim()
        if (key) headers[key] = [...(headers[key] ?? []), value]
      }
    } else if (token === "-A" || token === "--user-agent") {
      userAgent = tokens[++index] ?? ""
    } else if (!token.startsWith("-")) {
      urlValue = token
    }
  }
  if (!urlValue) throw new Error("cURL command does not include a URL.")
  let parsed: URL
  try {
    parsed = new URL(urlValue)
  } catch {
    parsed = new URL(urlValue, "https://localhost")
  }
  const query: Record<string, string> = {}
  parsed.searchParams.forEach((value, key) => {
    query[key] = value
  })
  return {
    method: method.toUpperCase(),
    path: `${parsed.pathname || "/"}`,
    host: parsed.host,
    scheme: parsed.protocol.replace(":", "") || "https",
    remote_addr: "",
    user_agent: userAgent,
    role: "",
    headers,
    query,
    fields: {},
  }
}

export interface ImportedPolicy {
  source: string
  request: RequestPayload
}

interface PolicyLabProps {
  source: string
  onSourceChange: (source: string) => void
  request: RequestPayload
  onRequestChange: (req: RequestPayload) => void
  validation: ValidateResponse | null
  decision: DecisionResponse | null
  loading: boolean
  error?: string
  isDirty: boolean
  lastRun?: { durationMs: number; completedAt: number }
  onValidate: () => void
  onEvaluate: () => void
  onReset: () => void
  onNewPolicy: () => void
  onImport: (policy: ImportedPolicy) => void
  onClearResult: () => void
}

export function PolicyLab({
  source,
  onSourceChange,
  request,
  onRequestChange,
  validation,
  decision,
  loading,
  error,
  isDirty,
  lastRun,
  onValidate,
  onEvaluate,
  onReset,
  onNewPolicy,
  onImport,
  onClearResult,
}: PolicyLabProps) {
  const isMobile = useIsMobile()
  const [copied, setCopied] = React.useState<string | null>(null)
  const [headerKey, setHeaderKey] = React.useState("")
  const [headerVal, setHeaderVal] = React.useState("")
  const [queryKey, setQueryKey] = React.useState("")
  const [queryVal, setQueryVal] = React.useState("")
  const [fieldKey, setFieldKey] = React.useState("")
  const [fieldVal, setFieldVal] = React.useState("")
  const [requestJSON, setRequestJSON] = React.useState(() =>
    JSON.stringify(request, null, 2)
  )
  const [jsonError, setJSONError] = React.useState<string>()
  const [entryError, setEntryError] = React.useState<string>()
  const [requestHistory, setRequestHistory] = React.useState<RequestPayload[]>(
    []
  )
  const [advancedRequestOpen, setAdvancedRequestOpen] = React.useState(false)
  const [showTrace, setShowTrace] = React.useState(false)
  const [mobilePane, setMobilePane] = React.useState<
    "editor" | "request" | "result"
  >("editor")
  const importInputRef = React.useRef<HTMLInputElement>(null)
  const editorRef = React.useRef<HTMLTextAreaElement>(null)

  React.useEffect(() => {
    try {
      const stored = window.localStorage.getItem("sentinel:request-history")
      if (stored) setRequestHistory(JSON.parse(stored) as RequestPayload[])
    } catch {
      setRequestHistory([])
    }
  }, [])

  React.useEffect(() => {
    setRequestJSON(JSON.stringify(request, null, 2))
    setJSONError(undefined)
  }, [request])

  React.useEffect(() => {
    setShowTrace(false)
  }, [decision])

  const copyText = (value: string, label: string) => {
    void navigator.clipboard.writeText(value).then(
      () => {
        setCopied(label)
        window.setTimeout(() => setCopied(null), 2000)
      },
      () => setCopied(null)
    )
  }

  const handleCopyJSON = () => {
    copyText(
      JSON.stringify(
        {
          source,
          request,
          decision,
          validation,
        },
        null,
        2
      ),
      "workspace"
    )
  }

  const copyDecision = () => {
    if (!decision) return
    copyText(JSON.stringify(decision, null, 2), "decision")
  }

  const updateMapValue = (
    map: "headers" | "query" | "fields",
    key: string,
    value: string
  ) => {
    if (map === "headers") {
      onRequestChange({
        ...request,
        headers: { ...request.headers, [key]: [value] },
      })
    } else if (map === "query") {
      onRequestChange({ ...request, query: { ...request.query, [key]: value } })
    } else {
      onRequestChange({
        ...request,
        fields: { ...request.fields, [key]: value },
      })
    }
  }

  const updateHeaderValue = (key: string, index: number, value: string) => {
    const values = [...(request.headers[key] ?? [])]
    values[index] = value
    onRequestChange({
      ...request,
      headers: { ...request.headers, [key]: values },
    })
  }

  const appendHeaderValue = (key: string) => {
    const values = [...(request.headers[key] ?? []), ""]
    onRequestChange({
      ...request,
      headers: { ...request.headers, [key]: values },
    })
  }

  const saveRequestPreset = () => {
    const next = [
      request,
      ...requestHistory.filter(
        (item) => JSON.stringify(item) !== JSON.stringify(request)
      ),
    ].slice(0, 8)
    setRequestHistory(next)
    try {
      window.localStorage.setItem(
        "sentinel:request-history",
        JSON.stringify(next)
      )
    } catch {
      // Keep the current request usable when browser storage is unavailable.
    }
  }

  const loadRequestPreset = (index: string) => {
    if (index === "") return
    const presetIndex = Number(index)
    if (presetIndex < 0 || presetIndex >= requestHistory.length) return
    onRequestChange(requestHistory[presetIndex])
  }

  const addEntry = (
    map: "headers" | "query" | "fields",
    key: string,
    value: string
  ) => {
    const trimmedKey = key.trim()
    if (!trimmedKey) {
      setEntryError("A key is required.")
      return false
    }
    if (!value.trim()) {
      setEntryError("A value is required.")
      return false
    }
    const values =
      map === "headers"
        ? request.headers
        : map === "query"
          ? request.query
          : request.fields
    if (
      Object.keys(values).some(
        (existing) => existing.toLowerCase() === trimmedKey.toLowerCase()
      )
    ) {
      setEntryError(`The ${map.slice(0, -1)} key already exists.`)
      return false
    }
    if (map === "headers")
      onRequestChange({
        ...request,
        headers: { ...request.headers, [trimmedKey]: [value.trim()] },
      })
    if (map === "query")
      onRequestChange({
        ...request,
        query: { ...request.query, [trimmedKey]: value.trim() },
      })
    if (map === "fields")
      onRequestChange({
        ...request,
        fields: { ...request.fields, [trimmedKey]: value.trim() },
      })
    setEntryError(undefined)
    return true
  }

  const addHeader = () => {
    if (addEntry("headers", headerKey, headerVal)) {
      setHeaderKey("")
      setHeaderVal("")
    }
  }

  const removeHeader = (key: string) => {
    const updated = { ...request.headers }
    delete updated[key]
    onRequestChange({ ...request, headers: updated })
  }

  const addQuery = () => {
    if (addEntry("query", queryKey, queryVal)) {
      setQueryKey("")
      setQueryVal("")
    }
  }

  const removeQuery = (key: string) => {
    const updated = { ...request.query }
    delete updated[key]
    onRequestChange({ ...request, query: updated })
  }

  const addField = () => {
    if (addEntry("fields", fieldKey, fieldVal)) {
      setFieldKey("")
      setFieldVal("")
    }
  }

  const removeField = (key: string) => {
    const updated = { ...request.fields }
    delete updated[key]
    onRequestChange({ ...request, fields: updated })
  }

  const applyJSON = () => {
    try {
      const parsed = JSON.parse(requestJSON) as Partial<RequestPayload>
      if (
        typeof parsed !== "object" ||
        typeof parsed.method !== "string" ||
        typeof parsed.path !== "string"
      ) {
        throw new Error(
          "Request JSON must include at least string method and path fields."
        )
      }
      onRequestChange({
        method: parsed.method,
        path: parsed.path,
        host: parsed.host ?? "",
        scheme: parsed.scheme ?? "https",
        remote_addr: parsed.remote_addr ?? "",
        user_agent: parsed.user_agent ?? "",
        role: parsed.role ?? "",
        headers: parsed.headers ?? {},
        query: parsed.query ?? {},
        fields: parsed.fields ?? {},
      })
      setJSONError(undefined)
    } catch (cause) {
      setJSONError(
        cause instanceof Error ? cause.message : "Invalid request JSON"
      )
    }
  }

  const curl = React.useMemo(() => requestToCurl(request), [request])

  const importCurl = () => {
    const value = window.prompt("Paste a cURL command")
    if (!value) return
    try {
      onRequestChange(parseCurl(value))
      setEntryError(undefined)
    } catch (cause) {
      setEntryError(
        cause instanceof Error ? cause.message : "Could not parse cURL command."
      )
    }
  }

  const importFile = async (file: File) => {
    try {
      const text = await file.text()
      if (file.name.toLowerCase().endsWith(".json")) {
        const parsed = JSON.parse(text) as Partial<ImportedPolicy>
        if (typeof parsed.source !== "string" || !parsed.request)
          throw new Error("JSON export must include source and request.")
        onImport({ source: parsed.source, request: parsed.request })
      } else {
        onImport({ source: text, request })
      }
    } catch (cause) {
      setEntryError(
        cause instanceof Error ? cause.message : "Could not import policy file."
      )
    } finally {
      if (importInputRef.current) importInputRef.current.value = ""
    }
  }

  const exportWorkspace = () => {
    const blob = new Blob([JSON.stringify({ source, request }, null, 2)], {
      type: "application/json",
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = url
    anchor.download = "sentinel-policy.json"
    anchor.click()
    URL.revokeObjectURL(url)
  }

  const downloadSource = () => {
    const blob = new Blob([source], { type: "text/plain" })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = url
    anchor.download = "policy.sntl"
    anchor.click()
    URL.revokeObjectURL(url)
  }

  const addRule = () => {
    const rule = `  rule CustomRule when\n    method == "GET"\n    and path matches "/api/.*"\n  then allow;`
    const closingBrace = source.lastIndexOf("}")
    const insertion = closingBrace >= 0 ? `\n\n${rule}\n` : `\n\n${rule}`
    const nextSource =
      closingBrace >= 0
        ? `${source.slice(0, closingBrace)}${insertion}${source.slice(closingBrace)}`
        : `${source}${insertion}`
    onSourceChange(nextSource)
  }

  const selectDiagnostic = (offset: number) => {
    editorRef.current?.focus()
    editorRef.current?.setSelectionRange(offset, offset)
  }

  const copyTrace = () => {
    if (!decision?.trace) return
    copyText(JSON.stringify(decision.trace, null, 2), "trace")
  }

  const lineCount = source.split("\n").length

  const handleRunPolicy = () => {
    if (isMobile) setMobilePane("result")
    onEvaluate()
  }

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden bg-background">
      {/* Keep the document chrome quiet: one primary action, everything else in More. */}
      <div className="flex min-h-11 items-center justify-between gap-3 bg-muted/30 px-4">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="font-mono text-xs text-foreground">policy.sntl</span>
          <span className="hidden text-[11px] text-muted-foreground sm:inline">
            {lineCount} {lineCount === 1 ? "line" : "lines"}
          </span>
          <span
            className={`hidden text-[11px] sm:inline ${isDirty ? "text-amber-500" : "text-muted-foreground"}`}
          >
            {isDirty ? "Unsaved changes" : "Saved locally"}
          </span>
          {validation && (
            <span
              className={`flex items-center gap-1.5 text-[11px] ${validation.valid ? "text-emerald-500" : "text-destructive"}`}
            >
              <span className="size-1.5 rounded-full bg-current" />
              {validation.valid
                ? "Ready"
                : `${validation.diagnostics.length} ${validation.diagnostics.length === 1 ? "issue" : "issues"}`}
            </span>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1.5">
          <input
            ref={importInputRef}
            type="file"
            accept=".sntl,.txt,.json"
            className="hidden"
            onChange={(event) => {
              const file = event.target.files?.[0]
              if (file) void importFile(file)
            }}
          />
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground hover:text-foreground"
                  aria-label="More policy actions"
                >
                  <DotsThree className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-48">
              <DropdownMenuItem onClick={onNewPolicy}>
                New policy
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => importInputRef.current?.click()}>
                Import policy
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={exportWorkspace}>
                Export workspace
              </DropdownMenuItem>
              <DropdownMenuItem onClick={downloadSource}>
                Download .sntl
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => onSourceChange(formatPolicy(source))}
              >
                Format source
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onValidate}>
                Validate source
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onReset}>
                <ArrowClockwise className="size-3.5" />
                Reset policy
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            size="sm"
            onClick={handleRunPolicy}
            disabled={loading}
            className="h-8 gap-1.5 px-3 text-xs font-medium"
          >
            <Play weight="fill" className="size-2.5" />
            <span>{loading ? "Running…" : "Run policy"}</span>
            <kbd className="hidden text-[10px] opacity-70 sm:inline">⌘↵</kbd>
          </Button>
        </div>
      </div>

      {error && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-2 bg-destructive/10 px-4 py-2 text-xs text-destructive"
        >
          <span>{error}</span>
          <span className="text-[11px] text-destructive/75">
            Check your connection, then run again.
          </span>
        </div>
      )}

      {isMobile && (
        <nav
          aria-label="Policy sections"
          className="flex shrink-0 gap-1 bg-muted/35 px-3 py-1.5"
        >
          {(
            [
              ["editor", "Policy"],
              ["request", "Request"],
              ["result", "Result"],
            ] as const
          ).map(([pane, label]) => (
            <button
              key={pane}
              type="button"
              onClick={() => setMobilePane(pane)}
              aria-current={mobilePane === pane ? "page" : undefined}
              className={`h-7 rounded-md px-3 text-[11px] ${mobilePane === pane ? "bg-background font-medium text-foreground" : "text-muted-foreground"}`}
            >
              {label}
            </button>
          ))}
        </nav>
      )}

      {/* Two focused panes keep the policy loop visible at a glance. */}
      <ResizablePanelGroup
        orientation={isMobile ? "vertical" : "horizontal"}
        className="h-full min-h-0 w-full flex-1 gap-2 overflow-hidden bg-muted/55 p-2"
      >
        {/* PANEL 1: Sentinel Policy Source Editor */}
        <ResizablePanel
          defaultSize={isMobile ? "100%" : "58%"}
          minSize={isMobile ? "100%" : "34%"}
          className={`flex min-w-0 flex-col overflow-hidden rounded-xl bg-background ${isMobile && mobilePane !== "editor" ? "hidden" : ""}`}
        >
          <div className="flex h-9 items-center justify-between bg-muted/60 px-3.5">
            <div>
              <span className="text-xs font-medium text-foreground">
                Policy
              </span>
              <span className="ml-2 text-[11px] text-muted-foreground">
                Write the rules that protect your request
              </span>
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-1.5 text-[11px] text-muted-foreground hover:text-foreground"
              onClick={addRule}
            >
              <Plus className="size-3" />
              Add rule
            </Button>
          </div>

          <div className="relative flex flex-1 overflow-hidden">
            {/* Line numbers gutter */}
            <div className="w-10 bg-muted/30 py-4 pr-2.5 text-right font-mono text-[13px] leading-[22px] text-muted-foreground/40 select-none">
              {Array.from({ length: lineCount }).map((_, i) => (
                <div key={i}>{i + 1}</div>
              ))}
            </div>

            {/* Editor Textarea */}
            <PolicyEditor
              ref={editorRef}
              value={source}
              onChange={onSourceChange}
              className="min-h-0 flex-1 rounded-none bg-transparent"
              placeholder="policy MyPolicy { ... }"
            />
          </div>

          {/* Diagnostic Alerts Footer */}
          {validation && validation.diagnostics.length > 0 && (
            <div className="max-h-32 overflow-y-auto bg-destructive/10 p-2 font-mono text-xs">
              {validation.diagnostics.map((d, i) => (
                <div
                  key={i}
                  role="button"
                  tabIndex={0}
                  onClick={() => selectDiagnostic(d.start.offset)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ")
                      selectDiagnostic(d.start.offset)
                  }}
                  className="mb-1 cursor-pointer text-[11px] text-destructive last:mb-0 hover:underline"
                >
                  <span className="font-semibold">
                    Line {d.start.line}:{d.start.column}
                  </span>
                  : {d.message}
                </div>
              ))}
            </div>
          )}
        </ResizablePanel>

        <ResizableHandle
          withHandle
          className={isMobile ? "hidden" : undefined}
        />

        {/* PANEL 2: Request inspector and live result */}
        <ResizablePanel
          defaultSize={isMobile ? "100%" : "42%"}
          minSize={isMobile ? "100%" : "30%"}
          className={`flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-background ${isMobile && mobilePane === "editor" ? "hidden" : ""}`}
        >
          <ResizablePanelGroup
            orientation="vertical"
            className="min-h-0 w-full flex-1 gap-2 overflow-hidden bg-muted/55 p-2"
          >
            <ResizablePanel
              defaultSize={isMobile ? "100%" : "58%"}
              minSize={isMobile ? "100%" : "32%"}
              className={`flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-muted/15 ${isMobile && mobilePane !== "request" ? "hidden" : ""}`}
            >
              <Tabs
                defaultValue="request"
                className="flex flex-1 flex-col overflow-hidden"
              >
                <div className="flex h-9 items-center justify-between bg-muted/60 px-3.5">
                  <TabsList className="h-7 gap-1 rounded-md bg-muted/50 p-0.5">
                    <TabsTrigger
                      value="request"
                      className="h-6 rounded px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                    >
                      Request
                    </TabsTrigger>
                    <TabsTrigger
                      value="ast"
                      className="h-6 rounded px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                    >
                      AST
                    </TabsTrigger>
                    <TabsTrigger
                      value="json"
                      className="h-6 rounded px-2.5 text-xs font-normal data-[state=active]:bg-background data-[state=active]:font-medium"
                    >
                      Raw JSON
                    </TabsTrigger>
                  </TabsList>
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-muted-foreground hover:text-foreground"
                          aria-label="More request actions"
                        >
                          <DotsThree className="size-4" />
                        </Button>
                      }
                    />
                    <DropdownMenuContent align="end" className="w-44">
                      <DropdownMenuItem onClick={() => copyText(curl, "curl")}>
                        {copied === "curl" ? "cURL copied" : "Copy cURL"}
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={importCurl}>
                        Import cURL
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {/* TAB: HTTP Request Builder */}
                <TabsContent
                  value="request"
                  className="m-0 flex flex-1 flex-col overflow-hidden"
                >
                  <div className="flex items-center justify-between gap-2 bg-background/60 px-3.5 py-2">
                    <div>
                      <span className="text-xs font-medium text-foreground">
                        Test request
                      </span>
                      <span className="ml-2 hidden text-[11px] text-muted-foreground lg:inline">
                        See how this policy responds
                      </span>
                    </div>
                    <div className="flex min-w-0 items-center gap-1">
                      <select
                        defaultValue=""
                        onChange={(event) =>
                          loadRequestPreset(event.target.value)
                        }
                        className="h-7 max-w-28 truncate rounded-md bg-muted/70 px-2 text-[11px] text-muted-foreground outline-none focus:bg-accent"
                        aria-label="Saved request history"
                      >
                        <option value="">History</option>
                        {requestHistory.map((item, index) => (
                          <option
                            key={`${item.method}-${item.path}-${index}`}
                            value={index}
                          >
                            {item.method} {item.path || "/"}
                          </option>
                        ))}
                      </select>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={saveRequestPreset}
                        className="h-7 px-2 text-[11px] text-muted-foreground"
                      >
                        Save
                      </Button>
                    </div>
                  </div>
                  <ScrollArea className="flex-1 p-3">
                    {(entryError || jsonError) && (
                      <div
                        role="alert"
                        className="mb-3 rounded bg-destructive/10 p-2 text-[11px] text-destructive"
                      >
                        {entryError || jsonError}
                      </div>
                    )}
                    <div className="flex flex-col gap-3">
                      {/* Common request fields */}
                      <div className="grid grid-cols-[84px_1fr] gap-2 rounded-xl bg-card p-3">
                        <div className="flex flex-col gap-1">
                          <Label className="text-[11px] text-muted-foreground">
                            Method
                          </Label>
                          <Select
                            value={request.method}
                            onValueChange={(val) =>
                              onRequestChange({
                                ...request,
                                method: val ?? "GET",
                              })
                            }
                          >
                            <SelectTrigger className="h-8 font-mono text-xs">
                              <SelectValue placeholder="Method" />
                            </SelectTrigger>
                            <SelectContent>
                              {[
                                "GET",
                                "POST",
                                "PUT",
                                "DELETE",
                                "PATCH",
                                "HEAD",
                                "OPTIONS",
                              ].map((m) => (
                                <SelectItem
                                  key={m}
                                  value={m}
                                  className="font-mono text-xs"
                                >
                                  {m}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        <div className="flex flex-col gap-1">
                          <Label className="text-[11px] text-muted-foreground">
                            Path
                          </Label>
                          <Input
                            value={request.path}
                            onChange={(e) =>
                              onRequestChange({
                                ...request,
                                path: e.target.value,
                              })
                            }
                            className="h-8 font-mono text-xs"
                            placeholder="/api/users/42"
                          />
                        </div>
                      </div>

                      {/* Role & Host */}
                      <div className="grid grid-cols-2 gap-2">
                        <div className="flex flex-col gap-1">
                          <Label className="text-[11px] text-muted-foreground">
                            Role
                          </Label>
                          <Input
                            value={request.role}
                            onChange={(e) =>
                              onRequestChange({
                                ...request,
                                role: e.target.value,
                              })
                            }
                            className="h-8 font-mono text-xs"
                            placeholder="admin / support"
                          />
                        </div>
                        <div className="flex flex-col gap-1">
                          <Label className="text-[11px] text-muted-foreground">
                            Host
                          </Label>
                          <Input
                            value={request.host}
                            onChange={(e) =>
                              onRequestChange({
                                ...request,
                                host: e.target.value,
                              })
                            }
                            className="h-8 font-mono text-xs"
                            placeholder="api.sentinel.local"
                          />
                        </div>
                      </div>

                      <button
                        type="button"
                        aria-expanded={advancedRequestOpen}
                        onClick={() => setAdvancedRequestOpen((open) => !open)}
                        className="flex items-center justify-between rounded-md px-1 py-2 text-left text-xs text-muted-foreground transition-colors hover:text-foreground"
                      >
                        <span>
                          Advanced request fields
                          <span className="ml-2 text-[10px] text-muted-foreground/70">
                            {Object.keys(request.headers).length +
                              Object.keys(request.query).length +
                              Object.keys(request.fields).length}{" "}
                            configured
                          </span>
                        </span>
                        <CaretDown
                          className={`size-3.5 transition-transform ${advancedRequestOpen ? "rotate-180" : ""}`}
                        />
                      </button>

                      {advancedRequestOpen && (
                        <div className="flex flex-col gap-3 rounded-lg bg-muted/25 p-3">
                          {/* Scheme & Remote IP */}
                          <div className="grid grid-cols-2 gap-2">
                            <div className="flex flex-col gap-1">
                              <Label className="text-[11px] text-muted-foreground">
                                Scheme
                              </Label>
                              <Input
                                value={request.scheme}
                                onChange={(e) =>
                                  onRequestChange({
                                    ...request,
                                    scheme: e.target.value,
                                  })
                                }
                                className="h-7 font-mono text-xs"
                                placeholder="https"
                              />
                            </div>
                            <div className="flex flex-col gap-1">
                              <Label className="text-[11px] text-muted-foreground">
                                Remote IP
                              </Label>
                              <Input
                                value={request.remote_addr}
                                onChange={(e) =>
                                  onRequestChange({
                                    ...request,
                                    remote_addr: e.target.value,
                                  })
                                }
                                className="h-7 font-mono text-xs"
                                placeholder="192.168.1.1"
                              />
                            </div>
                          </div>

                          {/* Client identity */}
                          <div className="flex flex-col gap-1">
                            <Label className="text-[11px] text-muted-foreground">
                              User agent
                            </Label>
                            <Input
                              value={request.user_agent}
                              onChange={(e) =>
                                onRequestChange({
                                  ...request,
                                  user_agent: e.target.value,
                                })
                              }
                              className="h-7 font-mono text-xs"
                              placeholder="Mozilla/5.0"
                            />
                          </div>

                          <Separator className="opacity-50" />

                          {/* Headers Editor */}
                          <div className="flex flex-col gap-1.5">
                            <div className="flex items-center justify-between">
                              <Label className="text-[11px] font-medium text-foreground">
                                Headers
                              </Label>
                              <span className="text-[10px] text-muted-foreground">
                                {Object.keys(request.headers).length}
                              </span>
                            </div>

                            <div className="flex gap-1.5">
                              <Input
                                placeholder="Key (e.g. Authorization)"
                                value={headerKey}
                                onChange={(e) => setHeaderKey(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Input
                                placeholder="Value"
                                value={headerVal}
                                onChange={(e) => setHeaderVal(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Button
                                size="icon"
                                variant="outline"
                                className="size-7 shrink-0"
                                onClick={addHeader}
                              >
                                <Plus className="size-3" />
                              </Button>
                            </div>

                            <div className="flex flex-col gap-1">
                              {Object.entries(request.headers).map(
                                ([k, vals]) => (
                                  <div
                                    key={k}
                                    className="flex items-start gap-1 rounded-lg bg-muted/55 px-2 py-1 font-mono text-xs"
                                  >
                                    <span
                                      className="w-24 shrink-0 pt-1 text-foreground"
                                      title={k}
                                    >
                                      {k}:
                                    </span>
                                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                                      {vals.map((value, index) => (
                                        <Input
                                          key={`${k}-${index}`}
                                          value={value}
                                          onChange={(event) =>
                                            updateHeaderValue(
                                              k,
                                              index,
                                              event.target.value
                                            )
                                          }
                                          className="h-6 min-w-0 bg-transparent p-0 text-xs text-muted-foreground focus-visible:ring-0"
                                          aria-label={`Header ${k} value ${index + 1}`}
                                        />
                                      ))}
                                      <Button
                                        variant="ghost"
                                        size="sm"
                                        className="h-5 w-fit px-1 text-[10px] text-muted-foreground"
                                        onClick={() => appendHeaderValue(k)}
                                      >
                                        <Plus className="mr-1 size-2.5" /> Add
                                        value
                                      </Button>
                                    </div>
                                    <Button
                                      variant="ghost"
                                      size="icon"
                                      className="size-5 text-muted-foreground hover:text-foreground"
                                      onClick={() => removeHeader(k)}
                                      aria-label={`Remove header ${k}`}
                                    >
                                      <X className="size-2.5" />
                                    </Button>
                                  </div>
                                )
                              )}
                            </div>
                          </div>

                          <Separator className="opacity-50" />

                          {/* Query Parameters */}
                          <div className="flex flex-col gap-1.5">
                            <div className="flex items-center justify-between">
                              <Label className="text-[11px] font-medium text-foreground">
                                Query Parameters
                              </Label>
                              <span className="text-[10px] text-muted-foreground">
                                {Object.keys(request.query).length}
                              </span>
                            </div>
                            <div className="flex gap-1.5">
                              <Input
                                placeholder="Key (e.g. limit)"
                                value={queryKey}
                                onChange={(e) => setQueryKey(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Input
                                placeholder="Value"
                                value={queryVal}
                                onChange={(e) => setQueryVal(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Button
                                size="icon"
                                variant="outline"
                                className="size-7 shrink-0"
                                onClick={addQuery}
                              >
                                <Plus className="size-3" />
                              </Button>
                            </div>
                            <div className="flex flex-col gap-1">
                              {Object.entries(request.query).map(([k, v]) => (
                                <div
                                  key={k}
                                  className="flex items-center gap-1 rounded-lg bg-muted/55 px-2 py-0.5 font-mono text-xs"
                                >
                                  <span
                                    className="w-24 shrink-0 truncate text-muted-foreground"
                                    title={k}
                                  >
                                    ?{k}=
                                  </span>
                                  <Input
                                    value={v}
                                    onChange={(event) =>
                                      updateMapValue(
                                        "query",
                                        k,
                                        event.target.value
                                      )
                                    }
                                    className="h-6 min-w-0 flex-1 bg-transparent p-0 text-xs text-foreground focus-visible:ring-0"
                                    aria-label={`Query ${k} value`}
                                  />
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="size-5 text-muted-foreground hover:text-foreground"
                                    onClick={() => removeQuery(k)}
                                    aria-label={`Remove query ${k}`}
                                  >
                                    <X className="size-2.5" />
                                  </Button>
                                </div>
                              ))}
                            </div>
                          </div>

                          <Separator className="opacity-50" />

                          {/* Custom Fields */}
                          <div className="flex flex-col gap-1.5">
                            <div className="flex items-center justify-between">
                              <Label className="text-[11px] font-medium text-foreground">
                                Custom Fields
                              </Label>
                              <span className="text-[10px] text-muted-foreground">
                                {Object.keys(request.fields).length}
                              </span>
                            </div>
                            <div className="flex gap-1.5">
                              <Input
                                placeholder="Attribute (e.g. tier)"
                                value={fieldKey}
                                onChange={(e) => setFieldKey(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Input
                                placeholder="Value"
                                value={fieldVal}
                                onChange={(e) => setFieldVal(e.target.value)}
                                className="h-7 font-mono text-xs"
                              />
                              <Button
                                size="icon"
                                variant="outline"
                                className="size-7 shrink-0"
                                onClick={addField}
                              >
                                <Plus className="size-3" />
                              </Button>
                            </div>
                            <div className="flex flex-col gap-1">
                              {Object.entries(request.fields).map(([k, v]) => (
                                <div
                                  key={k}
                                  className="flex items-center gap-1 rounded-lg bg-muted/55 px-2 py-0.5 font-mono text-xs"
                                >
                                  <span
                                    className="w-24 shrink-0 truncate text-muted-foreground"
                                    title={k}
                                  >
                                    fields.{k} =
                                  </span>
                                  <Input
                                    value={v}
                                    onChange={(event) =>
                                      updateMapValue(
                                        "fields",
                                        k,
                                        event.target.value
                                      )
                                    }
                                    className="h-6 min-w-0 flex-1 bg-transparent p-0 text-xs text-foreground focus-visible:ring-0"
                                    aria-label={`Field ${k} value`}
                                  />
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="size-5 text-muted-foreground hover:text-foreground"
                                    onClick={() => removeField(k)}
                                    aria-label={`Remove field ${k}`}
                                  >
                                    <X className="size-2.5" />
                                  </Button>
                                </div>
                              ))}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  </ScrollArea>
                </TabsContent>

                {/* TAB: AST Tree Visualizer */}
                <TabsContent value="ast" className="m-0 flex-1 overflow-hidden">
                  <ScrollArea className="h-full p-3">
                    {validation?.policy ? (
                      <ASTVisualizer policy={validation.policy} />
                    ) : (
                      <div className="py-8 text-center text-xs text-muted-foreground">
                        Compile a valid policy to view the AST structure.
                      </div>
                    )}
                  </ScrollArea>
                </TabsContent>

                {/* TAB: JSON Preview */}
                <TabsContent
                  value="json"
                  className="m-0 flex flex-1 flex-col overflow-hidden"
                >
                  <div className="flex justify-end bg-muted/30 p-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handleCopyJSON}
                      className="h-6 gap-1 text-[11px] text-muted-foreground hover:text-foreground"
                    >
                      {copied === "workspace" ? (
                        <Check className="size-3 text-emerald-500" />
                      ) : (
                        <Copy className="size-3" />
                      )}
                      <span>
                        {copied === "workspace" ? "Copied" : "Copy workspace"}
                      </span>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={applyJSON}
                      className="ml-2 h-6 text-[11px]"
                    >
                      Apply request JSON
                    </Button>
                  </div>
                  <ScrollArea className="flex-1 p-3">
                    <Textarea
                      value={requestJSON}
                      onChange={(event) => {
                        setRequestJSON(event.target.value)
                        setJSONError(undefined)
                      }}
                      className="min-h-80 resize-none bg-muted/30 font-mono text-[11px] leading-relaxed"
                      spellCheck={false}
                      aria-label="Raw request JSON"
                    />
                    <p className="mt-2 text-[10px] text-muted-foreground">
                      Edit the request payload, then apply it. Policy AST
                      remains available in the AST tab.
                    </p>
                  </ScrollArea>
                </TabsContent>
              </Tabs>
            </ResizablePanel>

            <ResizableHandle
              withHandle
              className={isMobile ? "hidden" : undefined}
            />

            {/* Live Decision & Evaluation Trace */}
            <ResizablePanel
              defaultSize={isMobile ? "100%" : "42%"}
              minSize={isMobile ? "100%" : "24%"}
              className={`flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl bg-background ${isMobile && mobilePane !== "result" ? "hidden" : ""}`}
            >
              <div className="flex h-9 items-center justify-between bg-muted/60 px-3.5">
                <div>
                  <span className="text-xs font-medium text-foreground">
                    Result
                  </span>
                  <span className="ml-2 text-[11px] text-muted-foreground">
                    Why this request was allowed or denied
                  </span>
                </div>
                <div className="flex items-center gap-1">
                  {lastRun && (
                    <span
                      className="font-mono text-[10px] text-muted-foreground"
                      title={new Date(lastRun.completedAt).toLocaleString()}
                    >
                      {lastRun.durationMs}ms
                    </span>
                  )}
                  {(decision || lastRun) && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={onClearResult}
                      className="h-6 px-1.5 text-[11px] text-muted-foreground"
                    >
                      Clear
                    </Button>
                  )}
                </div>
              </div>

              <ScrollArea className="flex-1 p-3">
                <div className="flex flex-col gap-3">
                  {/* Make the verdict the first and strongest piece of information. */}
                  {decision ? (
                    <div
                      className={`rounded-xl p-4 ${decision.allowed ? "bg-emerald-500/10" : "bg-destructive/10"}`}
                      role="status"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div
                            className={`text-2xl font-semibold tracking-tight ${decision.allowed ? "text-emerald-600 dark:text-emerald-400" : "text-destructive"}`}
                          >
                            {decision.allowed ? "ALLOW" : "DENY"}
                          </div>
                          <div className="mt-1 flex items-center gap-2 font-mono text-[11px] text-muted-foreground">
                            <span>
                              HTTP{" "}
                              {decision.status ||
                                (decision.allowed ? 200 : 403)}
                            </span>
                            <span aria-hidden="true">·</span>
                            <span>
                              {decision.default
                                ? "default action"
                                : `rule ${decision.rule}`}
                            </span>
                          </div>
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={copyDecision}
                          className="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
                        >
                          {copied === "decision" ? "Copied" : "Copy"}
                        </Button>
                      </div>

                      {decision.reason && (
                        <p className="mt-4 max-w-prose text-sm leading-relaxed text-foreground/80">
                          {decision.reason}
                        </p>
                      )}
                      {lastRun && (
                        <p className="mt-3 font-mono text-[10px] text-muted-foreground">
                          Completed{" "}
                          {new Date(lastRun.completedAt).toLocaleTimeString()} ·{" "}
                          {lastRun.durationMs}ms
                        </p>
                      )}

                      {decision.trace && decision.trace.length > 0 && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowTrace((visible) => !visible)}
                          className="mt-4 h-8 text-xs"
                        >
                          {showTrace
                            ? "Hide evaluation trace"
                            : "Show evaluation trace"}
                        </Button>
                      )}
                    </div>
                  ) : (
                    <div className="rounded-xl bg-muted/35 p-8 text-center">
                      <div className="mx-auto flex size-9 items-center justify-center rounded-full bg-muted text-muted-foreground">
                        <Play weight="fill" className="size-3.5" />
                      </div>
                      <p className="mt-3 text-sm font-medium text-foreground">
                        Run a request to see the result
                      </p>
                      <p className="mx-auto mt-1 max-w-xs text-xs leading-relaxed text-muted-foreground">
                        Sentinel will show the decision, matched rule, and
                        explanation here.
                      </p>
                    </div>
                  )}

                  {/* Evaluation Trace Timeline */}
                  {showTrace &&
                    decision?.trace &&
                    decision.trace.length > 0 && (
                      <div className="flex flex-col gap-2">
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-medium text-foreground">
                            Evaluation trace
                          </span>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={copyTrace}
                            className="h-7 px-2 text-[11px] text-muted-foreground"
                          >
                            {copied === "trace" ? "Copied" : "Copy trace"}
                          </Button>
                        </div>

                        <div className="flex flex-col gap-1.5">
                          {decision.trace.map((item, idx) => (
                            <div
                              key={idx}
                              className="flex flex-col gap-1 rounded-lg bg-muted/35 p-2.5 text-xs"
                            >
                              <div className="flex items-center justify-between">
                                <div className="flex items-center gap-1.5">
                                  <span
                                    className={`size-1.5 rounded-full ${
                                      item.matched
                                        ? "bg-emerald-500"
                                        : "bg-muted-foreground/40"
                                    }`}
                                  />
                                  <span className="font-mono font-medium text-foreground">
                                    {item.rule}
                                  </span>
                                </div>
                                <span
                                  className={`font-mono text-[10px] ${
                                    item.matched
                                      ? "font-medium text-emerald-500"
                                      : "text-muted-foreground"
                                  }`}
                                >
                                  {item.matched ? "matched" : "skipped"}
                                </span>
                              </div>
                              {item.detail && (
                                <span className="pl-3 font-mono text-[11px] text-muted-foreground">
                                  {item.detail}
                                </span>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                </div>
              </ScrollArea>
            </ResizablePanel>
          </ResizablePanelGroup>
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  )
}

function ASTVisualizer({ policy }: { policy: PolicySummary }) {
  return (
    <div className="flex flex-col gap-2 font-mono text-xs">
      <div className="flex items-center justify-between rounded-lg bg-muted/45 px-2.5 py-1.5">
        <span className="font-medium text-foreground">
          policy {policy.name}
        </span>
        <span className="text-[10px] text-muted-foreground">
          {policy.rule_count} rule(s)
        </span>
      </div>

      <div className="flex flex-col gap-1.5 rounded-lg bg-muted/20 p-2">
        {policy.rules.map((rule, idx) => (
          <div key={idx} className="rounded-lg bg-card p-2">
            <div className="flex items-center justify-between">
              <span className="font-medium text-foreground">
                rule {rule.name}
              </span>
              <span className="text-[10px] text-muted-foreground">
                then {rule.action.kind}{" "}
                {rule.action.status ? `(${rule.action.status})` : ""}
              </span>
            </div>

            {rule.condition && (
              <div className="mt-1.5 bg-muted/35 p-1.5 text-[11px]">
                <ASTNodeItem node={rule.condition} depth={0} />
              </div>
            )}
          </div>
        ))}

        {policy.default && (
          <div className="rounded-lg bg-muted/35 p-2">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">default</span>
              <span className="text-[10px] text-muted-foreground">
                {policy.default.kind}{" "}
                {policy.default.status ? `(${policy.default.status})` : ""}
              </span>
            </div>
            {policy.default.message && (
              <p className="mt-1 text-[11px] text-muted-foreground">
                &quot;{policy.default.message}&quot;
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function ASTNodeItem({ node, depth }: { node: ASTNode; depth: number }) {
  const indent = depth * 10

  if (node.type === "BinaryExpr" || node.type === "CompareExpr") {
    return (
      <div
        style={{ marginLeft: `${indent}px` }}
        className="my-0.5 flex flex-col gap-0.5"
      >
        <span className="text-[10px] font-semibold text-muted-foreground uppercase">
          {node.operator || node.type}
        </span>
        <div className="flex flex-col rounded bg-muted/25 p-1.5">
          {node.left && <ASTNodeItem node={node.left} depth={depth + 1} />}
          {node.right && <ASTNodeItem node={node.right} depth={depth + 1} />}
        </div>
      </div>
    )
  }

  if (node.type === "ListExpr" && node.children) {
    return (
      <div
        style={{ marginLeft: `${indent}px` }}
        className="my-0.5 flex items-center gap-1 text-[11px]"
      >
        <span className="text-muted-foreground">[</span>
        {node.children.map((c, i) => (
          <span key={i} className="text-foreground">
            {c.value}
            {i < (node.children?.length ?? 0) - 1 ? ", " : ""}
          </span>
        ))}
        <span className="text-muted-foreground">]</span>
      </div>
    )
  }

  return (
    <div
      style={{ marginLeft: `${indent}px` }}
      className="my-0.5 flex items-center gap-1 text-[11px]"
    >
      <span className="text-muted-foreground">{node.type}:</span>
      <span className="font-medium text-foreground">
        &quot;{node.value}&quot;
      </span>
    </div>
  )
}
