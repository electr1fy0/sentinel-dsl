export interface Point {
  offset: number
  line: number
  column: number
}

export interface Diagnostic {
  severity: "error" | "warning" | "info"
  message: string
  start: Point
  end: Point
}

export interface ASTNode {
  type: string
  value?: string
  operator?: string
  left?: ASTNode
  right?: ASTNode
  children?: ASTNode[]
  start: Point
  end: Point
}

export interface ActionSummary {
  kind: "allow" | "deny"
  status?: number
  message?: string
  start?: Point
  end?: Point
}

export interface RuleSummary {
  name: string
  action: ActionSummary
  condition?: ASTNode
  start: Point
  end: Point
}

export interface PolicySummary {
  name: string
  rule_count: number
  rules: RuleSummary[]
  default?: ActionSummary
  start: Point
  end: Point
}

export interface ValidateResponse {
  valid: boolean
  diagnostics: Diagnostic[]
  policy?: PolicySummary
}

export interface RequestPayload {
  method: string
  path: string
  host: string
  scheme: string
  remote_addr: string
  user_agent: string
  role: string
  headers: Record<string, string[]>
  query: Record<string, string>
  fields: Record<string, string>
}

export interface TraceResponse {
  rule: string
  matched: boolean
  detail: string
}

export interface DecisionResponse {
  allowed: boolean
  decision: "ALLOW" | "DENY"
  rule?: string
  default: boolean
  status?: number
  reason?: string
  trace?: TraceResponse[]
}

export interface EvaluateResponse {
  valid: boolean
  diagnostics: Diagnostic[]
  decision?: DecisionResponse
}

export interface GraphNode {
  id: string
  label: string
  start: boolean
  accept: boolean
  x?: number
  y?: number
}

export interface GraphEdge {
  from: string
  to: string
  label: string
  epsilon?: boolean
}

export interface GraphResponse {
  states: number
  accepting: number
  alphabet: string[]
  start: string
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface SimStep {
  step_index: number
  symbol: string
  active_states: string[]
  description: string
}

export interface AutomataResponse {
  valid: boolean
  pattern: string
  input?: string
  accepted: boolean
  diagnostics?: Diagnostic[]
  nfa?: GraphResponse
  dfa?: GraphResponse
  minimized?: GraphResponse
  nfa_sim_steps?: SimStep[]
  dfa_sim_steps?: SimStep[]
}

export interface ExampleScenario {
  id: string
  category: string
  name: string
  description: string
  source: string
  request: RequestPayload
}

export interface CFGProduction {
  lhs: string
  rhs: string[]
}

export interface CFGResponse {
  valid: boolean
  error?: string
  first?: Record<string, string[]>
  follow?: Record<string, string[]>
  derivations?: string[][]
  simplified?: {
    variables: string[]
    terminals: string[]
    start: string
    productions: CFGProduction[]
  }
}

export interface PDATraceItem {
  state: string
  position: number
  stack: string[]
}

export interface PDAResponse {
  valid: boolean
  error?: string
  accepted?: boolean
  timed_out?: boolean
  steps?: number
  trace?: PDATraceItem[]
}

export interface TMConfiguration {
  state: string
  head: number
  tape: Record<number, string>
  min_index: number
  max_index: number
}

export interface TMResponse {
  valid: boolean
  error?: string
  accepted?: boolean
  rejected?: boolean
  halted?: boolean
  timed_out?: boolean
  steps?: number
  configuration?: TMConfiguration
  trace?: TMConfiguration[]
}

/** LBA responses use the same bounded tape configuration shape as TMs. */
export interface LBAResponse {
  valid: boolean
  error?: string
  accepted?: boolean
  rejected?: boolean
  halted?: boolean
  timed_out?: boolean
  steps?: number
  configuration?: {
    state: string
    head: number
    tape: string[]
  }
  trace?: Array<{
    state: string
    head: number
    tape: string[]
  }>
}

export interface CSGResponse {
  valid: boolean
  error?: string
  accepted?: boolean
  steps?: number
  derivation?: string[][]
}

export interface LexerToken {
  kind: string
  text: string
  value?: string
  start: Point
  end: Point
}

export interface LexerResponse {
  valid: boolean
  diagnostics: Diagnostic[]
  tokens: LexerToken[]
}

export interface TestMatrixItem {
  id: string
  name: string
  description: string
  request: RequestPayload
  expectedDecision: "ALLOW" | "DENY"
  expectedRule?: string
}
