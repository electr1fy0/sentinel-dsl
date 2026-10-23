import type {
  AutomataResponse,
  CFGResponse,
  CSGResponse,
  EvaluateResponse,
  ExampleScenario,
  GraphResponse,
  LBAResponse,
  LexerResponse,
  PDAResponse,
  RequestPayload,
  SimStep,
  TMResponse,
  ValidateResponse,
} from "./types"

/** A structured error returned by the browser client for every failed request. */
export class ApiError extends Error {
  readonly kind: "http" | "network" | "timeout" | "cancelled" | "parse"
  readonly endpoint: string
  readonly status?: number
  readonly details?: unknown

  constructor(
    message: string,
    options: {
      kind: ApiError["kind"]
      endpoint: string
      status?: number
      details?: unknown
    }
  ) {
    super(message)
    this.name = "ApiError"
    this.kind = options.kind
    this.endpoint = options.endpoint
    this.status = options.status
    this.details = options.details
  }
}

export interface ApiRequestOptions {
  signal?: AbortSignal
  timeoutMs?: number
}

const configuredBase = import.meta.env.VITE_API_BASE_URL
const API_BASE = (configuredBase || "/api").replace(/\/$/, "")

export function getApiBase(): string {
  return API_BASE
}

export function isApiCancellation(error: unknown): boolean {
  return error instanceof ApiError && error.kind === "cancelled"
}

function messageFromError(data: unknown, fallback: string): string {
  if (typeof data === "object" && data !== null && "error" in data) {
    const value = (data as { error?: unknown }).error
    if (typeof value === "string" && value.trim()) return value
  }
  return fallback
}

async function readErrorBody(response: Response): Promise<unknown> {
  const text = await response.text().catch(() => "")
  if (!text) return undefined
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function requestUrl(endpoint: string): string {
  return `${API_BASE}${endpoint}`
}

function requestSignal(
  callerSignal: AbortSignal | undefined,
  timeoutMs: number
): {
  signal: AbortSignal
  didTimeout: () => boolean
  cleanup: () => void
} {
  const controller = new AbortController()
  let timedOut = false
  const timeout = globalThis.setTimeout(() => {
    timedOut = true
    controller.abort()
  }, timeoutMs)
  const onAbort = () => controller.abort()
  callerSignal?.addEventListener("abort", onAbort, { once: true })

  return {
    signal: controller.signal,
    didTimeout: () => timedOut,
    cleanup: () => {
      globalThis.clearTimeout(timeout)
      callerSignal?.removeEventListener("abort", onAbort)
    },
  }
}

async function fetchJSON<T>(
  endpoint: string,
  init: RequestInit = {},
  options: ApiRequestOptions = {}
): Promise<T> {
  const timeoutMs = options.timeoutMs ?? 10_000
  if (options.signal?.aborted) {
    throw new ApiError("Request cancelled", {
      kind: "cancelled",
      endpoint,
    })
  }

  const signalState = requestSignal(options.signal, timeoutMs)
  try {
    const response = await fetch(requestUrl(endpoint), {
      ...init,
      signal: signalState.signal,
    })
    if (!response.ok) {
      const details = await readErrorBody(response)
      throw new ApiError(
        messageFromError(
          details,
          `Request failed with HTTP ${response.status}`
        ),
        { kind: "http", endpoint, status: response.status, details }
      )
    }
    try {
      return (await response.json()) as T
    } catch (error) {
      throw new ApiError("The API returned invalid JSON", {
        kind: "parse",
        endpoint,
        details: error,
      })
    }
  } catch (error) {
    if (error instanceof ApiError) throw error
    if (options.signal?.aborted) {
      throw new ApiError("Request cancelled", {
        kind: "cancelled",
        endpoint,
      })
    }
    if (signalState.didTimeout()) {
      throw new ApiError(`Request timed out after ${timeoutMs}ms`, {
        kind: "timeout",
        endpoint,
      })
    }
    throw new ApiError("Unable to reach the Sentinel API", {
      kind: "network",
      endpoint,
      details: error,
    })
  } finally {
    signalState.cleanup()
  }
}

async function postJSON<T>(
  endpoint: string,
  payload: unknown,
  options?: ApiRequestOptions
): Promise<T> {
  return fetchJSON<T>(
    endpoint,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
    options
  )
}

export interface HealthResponse {
  ok: boolean
  service?: string
  version?: string
  error?: ApiError
}

export async function checkHealth(
  options?: ApiRequestOptions
): Promise<HealthResponse> {
  try {
    const data = await fetchJSON<{
      status?: string
      service?: string
      version?: string
    }>("/health", {}, { timeoutMs: 2_000, ...options })
    return {
      ok: data.status === "ok",
      service: data.service,
      version: data.version,
    }
  } catch (error) {
    return {
      ok: false,
      error:
        error instanceof ApiError
          ? error
          : new ApiError("Unable to check API health", {
              kind: "network",
              endpoint: "/health",
              details: error,
            }),
    }
  }
}

export function fetchExamples(
  options?: ApiRequestOptions
): Promise<ExampleScenario[]> {
  return fetchJSON<ExampleScenario[]>("/examples", {}, options)
}

export function fetchExample(
  id: string,
  options?: ApiRequestOptions
): Promise<ExampleScenario> {
  return fetchJSON<ExampleScenario>(
    `/examples/${encodeURIComponent(id)}`,
    {},
    options
  )
}

export function validatePolicy(
  source: string,
  options?: ApiRequestOptions
): Promise<ValidateResponse> {
  return postJSON<ValidateResponse>("/validate", { source }, options)
}

export function evaluatePolicy(
  source: string,
  request: RequestPayload,
  options?: ApiRequestOptions
): Promise<EvaluateResponse> {
  return postJSON<EvaluateResponse>("/evaluate", { source, request }, options)
}

export function evaluatePolicyBatch(
  source: string,
  requests: RequestPayload[],
  explain = false,
  options?: ApiRequestOptions
): Promise<{
  valid: boolean
  diagnostics: ValidateResponse["diagnostics"]
  results?: EvaluateResponse[]
}> {
  return postJSON("/evaluate/batch", { source, requests, explain }, options)
}

export function explainPolicy(
  source: string,
  request: RequestPayload,
  options?: ApiRequestOptions
): Promise<EvaluateResponse> {
  return postJSON<EvaluateResponse>("/explain", { source, request }, options)
}

export function analyzeAutomata(
  pattern: string,
  input: string,
  options?: ApiRequestOptions
): Promise<AutomataResponse> {
  return postJSON<AutomataResponse>(
    "/automata",
    { pattern, input },
    options
  ).then(normalizeAutomataResponse)
}

function normalizeGraph(
  graph: GraphResponse | null | undefined
): GraphResponse | undefined {
  if (!graph) return undefined
  return {
    ...graph,
    alphabet: Array.isArray(graph.alphabet) ? graph.alphabet : [],
    nodes: Array.isArray(graph.nodes) ? graph.nodes : [],
    edges: Array.isArray(graph.edges) ? graph.edges : [],
  }
}

function normalizeSteps(
  steps: SimStep[] | null | undefined
): SimStep[] | undefined {
  if (!Array.isArray(steps)) return undefined
  return steps.map((step) => ({
    ...step,
    active_states: Array.isArray(step.active_states) ? step.active_states : [],
  }))
}

function normalizeAutomataResponse(
  response: AutomataResponse
): AutomataResponse {
  return {
    ...response,
    diagnostics: Array.isArray(response.diagnostics)
      ? response.diagnostics
      : [],
    nfa: normalizeGraph(response.nfa),
    dfa: normalizeGraph(response.dfa),
    minimized: normalizeGraph(response.minimized),
    nfa_sim_steps: normalizeSteps(response.nfa_sim_steps),
    dfa_sim_steps: normalizeSteps(response.dfa_sim_steps),
  }
}

export function analyzeCFG(
  payload: {
    variables: string[]
    terminals: string[]
    start: string
    productions: { lhs: string; rhs: string[] }[]
    max_steps?: number
  },
  options?: ApiRequestOptions
): Promise<CFGResponse> {
  return postJSON<CFGResponse>("/theory/cfg", payload, options)
}

export function simulatePDA(
  payload: {
    states: string[]
    input_alphabet: string[]
    stack_alphabet: string[]
    start: string
    initial_stack: string
    accept_states: string[]
    accept_by_final_state: boolean
    accept_by_empty_stack: boolean
    transitions: {
      from_state: string
      input: string
      stack_top: string
      to_state: string
      push: string[]
    }[]
    input: string[]
    max_steps?: number
  },
  options?: ApiRequestOptions
): Promise<PDAResponse> {
  return postJSON<PDAResponse>("/theory/pda", payload, options)
}

export function simulateTM(
  payload: {
    states: string[]
    alphabet: string[]
    tape_alphabet: string[]
    blank: string
    start: string
    accept_states: string[]
    reject_states: string[]
    transitions: {
      from_state: string
      read: string
      to_state: string
      write: string
      move: number
    }[]
    input: string[]
    max_steps?: number
  },
  options?: ApiRequestOptions
): Promise<TMResponse> {
  return postJSON<TMResponse>("/theory/tm", payload, options)
}

export function simulateLBA(
  payload: {
    states: string[]
    alphabet: string[]
    tape_alphabet: string[]
    blank: string
    start: string
    accept_states: string[]
    reject_states: string[]
    transitions: {
      from_state: string
      read: string
      to_state: string
      write: string
      move: number
    }[]
    input: string[]
    max_steps?: number
  },
  options?: ApiRequestOptions
): Promise<LBAResponse> {
  return postJSON<LBAResponse>("/theory/lba", payload, options)
}

export function analyzeCSG(
  payload: {
    variables: string[]
    terminals: string[]
    start: string
    productions: { lhs: string[]; rhs: string[] }[]
    input: string[]
    max_steps?: number
    max_length?: number
  },
  options?: ApiRequestOptions
): Promise<CSGResponse> {
  return postJSON<CSGResponse>("/theory/csg", payload, options)
}

export function lexPolicy(
  source: string,
  options?: ApiRequestOptions
): Promise<LexerResponse> {
  return postJSON<LexerResponse>("/lexer", { source }, options)
}
