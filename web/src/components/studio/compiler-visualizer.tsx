import * as React from "react"
import { ScrollArea } from "@/components/ui/scroll-area"
import { lexPolicy } from "@/lib/api-client"
import type { Diagnostic, LexerToken, PolicySummary } from "@/lib/types"

interface CompilerVisualizerProps {
  source: string
  policy?: PolicySummary
}

export function CompilerVisualizer({
  source,
  policy,
}: CompilerVisualizerProps) {
  const [selectedStage, setSelectedStage] = React.useState<number>(1)

  const stages = [
    {
      step: 1,
      title: "Lexical Analysis",
      tag: "Source → Tokens",
      description:
        "Scans raw UTF-8 policy source text into a linear stream of strongly typed tokens with line/column coordinates.",
    },
    {
      step: 2,
      title: "Syntax Parsing",
      tag: "Tokens → AST",
      description:
        "Recursive-descent parser verifying grammar productions and generating an immutable AST hierarchy.",
    },
    {
      step: 3,
      title: "Semantic Analysis",
      tag: "AST → Validated Policy",
      description:
        "Enforces security invariants: validates request fields, checks operator compatibility, and mandates default posture.",
    },
    {
      step: 4,
      title: "Runtime Evaluator",
      tag: "AST + Request → Decision",
      description:
        "Evaluates rules in deterministic source order with short-circuiting and emits an explainable trace.",
    },
    {
      step: 5,
      title: "HTTP Middleware",
      tag: "net/http Enforcement",
      description:
        "Intercepts incoming HTTP requests, passes them into the evaluator, and writes status/headers or forwards downstream.",
    },
  ]

  return (
    <div className="flex h-full w-full flex-1 flex-col overflow-hidden bg-background">
      {/* The pipeline is an explanation of the policy flow, not another dashboard. */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/30 px-4 py-3">
        <div>
          <h1 className="text-sm font-medium text-foreground">Compiler</h1>
          <p className="text-[11px] text-muted-foreground">
            See how Sentinel turns policy source into an explainable decision
          </p>
        </div>
        <span className="font-mono text-[11px] text-muted-foreground">
          5 stages · always terminating
        </span>
      </div>

      <div className="grid flex-1 grid-cols-1 gap-2 overflow-hidden bg-muted/55 p-2 lg:grid-cols-[280px_1fr]">
        {/* Left: Stage Navigation Timeline */}
        <div className="flex flex-col gap-1 overflow-y-auto rounded-xl bg-background p-3">
          <span className="mb-2 px-2 text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
            Pipeline
          </span>

          <div className="flex flex-col gap-1">
            {stages.map((st) => {
              const isSelected = selectedStage === st.step
              return (
                <button
                  key={st.step}
                  type="button"
                  onClick={() => setSelectedStage(st.step)}
                  className={`flex flex-col items-start rounded-md px-2.5 py-2 text-left transition-colors ${
                    isSelected
                      ? "bg-muted font-medium text-foreground"
                      : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                  }`}
                >
                  <div className="flex items-center gap-1.5 text-xs">
                    <span className="font-mono text-[10px] text-muted-foreground">
                      0{st.step}
                    </span>
                    <span>{st.title}</span>
                  </div>
                  <span className="mt-0.5 font-mono text-[10px] opacity-70">
                    {st.tag}
                  </span>
                </button>
              )
            })}
          </div>
        </div>

        {/* Right: Stage Detail & Visual Inspection */}
        <ScrollArea className="h-full rounded-xl bg-background p-6">
          <div className="flex max-w-3xl flex-col gap-5">
            {/* Stage Title Card */}
            <div className="rounded-xl bg-card p-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium text-foreground">
                  Stage {selectedStage}: {stages[selectedStage - 1].title}
                </h3>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {stages[selectedStage - 1].tag}
                </span>
              </div>
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                {stages[selectedStage - 1].description}
              </p>
            </div>

            {/* Dynamic Stage Content */}
            {selectedStage === 1 && <Stage1Lexer source={source} />}
            {selectedStage === 2 && <Stage2Parser policy={policy} />}
            {selectedStage === 3 && (
              <Stage3Semantic source={source} policy={policy} />
            )}
            {selectedStage === 4 && <Stage4Evaluator />}
            {selectedStage === 5 && <Stage5Middleware />}
          </div>
        </ScrollArea>
      </div>
    </div>
  )
}

function Stage1Lexer({ source }: { source: string }) {
  const [state, setState] = React.useState<{
    tokens: LexerToken[]
    diagnostics: Diagnostic[]
    loading: boolean
    error?: string
  }>({ tokens: [], diagnostics: [], loading: false })

  React.useEffect(() => {
    const controller = new AbortController()
    if (!source.trim()) {
      setState({ tokens: [], diagnostics: [], loading: false })
      return () => controller.abort()
    }
    setState((current) => ({ ...current, loading: true, error: undefined }))
    const timer = window.setTimeout(() => {
      void lexPolicy(source, { signal: controller.signal })
        .then((response) => {
          if (controller.signal.aborted) return
          setState({
            tokens: response.tokens,
            diagnostics: response.diagnostics,
            loading: false,
          })
        })
        .catch((error: unknown) => {
          if (controller.signal.aborted) return
          setState({
            tokens: [],
            diagnostics: [],
            loading: false,
            error:
              error instanceof Error
                ? error.message
                : "Unable to fetch lexer output",
          })
        })
    }, 220)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [source])

  const errors = state.diagnostics
  return (
    <div className="rounded-xl bg-card p-4">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-xs font-medium text-foreground">
          Token Stream
        </span>
        <span className="font-mono text-[11px] text-muted-foreground">
          {state.loading ? "Loading lexer…" : `${state.tokens.length} tokens`}
          {!state.loading && errors.length
            ? ` · ${errors.length} diagnostic${errors.length === 1 ? "" : "s"}`
            : ""}
        </span>
      </div>
      {state.error ? (
        <div
          className="rounded-lg bg-destructive/10 p-3 text-xs text-destructive"
          role="alert"
        >
          {state.error}
        </div>
      ) : state.loading ? (
        <p className="text-xs text-muted-foreground" role="status">
          Requesting token stream from the Sentinel API…
        </p>
      ) : source.trim() === "" ? (
        <p className="text-xs text-muted-foreground">
          Type policy source to inspect its tokens.
        </p>
      ) : (
        <div className="flex max-h-72 flex-wrap gap-1.5 overflow-y-auto font-mono text-xs">
          {state.tokens.map((token, index) => (
            <div
              key={`${token.start.offset}-${index}`}
              title={`${token.kind} at ${token.start.line}:${token.start.column}`}
              className="flex items-center gap-1 rounded-lg bg-muted/55 px-2 py-1"
            >
              <span className="text-[10px] text-muted-foreground">
                {token.kind}
              </span>
              <span className="font-medium text-foreground">{token.text}</span>
              <span className="text-[10px] text-muted-foreground/60">
                {token.start.line}:{token.start.column} · {token.start.offset}
              </span>
            </div>
          ))}
        </div>
      )}
      {!state.loading && !state.error && errors.length > 0 && (
        <ul
          className="mt-3 list-inside list-disc text-[11px] text-destructive"
          role="alert"
        >
          {errors.map((diagnostic, index) => (
            <li key={`${diagnostic.message}-${index}`}>
              {diagnostic.message} ({diagnostic.start.line}:
              {diagnostic.start.column})
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function Stage2Parser({ policy }: { policy?: PolicySummary }) {
  return (
    <div className="flex flex-col gap-3">
      <div className="rounded-xl bg-card p-4">
        <span className="mb-2 block text-xs font-medium text-foreground">
          Grammar Specification (EBNF)
        </span>
        <pre className="overflow-x-auto rounded-lg bg-muted/55 p-3 font-mono text-xs leading-relaxed text-foreground">
          {`Policy     ::= "policy" Identifier "{" { Rule } [ Default ] "}"
Rule       ::= "rule" Identifier "when" Expr "then" Action ";"
Action     ::= "allow" [ Status ] | "deny" [ Status ] [ String ]
Default    ::= "default" Action ";"`}
        </pre>
      </div>

      {policy && (
        <div className="rounded-xl bg-card p-3 text-xs">
          <span className="font-medium text-foreground">
            AST: {policy.name}
          </span>
          <span className="ml-2 font-mono text-muted-foreground">
            ({policy.rule_count} rules parsed)
          </span>
        </div>
      )}
    </div>
  )
}

function Stage3Semantic({
  source,
  policy,
}: {
  source: string
  policy?: PolicySummary
}) {
  const securityChecks = [
    {
      name: "Mandatory Default Posture",
      desc: "Every policy must declare an explicit default action (default deny/allow).",
      passed: Boolean(policy?.default),
    },
    {
      name: "Terminating Guarantee",
      desc: "No while loops, recursion, or dynamic jumps. Language is strictly terminating.",
      passed:
        source.trim() !== "" && !/\b(?:while|loop|goto|recurs)\b/i.test(source),
    },
    {
      name: "Restricted Symbol Scope",
      desc: "Only fixed request fields (method, path, host, role, remote_addr, fields.*) are valid.",
      passed: Boolean(policy),
    },
    {
      name: "Type Compatibility",
      desc: "String comparisons, numerical comparisons, and list membership are strictly checked.",
      passed: Boolean(policy),
    },
  ]

  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
      {securityChecks.map((chk, i) => (
        <div key={i} className="rounded-xl bg-card p-3">
          <div className="mb-1 flex items-center gap-1.5">
            <span
              className={`flex size-3.5 items-center justify-center rounded-full text-[9px] ${chk.passed ? "bg-emerald-500/15 text-emerald-500" : "bg-muted text-muted-foreground"}`}
            >
              {chk.passed ? "✓" : "·"}
            </span>
            <span className="text-xs font-medium text-foreground">
              {chk.name}
            </span>
            <span className="ml-auto text-[10px] text-muted-foreground">
              {chk.passed ? "verified" : "pending validation"}
            </span>
          </div>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {chk.desc}
          </p>
        </div>
      ))}
    </div>
  )
}

function Stage4Evaluator() {
  return (
    <div className="rounded-xl bg-card p-4">
      <span className="mb-3 block text-xs font-medium text-foreground">
        Evaluation Semantics
      </span>
      <div className="flex flex-col gap-2 text-xs">
        <div className="flex flex-col gap-0.5 rounded-lg bg-muted/45 p-2.5">
          <span className="font-mono font-medium text-foreground">
            First Match Wins
          </span>
          <span className="text-[11px] leading-relaxed text-muted-foreground">
            Rules evaluate sequentially in source order. The first rule whose
            condition evaluates to true terminates evaluation and returns the
            decision.
          </span>
        </div>
        <div className="flex flex-col gap-0.5 rounded-lg bg-muted/45 p-2.5">
          <span className="font-mono font-medium text-foreground">
            Short Circuiting
          </span>
          <span className="text-[11px] leading-relaxed text-muted-foreground">
            Boolean expressions `and` and `or` use short-circuit evaluation to
            skip unnecessary branch computation.
          </span>
        </div>
      </div>
    </div>
  )
}

function Stage5Middleware() {
  return (
    <div className="rounded-xl bg-card p-4">
      <span className="mb-2 block text-xs font-medium text-foreground">
        Go HTTP Middleware Integration
      </span>
      <pre className="overflow-x-auto rounded-lg bg-muted/55 p-3 font-mono text-xs leading-relaxed text-foreground">
        {`// Wrap standard Go http.Handler with Sentinel authorization
middleware := sentinel.New(policy)

http.Handle("/api/", middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Only executed if Sentinel allows the request
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Authorized payload"))
})))`}
      </pre>
    </div>
  )
}
