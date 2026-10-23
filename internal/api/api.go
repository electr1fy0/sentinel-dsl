package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/diagnostics"
	"github.com/ayush/sentinel-dsl/internal/evaluator"
	"github.com/ayush/sentinel-dsl/internal/lexer"
	"github.com/ayush/sentinel-dsl/internal/parser"
	"github.com/ayush/sentinel-dsl/internal/semantic"
	"github.com/ayush/sentinel-dsl/theory/automata"
	"github.com/ayush/sentinel-dsl/theory/cfg"
	"github.com/ayush/sentinel-dsl/theory/csg"
	"github.com/ayush/sentinel-dsl/theory/lba"
	"github.com/ayush/sentinel-dsl/theory/pda"
	"github.com/ayush/sentinel-dsl/theory/regex"
	"github.com/ayush/sentinel-dsl/theory/tm"
)

// Handler exposes the browser-facing Sentinel Studio API. Policies are sent in
// each request so the editor can evaluate unsaved source without mutating the
// server's process state.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	mux.HandleFunc("GET /api/examples", examples)
	mux.HandleFunc("GET /api/examples/{id}", example)
	mux.HandleFunc("POST /api/validate", validate)
	mux.HandleFunc("POST /api/evaluate", evaluate)
	mux.HandleFunc("POST /api/evaluate/batch", evaluateBatch)
	mux.HandleFunc("POST /api/explain", explain)
	mux.HandleFunc("POST /api/ast", astHandler)
	mux.HandleFunc("POST /api/lexer", lexerHandler)
	mux.HandleFunc("POST /api/automata", automataHandler)
	mux.HandleFunc("POST /api/theory/cfg", cfgHandler)
	mux.HandleFunc("POST /api/theory/pda", pdaHandler)
	mux.HandleFunc("POST /api/theory/tm", tmHandler)
	mux.HandleFunc("POST /api/theory/lba", lbaHandler)
	mux.HandleFunc("POST /api/theory/csg", csgHandler)
	return cors(mux)
}

type diagnosticResponse struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Start    point  `json:"start"`
	End      point  `json:"end"`
}

type point struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type astNodeJSON struct {
	Type     string        `json:"type"`
	Value    string        `json:"value,omitempty"`
	Operator string        `json:"operator,omitempty"`
	Left     *astNodeJSON  `json:"left,omitempty"`
	Right    *astNodeJSON  `json:"right,omitempty"`
	Children []astNodeJSON `json:"children,omitempty"`
	Start    point         `json:"start"`
	End      point         `json:"end"`
}

type ruleSummary struct {
	Name      string        `json:"name"`
	Action    actionSummary `json:"action"`
	Condition *astNodeJSON  `json:"condition,omitempty"`
	Start     point         `json:"start"`
	End       point         `json:"end"`
}

type actionSummary struct {
	Kind    string `json:"kind"`
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Start   point  `json:"start,omitempty"`
	End     point  `json:"end,omitempty"`
}

type policySummary struct {
	Name      string         `json:"name"`
	RuleCount int            `json:"rule_count"`
	Rules     []ruleSummary  `json:"rules"`
	Default   *actionSummary `json:"default,omitempty"`
	Start     point          `json:"start"`
	End       point          `json:"end"`
}

type validateResponse struct {
	Valid       bool                 `json:"valid"`
	Diagnostics []diagnosticResponse `json:"diagnostics"`
	Policy      *policySummary       `json:"policy,omitempty"`
}

type evaluatePayload struct {
	Source  string         `json:"source"`
	Request requestPayload `json:"request"`
}

type requestPayload struct {
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Host       string              `json:"host"`
	Scheme     string              `json:"scheme"`
	RemoteAddr string              `json:"remote_addr"`
	UserAgent  string              `json:"user_agent"`
	Role       string              `json:"role"`
	Headers    map[string][]string `json:"headers"`
	Query      map[string]string   `json:"query"`
	Fields     map[string]string   `json:"fields"`
}

func (r requestPayload) request() evaluator.Request {
	return evaluator.Request{
		Method:     r.Method,
		Path:       r.Path,
		Host:       r.Host,
		Scheme:     r.Scheme,
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.UserAgent,
		Role:       r.Role,
		Headers:    r.Headers,
		Query:      r.Query,
		Fields:     r.Fields,
	}
}

type traceResponse struct {
	Rule    string `json:"rule"`
	Matched bool   `json:"matched"`
	Detail  string `json:"detail"`
}

type decisionResponse struct {
	Allowed  bool            `json:"allowed"`
	Decision string          `json:"decision"`
	Rule     string          `json:"rule,omitempty"`
	Default  bool            `json:"default"`
	Status   int             `json:"status,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Trace    []traceResponse `json:"trace,omitempty"`
}

type evaluateResponse struct {
	Valid       bool                 `json:"valid"`
	Diagnostics []diagnosticResponse `json:"diagnostics"`
	Decision    *decisionResponse    `json:"decision,omitempty"`
}

type automataPayload struct {
	Pattern string `json:"pattern"`
	Input   string `json:"input"`
}

type graphNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Start  bool   `json:"start"`
	Accept bool   `json:"accept"`
}

type graphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Label   string `json:"label"`
	Epsilon bool   `json:"epsilon,omitempty"`
}

type graphResponse struct {
	States    int         `json:"states"`
	Accepting int         `json:"accepting"`
	Alphabet  []string    `json:"alphabet"`
	Start     string      `json:"start"`
	Nodes     []graphNode `json:"nodes"`
	Edges     []graphEdge `json:"edges"`
}

type simStep struct {
	StepIndex    int      `json:"step_index"`
	Symbol       string   `json:"symbol"`
	ActiveStates []string `json:"active_states"`
	Description  string   `json:"description"`
}

type automataResponse struct {
	Valid       bool                 `json:"valid"`
	Pattern     string               `json:"pattern"`
	Input       string               `json:"input,omitempty"`
	Accepted    bool                 `json:"accepted"`
	Diagnostics []diagnosticResponse `json:"diagnostics"`
	NFA         *graphResponse       `json:"nfa,omitempty"`
	DFA         *graphResponse       `json:"dfa,omitempty"`
	Minimized   *graphResponse       `json:"minimized,omitempty"`
	NFASimSteps []simStep            `json:"nfa_sim_steps,omitempty"`
	DFASimSteps []simStep            `json:"dfa_sim_steps,omitempty"`
}

type exampleResponse struct {
	ID          string         `json:"id"`
	Category    string         `json:"category"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Source      string         `json:"source"`
	Request     requestPayload `json:"request"`
}

var exampleData = []exampleResponse{
	{
		ID:          "role-access",
		Category:    "Policy",
		Name:        "Role-Based Resource Access",
		Description: "A support user accesses a numeric user endpoint with GET.",
		Source: `policy UserAccess {
  rule ReadUsers when
    method == "GET"
    and path matches "/api/users/[0-9]+"
    and role in ["admin", "support"]
  then allow;

  rule DeleteUsers when
    method == "DELETE"
  then deny 403 "Delete operation not permitted for role";

  default deny 401 "No matching authorization rule";
}`,
		Request: requestPayload{
			Method:     "GET",
			Path:       "/api/users/42",
			Host:       "api.sentinel.local",
			Scheme:     "https",
			Role:       "support",
			Headers:    map[string][]string{"X-Request-ID": {"req-101"}, "Authorization": {"Bearer token_xyz"}},
			Query:      map[string]string{"verbose": "true"},
			Fields:     map[string]string{"tier": "enterprise"},
			RemoteAddr: "192.168.1.50",
			UserAgent:  "SentinelClient/1.0",
		},
	},
	{
		ID:          "explicit-deny",
		Category:    "Policy",
		Name:        "Destructive Method Prohibition",
		Description: "A destructive DELETE request hits an explicit denial with custom HTTP 405 status.",
		Source: `policy SecurityGuard {
  rule ProhibitDestructive when
    method in ["DELETE", "PURGE"]
  then deny 405 "Destructive HTTP methods are prohibited on public gateway";

  rule AllowSafeRead when
    method == "GET"
  then allow;

  default deny 403 "Action forbidden";
}`,
		Request: requestPayload{
			Method:     "DELETE",
			Path:       "/api/v1/clusters/prod-db",
			Host:       "gateway.sentinel.local",
			Scheme:     "https",
			Role:       "developer",
			Headers:    map[string][]string{},
			Query:      map[string]string{},
			Fields:     map[string]string{},
			RemoteAddr: "10.0.0.12",
			UserAgent:  "curl/8.4.0",
		},
	},
	{
		ID:          "default-deny",
		Category:    "Policy",
		Name:        "Default Posture Fallback",
		Description: "An unauthenticated guest requesting an unmapped resource falls through to default deny.",
		Source: `policy ProductionAPI {
  rule AdminMetrics when
    role == "admin"
    and path matches "/metrics.*"
  then allow;

  rule InternalHealth when
    remote_addr == "127.0.0.1"
    and path == "/healthz"
  then allow;

  default deny 403 "Unauthorized: Access not granted by any rule";
}`,
		Request: requestPayload{
			Method:     "GET",
			Path:       "/metrics/cpu",
			Host:       "api.sentinel.local",
			Scheme:     "https",
			Role:       "guest",
			Headers:    map[string][]string{},
			Query:      map[string]string{},
			Fields:     map[string]string{},
			RemoteAddr: "203.0.113.195",
			UserAgent:  "Mozilla/5.0",
		},
	},
	{
		ID:          "multi-attribute",
		Category:    "Policy",
		Name:        "Multi-Attribute ABAC Rule",
		Description: "Evaluates combination of method, regex path, role set, custom fields, and host.",
		Source: `policy CorporateABAC {
  rule PremiumBillingAccess when
    method in ["GET", "POST"]
    and path matches "/api/billing/(invoices|statements)/[0-9]+"
    and (role == "finance" or role == "billing-admin")
    and host == "secure.sentinel.corp"
  then allow;

  rule StandardInquiries when
    method == "GET"
    and path matches "/api/public/.*"
  then allow;

  default deny 403 "Corporate ABAC policy denied request";
}`,
		Request: requestPayload{
			Method:     "POST",
			Path:       "/api/billing/invoices/98712",
			Host:       "secure.sentinel.corp",
			Scheme:     "https",
			Role:       "finance",
			Headers:    map[string][]string{"X-Tenant-ID": {"corp-alpha"}},
			Query:      map[string]string{"format": "pdf"},
			Fields:     map[string]string{"region": "us-east"},
			RemoteAddr: "172.16.0.4",
			UserAgent:  "InternalFinApp/2.4",
		},
	},
	{
		ID:          "syntax-error-demo",
		Category:    "Diagnostics",
		Name:        "Syntax & Semantic Diagnostics",
		Description: "Demonstrates exact source-span error reporting for unknown fields, missing actions, and unclosed tokens.",
		Source: `policy BrokenExample {
  rule InvalidField when
    unknown_attr == "test"
    and method == "GET"
  then allow;

  rule MissingThen when
    path == "/demo"

  default allow;
}`,
		Request: requestPayload{
			Method:  "GET",
			Path:    "/demo",
			Host:    "localhost",
			Scheme:  "http",
			Role:    "guest",
			Headers: map[string][]string{},
			Query:   map[string]string{},
			Fields:  map[string]string{},
		},
	},
	{
		ID:          "regex-routing",
		Category:    "Automata",
		Name:        "Regex Path Pattern: API Versions",
		Description: "Regular expression demonstrating Thompson NFA, Subset Construction DFA, and Hopcroft Minimization.",
		Source: `policy RegexRouting {
  rule VersionedEndpoints when
    path matches "/v(1|2)/(users|teams)/[a-z]+"
  then allow;

  default deny 404 "Route not found";
}`,
		Request: requestPayload{
			Method:  "GET",
			Path:    "/v1/users/alice",
			Host:    "api.sentinel.local",
			Scheme:  "https",
			Role:    "user",
			Headers: map[string][]string{},
			Query:   map[string]string{},
			Fields:  map[string]string{},
		},
	},
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "sentinel-api",
		"version": "1.0.0",
	})
}

func examples(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, exampleData)
}

func example(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, item := range exampleData {
		if item.ID == id {
			writeJSON(w, http.StatusOK, item)
			return
		}
	}
	writeError(w, http.StatusNotFound, "example not found")
}

func validate(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Source string `json:"source"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	policy, ds := compile(payload.Source)
	response := validateResponse{
		Valid:       !ds.HasErrors(),
		Diagnostics: diagnosticsJSON(ds),
	}
	if policy != nil {
		response.Policy = summarize(policy)
	}
	writeJSON(w, http.StatusOK, response)
}

func evaluate(w http.ResponseWriter, r *http.Request) { evaluateWithTrace(w, r, false) }
func explain(w http.ResponseWriter, r *http.Request)  { evaluateWithTrace(w, r, true) }

type batchEvaluatePayload struct {
	Source   string           `json:"source"`
	Requests []requestPayload `json:"requests"`
	Explain  bool             `json:"explain"`
}

type batchEvaluateResponse struct {
	Valid       bool                 `json:"valid"`
	Diagnostics []diagnosticResponse `json:"diagnostics"`
	Results     []evaluateResponse   `json:"results,omitempty"`
}

// evaluateBatch compiles the policy once and evaluates a bounded batch of
// requests. It is useful for the test matrix and avoids six round trips for a
// single suite run.
func evaluateBatch(w http.ResponseWriter, r *http.Request) {
	var payload batchEvaluatePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if len(payload.Requests) > 1000 {
		writeError(w, http.StatusRequestEntityTooLarge, "batch is limited to 1000 requests")
		return
	}
	policy, ds := compile(payload.Source)
	response := batchEvaluateResponse{Valid: !ds.HasErrors(), Diagnostics: diagnosticsJSON(ds)}
	if policy == nil || ds.HasErrors() {
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Results = make([]evaluateResponse, 0, len(payload.Requests))
	for _, request := range payload.Requests {
		result := evaluator.Evaluate(policy, request.request())
		decision := &decisionResponse{Allowed: result.Allowed, Decision: "DENY", Default: result.Default, Status: result.Action.Status, Reason: result.Action.Message}
		if result.Allowed {
			decision.Decision = "ALLOW"
		}
		if result.Rule != nil {
			decision.Rule = result.Rule.Name
		}
		if payload.Explain {
			for _, item := range result.Trace {
				decision.Trace = append(decision.Trace, traceResponse{Rule: item.Rule, Matched: item.Matched, Detail: item.Detail})
			}
		}
		response.Results = append(response.Results, evaluateResponse{
			Valid:       true,
			Diagnostics: []diagnosticResponse{},
			Decision:    decision,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func evaluateWithTrace(w http.ResponseWriter, r *http.Request, includeTrace bool) {
	var payload evaluatePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	policy, ds := compile(payload.Source)
	response := evaluateResponse{
		Valid:       !ds.HasErrors(),
		Diagnostics: diagnosticsJSON(ds),
	}
	if response.Valid && policy != nil {
		result := evaluator.Evaluate(policy, payload.Request.request())
		decision := &decisionResponse{
			Allowed:  result.Allowed,
			Decision: "DENY",
			Default:  result.Default,
			Status:   result.Action.Status,
			Reason:   result.Action.Message,
		}
		if result.Allowed {
			decision.Decision = "ALLOW"
		}
		if result.Rule != nil {
			decision.Rule = result.Rule.Name
		}
		if includeTrace {
			for _, item := range result.Trace {
				decision.Trace = append(decision.Trace, traceResponse{
					Rule:    item.Rule,
					Matched: item.Matched,
					Detail:  item.Detail,
				})
			}
		}
		response.Decision = decision
	}
	writeJSON(w, http.StatusOK, response)
}

func astHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Source string `json:"source"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	policy, ds := compile(payload.Source)
	resp := map[string]any{
		"valid":       !ds.HasErrors(),
		"diagnostics": diagnosticsJSON(ds),
	}
	if policy != nil {
		resp["policy"] = summarize(policy)
	}
	writeJSON(w, http.StatusOK, resp)
}

type lexerTokenResponse struct {
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Value string `json:"value,omitempty"`
	Start point  `json:"start"`
	End   point  `json:"end"`
}

// lexerHandler exposes the same lexer used by the compiler so visualizations
// never drift from the actual policy language implementation.
func lexerHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Source string `json:"source"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	tokens, ds := lexer.Lex(payload.Source)
	response := struct {
		Valid       bool                 `json:"valid"`
		Diagnostics []diagnosticResponse `json:"diagnostics"`
		Tokens      []lexerTokenResponse `json:"tokens"`
	}{Valid: !ds.HasErrors(), Diagnostics: diagnosticsJSON(ds), Tokens: make([]lexerTokenResponse, 0, len(tokens))}
	for _, token := range tokens {
		response.Tokens = append(response.Tokens, lexerTokenResponse{
			Kind: token.Kind.String(), Text: token.Text, Value: token.Value,
			Start: point{Offset: token.Span.Start.Offset, Line: token.Span.Start.Line, Column: token.Span.Start.Column},
			End:   point{Offset: token.Span.End.Offset, Line: token.Span.End.Line, Column: token.Span.End.Column},
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func automataHandler(w http.ResponseWriter, r *http.Request) {
	var payload automataPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	response := automataResponse{
		Pattern: payload.Pattern,
		Input:   payload.Input,
	}
	expression, err := regex.Parse(payload.Pattern)
	if err != nil {
		response.Diagnostics = []diagnosticResponse{{
			Severity: "error",
			Message:  err.Error(),
		}}
		writeJSON(w, http.StatusOK, response)
		return
	}

	nfa := regex.Thompson(expression)
	theoryNFA := toTheoryNFA(nfa)
	dfa := theoryNFA.SubsetConstruction()
	minimized := dfa.Minimize()

	symbols := strings.Split(payload.Input, "")
	if payload.Input == "" {
		symbols = []string{}
	}

	response.Valid = true
	response.Accepted = nfa.Run(symbols)
	response.NFA = nfaGraph(nfa)
	response.DFA = dfaGraph(dfa)
	response.Minimized = dfaGraph(minimized)

	// Step-by-step NFA simulation
	currentNFASet := nfa.EpsilonClosure(map[int]bool{nfa.Start: true})
	var nfaSteps []simStep
	nfaSteps = append(nfaSteps, simStep{
		StepIndex:    0,
		Symbol:       "START",
		ActiveStates: intSetToSortedStrings(currentNFASet),
		Description:  fmt.Sprintf("Start state ε-closure: %v", intSetToSortedStrings(currentNFASet)),
	})
	for i, sym := range symbols {
		nextSet := map[int]bool{}
		for s := range currentNFASet {
			for _, target := range nfa.Transitions[s][sym] {
				nextSet[target] = true
			}
		}
		currentNFASet = nfa.EpsilonClosure(nextSet)
		nfaSteps = append(nfaSteps, simStep{
			StepIndex:    i + 1,
			Symbol:       sym,
			ActiveStates: intSetToSortedStrings(currentNFASet),
			Description:  fmt.Sprintf("Consumed %q → Active: %v", sym, intSetToSortedStrings(currentNFASet)),
		})
	}
	response.NFASimSteps = nfaSteps

	// Step-by-step DFA simulation
	currDFAState := dfa.Start
	var dfaSteps []simStep
	dfaSteps = append(dfaSteps, simStep{
		StepIndex:    0,
		Symbol:       "START",
		ActiveStates: []string{currDFAState},
		Description:  fmt.Sprintf("Start state: %s", currDFAState),
	})
	for i, sym := range symbols {
		nextState, ok := dfa.Transitions[currDFAState][sym]
		if !ok {
			dfaSteps = append(dfaSteps, simStep{
				StepIndex:    i + 1,
				Symbol:       sym,
				ActiveStates: []string{},
				Description:  fmt.Sprintf("No transition on %q from %s (REJECT)", sym, currDFAState),
			})
			break
		}
		currDFAState = nextState
		dfaSteps = append(dfaSteps, simStep{
			StepIndex:    i + 1,
			Symbol:       sym,
			ActiveStates: []string{currDFAState},
			Description:  fmt.Sprintf("Consumed %q → %s (Accepting: %v)", sym, currDFAState, dfa.Accept[currDFAState]),
		})
	}
	response.DFASimSteps = dfaSteps

	writeJSON(w, http.StatusOK, response)
}

// CFG Handler
type cfgPayload struct {
	Variables   []string         `json:"variables"`
	Terminals   []string         `json:"terminals"`
	Start       string           `json:"start"`
	Productions []cfg.Production `json:"productions"`
	MaxSteps    int              `json:"max_steps"`
}

func cfgHandler(w http.ResponseWriter, r *http.Request) {
	var payload cfgPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	g := cfg.Grammar{
		Variables:   payload.Variables,
		Terminals:   payload.Terminals,
		Start:       payload.Start,
		Productions: payload.Productions,
	}
	if err := g.Validate(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}

	maxSteps := payload.MaxSteps
	if maxSteps <= 0 || maxSteps > 20 {
		maxSteps = 8
	}

	firstRaw := g.First()
	followRaw := g.Follow()
	simplified := g.Simplify()
	derivations := g.LeftmostDerivations(maxSteps)

	firstClean := map[string][]string{}
	for v, set := range firstRaw {
		firstClean[v] = cfg.Sorted(set)
	}
	followClean := map[string][]string{}
	for v, set := range followRaw {
		followClean[v] = cfg.Sorted(set)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":       true,
		"first":       firstClean,
		"follow":      followClean,
		"derivations": derivations,
		"simplified":  simplified,
	})
}

// PDA Handler
type pdaPayload struct {
	States             []string            `json:"states"`
	InputAlphabet      []string            `json:"input_alphabet"`
	StackAlphabet      []string            `json:"stack_alphabet"`
	Start              string              `json:"start"`
	InitialStack       string              `json:"initial_stack"`
	AcceptStates       []string            `json:"accept_states"`
	AcceptByFinalState bool                `json:"accept_by_final_state"`
	AcceptByEmptyStack bool                `json:"accept_by_empty_stack"`
	Transitions        []pdaTransitionItem `json:"transitions"`
	Input              []string            `json:"input"`
	MaxSteps           int                 `json:"max_steps"`
}

type pdaTransitionItem struct {
	FromState string   `json:"from_state"`
	Input     string   `json:"input"`
	StackTop  string   `json:"stack_top"`
	ToState   string   `json:"to_state"`
	Push      []string `json:"push"`
}

func pdaHandler(w http.ResponseWriter, r *http.Request) {
	var payload pdaPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := validatePDAPayload(payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	acceptMap := map[string]bool{}
	for _, s := range payload.AcceptStates {
		acceptMap[s] = true
	}
	transMap := map[pda.TransitionKey][]pda.Transition{}
	for _, t := range payload.Transitions {
		k := pda.TransitionKey{State: t.FromState, Input: t.Input, StackTop: t.StackTop}
		transMap[k] = append(transMap[k], pda.Transition{ToState: t.ToState, Push: t.Push})
	}
	p := pda.Automaton{
		States:             payload.States,
		InputAlphabet:      payload.InputAlphabet,
		StackAlphabet:      payload.StackAlphabet,
		Start:              payload.Start,
		InitialStack:       payload.InitialStack,
		AcceptStates:       acceptMap,
		AcceptByFinalState: payload.AcceptByFinalState,
		AcceptByEmptyStack: payload.AcceptByEmptyStack,
		Transitions:        transMap,
	}
	if err := p.Validate(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}

	maxSteps := payload.MaxSteps
	if maxSteps <= 0 || maxSteps > 100 {
		maxSteps = 50
	}
	result := p.Run(payload.Input, maxSteps)
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":     true,
		"accepted":  result.Accepted,
		"timed_out": result.TimedOut,
		"steps":     result.Steps,
		"trace":     result.Trace,
	})
}

func validatePDAPayload(payload pdaPayload) error {
	states, err := uniqueSymbols("states", payload.States)
	if err != nil {
		return err
	}
	if !states[payload.Start] {
		return fmt.Errorf("start state %q is not declared", payload.Start)
	}
	if payload.InitialStack == "" || !contains(payload.StackAlphabet, payload.InitialStack) {
		return fmt.Errorf("initial stack symbol %q is not in stack_alphabet", payload.InitialStack)
	}
	if !payload.AcceptByFinalState && !payload.AcceptByEmptyStack {
		return fmt.Errorf("at least one PDA acceptance mode must be enabled")
	}
	inputAlphabet, err := symbolSet("input_alphabet", payload.InputAlphabet)
	if err != nil {
		return err
	}
	stackAlphabet, err := symbolSet("stack_alphabet", payload.StackAlphabet)
	if err != nil {
		return err
	}
	for _, symbol := range payload.Input {
		if !inputAlphabet[symbol] {
			return fmt.Errorf("input symbol %q is not in input_alphabet", symbol)
		}
	}
	for index, transition := range payload.Transitions {
		if !states[transition.FromState] {
			return fmt.Errorf("transition %d references an undeclared state", index+1)
		}
		if transition.Input != "" && !inputAlphabet[transition.Input] {
			return fmt.Errorf("transition %d input %q is not in input_alphabet", index+1, transition.Input)
		}
		if !stackAlphabet[transition.StackTop] {
			return fmt.Errorf("transition %d stack_top %q is not in stack_alphabet", index+1, transition.StackTop)
		}
		for _, symbol := range transition.Push {
			if !stackAlphabet[symbol] {
				return fmt.Errorf("transition %d push symbol %q is not in stack_alphabet", index+1, symbol)
			}
		}
	}
	for state := range mapFromSlice(payload.AcceptStates) {
		if !states[state] {
			return fmt.Errorf("accept state %q is not declared", state)
		}
	}
	return nil
}

// TM Handler
type tmPayload struct {
	States       []string           `json:"states"`
	Alphabet     []string           `json:"alphabet"`
	TapeAlphabet []string           `json:"tape_alphabet"`
	Blank        string             `json:"blank"`
	Start        string             `json:"start"`
	AcceptStates []string           `json:"accept_states"`
	RejectStates []string           `json:"reject_states"`
	Transitions  []tmTransitionItem `json:"transitions"`
	Input        []string           `json:"input"`
	MaxSteps     int                `json:"max_steps"`
}

type tmTransitionItem struct {
	FromState string `json:"from_state"`
	Read      string `json:"read"`
	ToState   string `json:"to_state"`
	Write     string `json:"write"`
	Move      int    `json:"move"` // -1, 0, 1
}

func tmHandler(w http.ResponseWriter, r *http.Request) {
	var payload tmPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := validateTMPayload(payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	acceptMap := map[string]bool{}
	for _, s := range payload.AcceptStates {
		acceptMap[s] = true
	}
	rejectMap := map[string]bool{}
	for _, s := range payload.RejectStates {
		rejectMap[s] = true
	}
	transMap := map[tm.TransitionKey]tm.Transition{}
	for _, t := range payload.Transitions {
		k := tm.TransitionKey{State: t.FromState, Read: t.Read}
		transMap[k] = tm.Transition{
			ToState: t.ToState,
			Write:   t.Write,
			Move:    tm.Move(t.Move),
		}
	}
	m := tm.Machine{
		States:       payload.States,
		Alphabet:     payload.Alphabet,
		TapeAlphabet: payload.TapeAlphabet,
		Blank:        payload.Blank,
		Start:        payload.Start,
		AcceptStates: acceptMap,
		RejectStates: rejectMap,
		Transitions:  transMap,
	}
	if err := m.Validate(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}

	maxSteps := payload.MaxSteps
	if maxSteps <= 0 || maxSteps > 200 {
		maxSteps = 100
	}
	result := m.Run(payload.Input, maxSteps, true)
	if result.Err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": result.Err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":         true,
		"accepted":      result.Accepted,
		"rejected":      result.Rejected,
		"halted":        result.Halted,
		"timed_out":     result.TimedOut,
		"steps":         result.Steps,
		"configuration": result.Configuration,
		"trace":         result.Trace,
	})
}

func validateTMPayload(payload tmPayload) error {
	states, err := uniqueSymbols("states", payload.States)
	if err != nil {
		return err
	}
	if !states[payload.Start] {
		return fmt.Errorf("start state %q is not declared", payload.Start)
	}
	tapeAlphabet, err := symbolSet("tape_alphabet", payload.TapeAlphabet)
	if err != nil {
		return err
	}
	if !tapeAlphabet[payload.Blank] {
		return fmt.Errorf("blank %q is not in tape_alphabet", payload.Blank)
	}
	inputAlphabet, err := symbolSet("alphabet", payload.Alphabet)
	if err != nil {
		return err
	}
	for symbol := range inputAlphabet {
		if !tapeAlphabet[symbol] {
			return fmt.Errorf("alphabet symbol %q is not in tape_alphabet", symbol)
		}
	}
	if err := validateAcceptRejectStates(states, payload.AcceptStates, payload.RejectStates); err != nil {
		return err
	}
	seen := map[tm.TransitionKey]bool{}
	for index, transition := range payload.Transitions {
		if transition.Move < -1 || transition.Move > 1 {
			return fmt.Errorf("transition %d move must be -1, 0, or 1", index+1)
		}
		if !states[transition.FromState] || !tapeAlphabet[transition.Read] || !tapeAlphabet[transition.Write] {
			return fmt.Errorf("transition %d references an undeclared state or tape symbol", index+1)
		}
		key := tm.TransitionKey{State: transition.FromState, Read: transition.Read}
		if seen[key] {
			return fmt.Errorf("duplicate deterministic transition from %q reading %q", transition.FromState, transition.Read)
		}
		seen[key] = true
	}
	return nil
}

func validateAcceptRejectStates(states map[string]bool, accepts, rejects []string) error {
	seenAccept := map[string]bool{}
	for _, state := range accepts {
		if !states[state] {
			return fmt.Errorf("halting state %q is not declared", state)
		}
		if seenAccept[state] {
			return fmt.Errorf("halting state %q is declared more than once", state)
		}
		seenAccept[state] = true
	}
	seenReject := map[string]bool{}
	for _, state := range rejects {
		if !states[state] {
			return fmt.Errorf("halting state %q is not declared", state)
		}
		if seenReject[state] {
			return fmt.Errorf("halting state %q is declared more than once", state)
		}
		seenReject[state] = true
	}
	for _, accept := range accepts {
		if seenReject[accept] {
			return fmt.Errorf("state %q cannot be both accepting and rejecting", accept)
		}
	}
	return nil
}

// LBA Handler
type lbaPayload struct {
	States       []string            `json:"states"`
	Alphabet     []string            `json:"alphabet"`
	TapeAlphabet []string            `json:"tape_alphabet"`
	Blank        string              `json:"blank"`
	Start        string              `json:"start"`
	AcceptStates []string            `json:"accept_states"`
	RejectStates []string            `json:"reject_states"`
	Transitions  []lbaTransitionItem `json:"transitions"`
	Input        []string            `json:"input"`
	MaxSteps     int                 `json:"max_steps"`
}

type lbaTransitionItem struct {
	FromState string `json:"from_state"`
	Read      string `json:"read"`
	ToState   string `json:"to_state"`
	Write     string `json:"write"`
	Move      int    `json:"move"`
}

func lbaHandler(w http.ResponseWriter, r *http.Request) {
	var payload lbaPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := validateLBAPayload(payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	acceptMap := map[string]bool{}
	for _, s := range payload.AcceptStates {
		acceptMap[s] = true
	}
	rejectMap := map[string]bool{}
	for _, s := range payload.RejectStates {
		rejectMap[s] = true
	}
	transMap := map[lba.TransitionKey]lba.Transition{}
	for _, t := range payload.Transitions {
		k := lba.TransitionKey{State: t.FromState, Read: t.Read}
		transMap[k] = lba.Transition{
			ToState: t.ToState,
			Write:   t.Write,
			Move:    lba.Move(t.Move),
		}
	}
	m := lba.Machine{
		States:       payload.States,
		Alphabet:     payload.Alphabet,
		TapeAlphabet: payload.TapeAlphabet,
		Blank:        payload.Blank,
		Start:        payload.Start,
		AcceptStates: acceptMap,
		RejectStates: rejectMap,
		Transitions:  transMap,
	}
	if err := m.Validate(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}

	maxSteps := payload.MaxSteps
	if maxSteps <= 0 || maxSteps > 100 {
		maxSteps = 50
	}
	result := m.Run(payload.Input, maxSteps)
	if result.Err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": result.Err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":         true,
		"accepted":      result.Accepted,
		"rejected":      result.Rejected,
		"halted":        result.Halted,
		"timed_out":     result.TimedOut,
		"steps":         result.Steps,
		"configuration": result.Configuration,
		"trace":         result.Trace,
	})
}

func validateLBAPayload(payload lbaPayload) error {
	states, err := uniqueSymbols("states", payload.States)
	if err != nil {
		return err
	}
	if !states[payload.Start] {
		return fmt.Errorf("start state %q is not declared", payload.Start)
	}
	tapeAlphabet, err := symbolSet("tape_alphabet", payload.TapeAlphabet)
	if err != nil {
		return err
	}
	if !tapeAlphabet[payload.Blank] {
		return fmt.Errorf("blank %q is not in tape_alphabet", payload.Blank)
	}
	inputAlphabet, err := symbolSet("alphabet", payload.Alphabet)
	if err != nil {
		return err
	}
	for symbol := range inputAlphabet {
		if !tapeAlphabet[symbol] {
			return fmt.Errorf("alphabet symbol %q is not in tape_alphabet", symbol)
		}
	}
	if err := validateAcceptRejectStates(states, payload.AcceptStates, payload.RejectStates); err != nil {
		return err
	}
	seen := map[lba.TransitionKey]bool{}
	for index, transition := range payload.Transitions {
		if transition.Move < -1 || transition.Move > 1 {
			return fmt.Errorf("transition %d move must be -1, 0, or 1", index+1)
		}
		if !states[transition.FromState] || !tapeAlphabet[transition.Read] || !tapeAlphabet[transition.Write] {
			return fmt.Errorf("transition %d references an undeclared state or tape symbol", index+1)
		}
		key := lba.TransitionKey{State: transition.FromState, Read: transition.Read}
		if seen[key] {
			return fmt.Errorf("duplicate deterministic transition from %q reading %q", transition.FromState, transition.Read)
		}
		seen[key] = true
	}
	for _, symbol := range payload.Input {
		if !inputAlphabet[symbol] {
			return fmt.Errorf("input symbol %q is not in alphabet", symbol)
		}
	}
	return nil
}

// CSG Handler
type csgPayload struct {
	Variables   []string         `json:"variables"`
	Terminals   []string         `json:"terminals"`
	Start       string           `json:"start"`
	Productions []csg.Production `json:"productions"`
	Input       []string         `json:"input"`
	MaxSteps    int              `json:"max_steps"`
	MaxLength   int              `json:"max_length"`
}

func csgHandler(w http.ResponseWriter, r *http.Request) {
	var payload csgPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	g := csg.Grammar{
		Variables:   payload.Variables,
		Terminals:   payload.Terminals,
		Start:       payload.Start,
		Productions: payload.Productions,
	}
	if err := validateCSG(g); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	if err := validateCSGInput(g, payload.Input); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	maxSteps := payload.MaxSteps
	if maxSteps <= 0 || maxSteps > 50 {
		maxSteps = 20
	}
	maxLength := payload.MaxLength
	if maxLength <= 0 {
		maxLength = len(payload.Input)
		if maxLength == 0 {
			maxLength = 1
		}
	}
	if maxLength > 100 {
		maxLength = 100
	}
	result := g.Recognize(payload.Input, csg.Options{MaxLength: maxLength, MaxSteps: maxSteps})
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":      true,
		"accepted":   result.Accepted,
		"steps":      result.Steps,
		"derivation": result.Derivation,
	})
}

func validateCSG(g csg.Grammar) error {
	variables := make(map[string]bool, len(g.Variables))
	terminals := make(map[string]bool, len(g.Terminals))
	for _, variable := range g.Variables {
		if variable == "" || variables[variable] {
			return fmt.Errorf("variables must be non-empty and unique")
		}
		variables[variable] = true
	}
	for _, terminal := range g.Terminals {
		if terminal == "" || terminals[terminal] || variables[terminal] {
			return fmt.Errorf("terminals must be non-empty and disjoint from variables")
		}
		terminals[terminal] = true
	}
	if !variables[g.Start] {
		return fmt.Errorf("start variable %q is not declared", g.Start)
	}
	for index, production := range g.Productions {
		if len(production.LHS) == 0 {
			return fmt.Errorf("production %d has an empty left-hand side", index+1)
		}
		for _, symbol := range append(append([]string{}, production.LHS...), production.RHS...) {
			if !variables[symbol] && !terminals[symbol] {
				return fmt.Errorf("production %d references undeclared symbol %q", index+1, symbol)
			}
		}
	}
	return nil
}

func validateCSGInput(g csg.Grammar, input []string) error {
	terminals := mapFromSlice(g.Terminals)
	for _, symbol := range input {
		if !terminals[symbol] {
			return fmt.Errorf("input symbol %q is not a declared terminal", symbol)
		}
	}
	return nil
}

func uniqueSymbols(name string, values []string) (map[string]bool, error) {
	result := make(map[string]bool, len(values))
	if len(values) == 0 {
		return nil, fmt.Errorf("%s must contain at least one symbol", name)
	}
	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("%s cannot contain empty symbols", name)
		}
		if result[value] {
			return nil, fmt.Errorf("%s contains duplicate symbol %q", name, value)
		}
		result[value] = true
	}
	return result, nil
}

func symbolSet(name string, values []string) (map[string]bool, error) {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" {
			return nil, fmt.Errorf("%s cannot contain empty symbols", name)
		}
		if result[value] {
			return nil, fmt.Errorf("%s contains duplicate symbol %q", name, value)
		}
		result[value] = true
	}
	return result, nil
}

func mapFromSlice(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func compile(source string) (*ast.Policy, diagnostics.Diagnostics) {
	policy, ds := parser.Parse(source)
	if !ds.HasErrors() && policy != nil {
		ds = append(ds, semantic.Validate(policy)...)
	}
	return policy, ds
}

func summarize(policy *ast.Policy) *policySummary {
	if policy == nil {
		return nil
	}
	span := policy.Span()
	result := &policySummary{
		Name:      policy.Name,
		RuleCount: len(policy.Rules),
		Rules:     make([]ruleSummary, 0, len(policy.Rules)),
		Start:     point{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column},
		End:       point{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column},
	}
	for _, rule := range policy.Rules {
		rSpan := rule.Span()
		item := ruleSummary{
			Name:      rule.Name,
			Action:    action(rule.Action),
			Condition: exprToJSON(rule.When),
			Start:     point{Offset: rSpan.Start.Offset, Line: rSpan.Start.Line, Column: rSpan.Start.Column},
			End:       point{Offset: rSpan.End.Offset, Line: rSpan.End.Line, Column: rSpan.End.Column},
		}
		result.Rules = append(result.Rules, item)
	}
	if policy.Default != nil {
		value := action(*policy.Default)
		result.Default = &value
	}
	return result
}

func action(value ast.Action) actionSummary {
	span := value.Span()
	return actionSummary{
		Kind:    value.Kind.String(),
		Status:  value.Status,
		Message: value.Message,
		Start:   point{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column},
		End:     point{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column},
	}
}

func exprToJSON(e ast.Expr) *astNodeJSON {
	if e == nil {
		return nil
	}
	span := e.Span()
	ptStart := point{Offset: span.Start.Offset, Line: span.Start.Line, Column: span.Start.Column}
	ptEnd := point{Offset: span.End.Offset, Line: span.End.Line, Column: span.End.Column}

	switch node := e.(type) {
	case ast.Identifier:
		return &astNodeJSON{Type: "Identifier", Value: node.Name, Start: ptStart, End: ptEnd}
	case ast.StringLiteral:
		return &astNodeJSON{Type: "StringLiteral", Value: node.Value, Start: ptStart, End: ptEnd}
	case ast.NumberLiteral:
		return &astNodeJSON{Type: "NumberLiteral", Value: fmt.Sprintf("%v", node.Value), Start: ptStart, End: ptEnd}
	case ast.BoolLiteral:
		return &astNodeJSON{Type: "BoolLiteral", Value: fmt.Sprintf("%v", node.Value), Start: ptStart, End: ptEnd}
	case ast.UnaryExpr:
		return &astNodeJSON{
			Type:     "UnaryExpr",
			Operator: node.Op.String(),
			Left:     exprToJSON(node.Expr),
			Start:    ptStart,
			End:      ptEnd,
		}
	case ast.BinaryExpr:
		return &astNodeJSON{
			Type:     "BinaryExpr",
			Operator: node.Op.String(),
			Left:     exprToJSON(node.Left),
			Right:    exprToJSON(node.Right),
			Start:    ptStart,
			End:      ptEnd,
		}
	case ast.CompareExpr:
		return &astNodeJSON{
			Type:     "CompareExpr",
			Operator: node.Op.String(),
			Left:     exprToJSON(node.Left),
			Right:    exprToJSON(node.Right),
			Start:    ptStart,
			End:      ptEnd,
		}
	case ast.ListExpr:
		var children []astNodeJSON
		for _, item := range node.Values {
			if childJSON := exprToJSON(item); childJSON != nil {
				children = append(children, *childJSON)
			}
		}
		return &astNodeJSON{
			Type:     "ListExpr",
			Children: children,
			Start:    ptStart,
			End:      ptEnd,
		}
	default:
		return &astNodeJSON{Type: "Unknown", Start: ptStart, End: ptEnd}
	}
}

func diagnosticsJSON(ds diagnostics.Diagnostics) []diagnosticResponse {
	result := make([]diagnosticResponse, 0, len(ds))
	for _, d := range ds {
		result = append(result, diagnosticResponse{
			Severity: d.Severity.String(),
			Message:  d.Message,
			Start: point{
				Offset: d.Span.Start.Offset,
				Line:   d.Span.Start.Line,
				Column: d.Span.Start.Column,
			},
			End: point{
				Offset: d.Span.End.Offset,
				Line:   d.Span.End.Line,
				Column: d.Span.End.Column,
			},
		})
	}
	return result
}

func intSetToSortedStrings(s map[int]bool) []string {
	var nums []int
	for n := range s {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var out []string
	for _, n := range nums {
		out = append(out, fmt.Sprintf("q%d", n))
	}
	return out
}

func nfaGraph(n regex.NFA) *graphResponse {
	result := &graphResponse{
		States:   len(n.States),
		Alphabet: append([]string{}, n.Alphabet...),
		Start:    fmt.Sprint(n.Start),
		Nodes:    make([]graphNode, 0, len(n.States)),
		Edges:    make([]graphEdge, 0),
	}
	for _, state := range n.States {
		id := fmt.Sprint(state)
		accept := state == n.Accept
		if accept {
			result.Accepting++
		}
		result.Nodes = append(result.Nodes, graphNode{
			ID:     id,
			Label:  "q" + id,
			Start:  state == n.Start,
			Accept: accept,
		})
	}
	for from, row := range n.Transitions {
		for label, targets := range row {
			for _, to := range targets {
				result.Edges = append(result.Edges, graphEdge{
					From:  fmt.Sprint(from),
					To:    fmt.Sprint(to),
					Label: label,
				})
			}
		}
	}
	for from, targets := range n.Epsilon {
		for _, to := range targets {
			result.Edges = append(result.Edges, graphEdge{
				From:    fmt.Sprint(from),
				To:      fmt.Sprint(to),
				Label:   "ε",
				Epsilon: true,
			})
		}
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].From != result.Edges[j].From {
			return result.Edges[i].From < result.Edges[j].From
		}
		if result.Edges[i].To != result.Edges[j].To {
			return result.Edges[i].To < result.Edges[j].To
		}
		return result.Edges[i].Label < result.Edges[j].Label
	})
	return result
}

func dfaGraph(d automata.DFA) *graphResponse {
	result := &graphResponse{
		States:   len(d.States),
		Alphabet: append([]string{}, d.Alphabet...),
		Start:    d.Start,
		Nodes:    make([]graphNode, 0, len(d.States)),
		Edges:    make([]graphEdge, 0),
	}
	for _, state := range d.States {
		accept := d.Accept[state]
		if accept {
			result.Accepting++
		}
		result.Nodes = append(result.Nodes, graphNode{
			ID:     state,
			Label:  state,
			Start:  state == d.Start,
			Accept: accept,
		})
	}
	for from, row := range d.Transitions {
		for label, to := range row {
			result.Edges = append(result.Edges, graphEdge{
				From:  from,
				To:    to,
				Label: label,
			})
		}
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].From != result.Edges[j].From {
			return result.Edges[i].From < result.Edges[j].From
		}
		if result.Edges[i].To != result.Edges[j].To {
			return result.Edges[i].To < result.Edges[j].To
		}
		return result.Edges[i].Label < result.Edges[j].Label
	})
	return result
}

func toTheoryNFA(source regex.NFA) automata.NFA {
	n := automata.NFA{
		Start:       fmt.Sprint(source.Start),
		Accept:      map[string]bool{fmt.Sprint(source.Accept): true},
		Transitions: map[string]map[string][]string{},
		Epsilon:     map[string][]string{},
	}
	for _, state := range source.States {
		n.States = append(n.States, fmt.Sprint(state))
	}
	n.Alphabet = append(n.Alphabet, source.Alphabet...)
	for from, row := range source.Transitions {
		for symbol, targets := range row {
			for _, to := range targets {
				n.Transitions[fmt.Sprint(from)] = appendTransition(n.Transitions[fmt.Sprint(from)], symbol, fmt.Sprint(to))
			}
		}
	}
	for from, targets := range source.Epsilon {
		for _, to := range targets {
			n.Epsilon[fmt.Sprint(from)] = append(n.Epsilon[fmt.Sprint(from)], fmt.Sprint(to))
		}
	}
	return n
}

func appendTransition(row map[string][]string, symbol, target string) map[string][]string {
	if row == nil {
		row = map[string][]string{}
	}
	row[symbol] = append(row[symbol], target)
	return row
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	configured := os.Getenv("SENTINEL_ALLOWED_ORIGIN")
	if configured == "" {
		configured = "http://localhost:3000,http://127.0.0.1:3000"
	}
	for _, candidate := range strings.Split(configured, ",") {
		if strings.TrimSpace(candidate) == origin {
			return true
		}
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return false
		}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			writeError(w, http.StatusBadRequest, "request body must contain one JSON value")
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
