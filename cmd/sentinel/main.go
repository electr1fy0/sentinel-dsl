package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ayush/sentinel-dsl/internal/api"
	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/evaluator"
	"github.com/ayush/sentinel-dsl/internal/lexer"
	"github.com/ayush/sentinel-dsl/internal/middleware"
	"github.com/ayush/sentinel-dsl/internal/parser"
	"github.com/ayush/sentinel-dsl/internal/semantic"
	"github.com/ayush/sentinel-dsl/theory/automata"
	"github.com/ayush/sentinel-dsl/theory/regex"
)

var (
	osExit             = os.Exit
	httpListenAndServe = http.ListenAndServe
)

func main() {
	if len(os.Args) < 2 {
		usage()
		osExit(2)
		return
	}
	var err error
	switch os.Args[1] {
	case "check":
		err = commandCheck(os.Args[2:])
	case "compile":
		err = commandCompile(os.Args[2:])
	case "eval", "explain":
		err = commandEvaluate(os.Args[1] == "explain", os.Args[2:])
	case "automata":
		err = commandAutomata(os.Args[2:])
	case "serve":
		err = commandServe(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sentinel:", err)
		osExit(1)
		return
	}
}

func usage() {
	fmt.Print(`SentinelDSL policy compiler and theory laboratory

Usage:
  sentinel check policy.sntl
  sentinel compile policy.sntl
  sentinel eval policy.sntl request.json
  sentinel explain policy.sntl request.json
  sentinel automata build|minimize|test pattern [input]
  sentinel serve policy.sntl [addr]
`)
}

func loadPolicy(path string) (*ast.Policy, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	policy, syntax := parser.Parse(string(source))
	if len(syntax) > 0 {
		return nil, syntax
	}
	if checks := semantic.Validate(policy); len(checks) > 0 {
		return nil, checks
	}
	return policy, nil
}

func commandCheck(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: sentinel check policy.sntl")
	}
	source, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	tokens, syntax := lexer.Lex(string(source))
	policy, parseDiagnostics := parser.Parse(string(source))
	all := append(syntax, parseDiagnostics...)
	all = append(all, semantic.Validate(policy)...)
	if len(all) > 0 {
		return all
	}
	fmt.Printf("valid policy %q (%d tokens, %d rules)\n", policy.Name, len(tokens)-1, len(policy.Rules))
	return nil
}

func commandCompile(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: sentinel compile policy.sntl")
	}
	policy, err := loadPolicy(args[0])
	if err != nil {
		return err
	}
	result := struct {
		Name    string   `json:"name"`
		Rules   []string `json:"rules"`
		Default string   `json:"default"`
	}{Name: policy.Name}
	for _, rule := range policy.Rules {
		result.Rules = append(result.Rules, rule.Name)
	}
	if policy.Default != nil {
		result.Default = policy.Default.Kind.String()
	}
	return writeJSON(result)
}

type requestFile struct {
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

func (r requestFile) request() evaluator.Request {
	return evaluator.Request{Method: r.Method, Path: r.Path, Host: r.Host, Scheme: r.Scheme, RemoteAddr: r.RemoteAddr, UserAgent: r.UserAgent, Role: r.Role, Headers: http.Header(r.Headers), Query: r.Query, Fields: r.Fields}
}

func commandEvaluate(explain bool, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: sentinel eval|explain policy.sntl request.json")
	}
	policy, err := loadPolicy(args[0])
	if err != nil {
		return err
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var input requestFile
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	decision := evaluator.Evaluate(policy, input.request())
	output := struct {
		Allowed  bool                   `json:"allowed"`
		Decision string                 `json:"decision"`
		Rule     string                 `json:"rule,omitempty"`
		Default  bool                   `json:"default"`
		Status   int                    `json:"status,omitempty"`
		Reason   string                 `json:"reason,omitempty"`
		Trace    []evaluator.TraceEntry `json:"trace,omitempty"`
	}{Allowed: decision.Allowed, Default: decision.Default, Trace: decision.Trace}
	if decision.Allowed {
		output.Decision = "ALLOW"
	} else {
		output.Decision = "DENY"
	}
	output.Status, output.Reason = decision.Action.Status, decision.Action.Message
	if decision.Rule != nil {
		output.Rule = decision.Rule.Name
	}
	if !explain {
		output.Trace = nil
	}
	return writeJSON(output)
}

func commandAutomata(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: sentinel automata build|minimize|test pattern [input]")
	}
	expression, err := regex.Parse(args[1])
	if err != nil {
		return err
	}
	nfa := regex.Thompson(expression)
	dfa := toTheoryNFA(nfa).SubsetConstruction()
	switch args[0] {
	case "build":
		return writeJSON(struct {
			NFAStates int      `json:"nfa_states"`
			DFAStates int      `json:"dfa_states"`
			Alphabet  []string `json:"alphabet"`
		}{len(nfa.States), len(dfa.States), dfa.Alphabet})
	case "minimize":
		min := dfa.Minimize()
		return writeJSON(struct {
			Before int      `json:"before"`
			After  int      `json:"after"`
			States []string `json:"states"`
		}{len(dfa.States), len(min.States), min.States})
	case "test":
		if len(args) != 3 {
			return errors.New("usage: sentinel automata test pattern input")
		}
		accepted := nfa.Run(strings.Split(args[2], ""))
		fmt.Printf("%t\n", accepted)
		return nil
	default:
		return fmt.Errorf("unknown automata command %q", args[0])
	}
}

func toTheoryNFA(source regex.NFA) automata.NFA {
	n := automata.NFA{Start: fmt.Sprint(source.Start), Accept: map[string]bool{fmt.Sprint(source.Accept): true}, Transitions: map[string]map[string][]string{}, Epsilon: map[string][]string{}}
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

func commandServe(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return errors.New("usage: sentinel serve policy.sntl [addr]")
	}
	policy, err := loadPolicy(args[0])
	if err != nil {
		return err
	}
	addr := ":8080"
	if len(args) == 2 {
		addr = args[1]
	}
	protected := middleware.New(policy).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("allowed\n"))
	}))
	mux := http.NewServeMux()
	mux.Handle("/api/", api.Handler())
	mux.Handle("/", protected)
	fmt.Println("listening on", addr)
	return httpListenAndServe(addr, mux)
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
