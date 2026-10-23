package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/theory/regex"
)

func TestHealth(t *testing.T) {
	handler := Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestExamples(t *testing.T) {
	handler := Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/examples", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []exampleResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) < 3 {
		t.Errorf("expected >= 3 examples, got %d", len(resp))
	}

	// Test single example
	reqSingle := httptest.NewRequest(http.MethodGet, "/api/examples/role-access", nil)
	wSingle := httptest.NewRecorder()
	handler.ServeHTTP(wSingle, reqSingle)
	if wSingle.Code != http.StatusOK {
		t.Fatalf("expected 200 for role-access, got %d", wSingle.Code)
	}
}

func TestValidate(t *testing.T) {
	handler := Handler()
	body := []byte(`{"source": "policy Test { rule R1 when method == \"GET\" then allow; default deny; }"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp validateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Valid {
		t.Errorf("expected policy to be valid, got diagnostics: %v", resp.Diagnostics)
	}
	if resp.Policy == nil || resp.Policy.Name != "Test" {
		t.Errorf("unexpected policy summary: %v", resp.Policy)
	}
}

func TestEvaluateAndExplain(t *testing.T) {
	handler := Handler()
	payload := `{"source": "policy Test { rule R1 when method == \"GET\" and path == \"/test\" then allow; default deny \"Denied\"; }", "request": {"method": "GET", "path": "/test"}}`

	// Evaluate
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp evaluateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Valid || resp.Decision == nil || !resp.Decision.Allowed {
		t.Fatalf("expected allowed decision, got %v", resp.Decision)
	}

	// Explain
	reqExp := httptest.NewRequest(http.MethodPost, "/api/explain", strings.NewReader(payload))
	wExp := httptest.NewRecorder()
	handler.ServeHTTP(wExp, reqExp)
	var respExp evaluateResponse
	if err := json.NewDecoder(wExp.Body).Decode(&respExp); err != nil {
		t.Fatal(err)
	}
	if len(respExp.Decision.Trace) == 0 {
		t.Errorf("expected non-empty trace in explain response")
	}
}

func TestEvaluateBatch(t *testing.T) {
	handler := Handler()
	payload := `{"source":"policy Test { rule R1 when method == \"GET\" then allow; default deny; }", "explain":true, "requests":[{"method":"GET"},{"method":"POST"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate/batch", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response batchEvaluateResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || len(response.Results) != 2 || response.Results[0].Decision == nil || !response.Results[0].Decision.Allowed || response.Results[1].Decision == nil || response.Results[1].Decision.Allowed {
		t.Fatalf("unexpected batch response: %#v", response)
	}
}

func TestAutomata(t *testing.T) {
	handler := Handler()
	payload := `{"pattern": "a(b|c)*d", "input": "abbcd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/automata", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp automataResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Valid || !resp.Accepted {
		t.Errorf("expected valid and accepted automata run, got valid=%v accepted=%v", resp.Valid, resp.Accepted)
	}
	if resp.NFA == nil || resp.DFA == nil || resp.Minimized == nil {
		t.Errorf("expected NFA, DFA, and Minimized graphs to be non-nil")
	}
	if len(resp.NFASimSteps) == 0 || len(resp.DFASimSteps) == 0 {
		t.Errorf("expected simulation steps to be populated")
	}
}

func TestAutomataEmptyCollectionsAreJSONArrays(t *testing.T) {
	handler := Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/automata", strings.NewReader(`{"pattern":"","input":""}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response struct {
		NFA       json.RawMessage `json:"nfa"`
		DFA       json.RawMessage `json:"dfa"`
		Minimized json.RawMessage `json:"minimized"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]json.RawMessage{
		"nfa": response.NFA, "dfa": response.DFA, "minimized": response.Minimized,
	} {
		var graph map[string]json.RawMessage
		if err := json.Unmarshal(raw, &graph); err != nil {
			t.Fatalf("decode %s graph: %v", name, err)
		}
		for _, field := range []string{"alphabet", "nodes", "edges"} {
			got := string(graph[field])
			if len(got) == 0 || got[0] != '[' {
				t.Errorf("%s.%s = %s, want a JSON array", name, field, got)
			}
			if field == "alphabet" && got != "[]" {
				t.Errorf("%s.%s = %s, want []", name, field, got)
			}
		}
	}
}

func TestLexer(t *testing.T) {
	handler := Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/lexer", strings.NewReader(`{"source":"policy Demo { default allow; }"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response struct {
		Valid       bool                 `json:"valid"`
		Diagnostics []diagnosticResponse `json:"diagnostics"`
		Tokens      []struct {
			Kind string `json:"kind"`
		} `json:"tokens"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || len(response.Tokens) < 5 {
		t.Fatalf("expected valid token stream, got valid=%v tokens=%d diagnostics=%v", response.Valid, len(response.Tokens), response.Diagnostics)
	}
	if response.Tokens[0].Kind != "policy" {
		t.Errorf("expected first token to be policy, got %q", response.Tokens[0].Kind)
	}
}

func TestJSONRequestHardening(t *testing.T) {
	handler := Handler()
	wrongType := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(`{"source":"policy Demo {}"}`))
	wrongType.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, wrongType)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for unsupported content type, got %d", w.Code)
	}

	extraJSON := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(`{"source":"policy Demo {}"} {"source":"extra"}`))
	wr := httptest.NewRecorder()
	handler.ServeHTTP(wr, extraJSON)
	if wr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for multiple JSON values, got %d", wr.Code)
	}

	trailingInvalid := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(`{"source":"policy Demo {}"} invalid_trailing_token`))
	wti := httptest.NewRecorder()
	handler.ServeHTTP(wti, trailingInvalid)
	if wti.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for trailing invalid token, got %d", wti.Code)
	}
}

func TestConfiguredCORS(t *testing.T) {
	t.Setenv("SENTINEL_ALLOWED_ORIGIN", "https://studio.example, http://localhost:3000")
	handler := Handler()
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "https://studio.example")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 preflight, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://studio.example" {
		t.Errorf("expected configured origin, got %q", got)
	}

	req = httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "https://not-allowed.example")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected disallowed origin to be omitted, got %q", got)
	}
}

func TestTheoryCFG(t *testing.T) {
	handler := Handler()
	payload := `{
		"variables": ["S", "A"],
		"terminals": ["a", "b"],
		"start": "S",
		"productions": [
			{"lhs": "S", "rhs": ["a", "S", "b"]},
			{"lhs": "S", "rhs": []}
		],
		"max_steps": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/cfg", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true {
		t.Errorf("expected CFG response valid=true, got %v", resp)
	}
}

func TestTheoryPDA(t *testing.T) {
	handler := Handler()
	payload := `{
		"states": ["q0", "q1", "q_accept"],
		"input_alphabet": ["0", "1"],
		"stack_alphabet": ["Z", "0"],
		"start": "q0",
		"initial_stack": "Z",
		"accept_states": ["q_accept"],
		"accept_by_final_state": true,
		"transitions": [
			{"from_state": "q0", "input": "0", "stack_top": "Z", "to_state": "q_accept", "push": ["Z"]}
		],
		"input": ["0"],
		"max_steps": 10
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/pda", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true || resp["accepted"] != true {
		t.Errorf("expected PDA accepted=true, got %v", resp)
	}
}

func TestTheoryTM(t *testing.T) {
	handler := Handler()
	payload := `{
		"states": ["q0", "q_accept", "q_reject"],
		"alphabet": ["0", "1"],
		"tape_alphabet": ["0", "1", "_"],
		"blank": "_",
		"start": "q0",
		"accept_states": ["q_accept"],
		"reject_states": ["q_reject"],
		"transitions": [
			{"from_state": "q0", "read": "0", "to_state": "q_accept", "write": "1", "move": 1}
		],
		"input": ["0"],
		"max_steps": 10
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/tm", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true || resp["accepted"] != true {
		t.Errorf("expected TM accepted=true, got %v", resp)
	}
}

func TestTheoryLBA(t *testing.T) {
	handler := Handler()
	payload := `{
		"states": ["q0", "q_accept"],
		"alphabet": ["a", "b"],
		"tape_alphabet": ["a", "b", "_"],
		"blank": "_",
		"start": "q0",
		"accept_states": ["q_accept"],
		"reject_states": [],
		"transitions": [
			{"from_state": "q0", "read": "a", "to_state": "q_accept", "write": "a", "move": 0}
		],
		"input": ["a"],
		"max_steps": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/lba", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true || resp["accepted"] != true {
		t.Errorf("expected LBA accepted=true, got %v", resp)
	}
}

func TestTheoryLBARejectsInvalidDefinition(t *testing.T) {
	handler := Handler()
	payload := `{
		"states": ["q0", "q_accept"],
		"alphabet": ["a"],
		"tape_alphabet": ["a", "_"],
		"blank": "_",
		"start": "q0",
		"accept_states": ["q_accept"],
		"transitions": [{"from_state":"q0","read":"a","to_state":"q_accept","write":"a","move":2}],
		"input": ["a"], "max_steps": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/lba", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["valid"] != false || response["error"] == nil {
		t.Fatalf("expected invalid LBA response, got %#v", response)
	}
}

func TestTheoryCSG(t *testing.T) {
	handler := Handler()
	payload := `{
		"variables": ["S", "A", "B"],
		"terminals": ["a", "b"],
		"start": "S",
		"productions": [
			{"lhs": ["S"], "rhs": ["a", "b"]}
		],
		"input": ["a", "b"],
		"max_steps": 5,
		"max_length": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/csg", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["valid"] != true || resp["accepted"] != true {
		t.Errorf("expected CSG accepted=true, got %v", resp)
	}
}

func TestTheoryCSGRejectsUndeclaredSymbol(t *testing.T) {
	handler := Handler()
	payload := `{
		"variables": ["S"], "terminals": ["a"], "start": "S",
		"productions": [{"lhs":["S"],"rhs":["b"]}],
		"input": ["b"], "max_steps": 5, "max_length": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/csg", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["valid"] != false || response["error"] == nil {
		t.Fatalf("expected invalid CSG response, got %#v", response)
	}
}

func TestTheoryCSGRejectsUndeclaredInput(t *testing.T) {
	handler := Handler()
	payload := `{
		"variables": ["S"], "terminals": ["a"], "start": "S",
		"productions": [{"lhs":["S"],"rhs":["a"]}],
		"input": ["b"], "max_steps": 5, "max_length": 5
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/theory/csg", strings.NewReader(payload))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["valid"] != false || response["error"] == nil {
		t.Fatalf("expected undeclared input to be rejected, got %#v", response)
	}
}

func TestApiErrorsAndEdgeCases(t *testing.T) {
	handler := Handler()

	// 1. Example not found
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/examples/nonexistent", nil)
	wNotFound := httptest.NewRecorder()
	handler.ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent example, got %d", wNotFound.Code)
	}

	// 2. Decode errors across endpoints
	endpoints := []string{
		"/api/validate",
		"/api/evaluate",
		"/api/evaluate/batch",
		"/api/explain",
		"/api/ast",
		"/api/lexer",
		"/api/automata",
		"/api/theory/cfg",
		"/api/theory/pda",
		"/api/theory/tm",
		"/api/theory/lba",
		"/api/theory/csg",
	}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodPost, ep, strings.NewReader(`{invalid json`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for bad json on %s, got %d", ep, w.Code)
		}
	}

	// 3. Batch evaluate > 1000 requests & invalid policy
	reqHuge := httptest.NewRequest(http.MethodPost, "/api/evaluate/batch", strings.NewReader(`{"source":"policy p { default allow; }","requests":`+makeHugeRequestsJSON(1001)+`}`))
	wHuge := httptest.NewRecorder()
	handler.ServeHTTP(wHuge, reqHuge)
	if wHuge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for >1000 batch requests, got %d", wHuge.Code)
	}

	reqBadPolicy := httptest.NewRequest(http.MethodPost, "/api/evaluate/batch", strings.NewReader(`{"source":"broken policy","requests":[{"method":"GET"}]}`))
	wBadPolicy := httptest.NewRecorder()
	handler.ServeHTTP(wBadPolicy, reqBadPolicy)
	if wBadPolicy.Code != http.StatusOK {
		t.Fatalf("expected 200 with diagnostics for bad policy batch, got %d", wBadPolicy.Code)
	}

	// 4. AST Handler with all node types and invalid policy
	astSrc := `{"source": "policy AllNodes { rule r when not (method == \"GET\" and 10 < 20 or [\"a\"] in [\"b\"]) and true then allow 200 \"ok\"; default deny; }"}`
	reqAST := httptest.NewRequest(http.MethodPost, "/api/ast", strings.NewReader(astSrc))
	wAST := httptest.NewRecorder()
	handler.ServeHTTP(wAST, reqAST)
	if wAST.Code != http.StatusOK {
		t.Fatalf("expected 200 for ast handler, got %d", wAST.Code)
	}

	// 5. Automata with invalid pattern and DFA with no transition
	reqBadPattern := httptest.NewRequest(http.MethodPost, "/api/automata", strings.NewReader(`{"pattern":"(unclosed","input":"a"}`))
	wBadPattern := httptest.NewRecorder()
	handler.ServeHTTP(wBadPattern, reqBadPattern)
	var respBadPat automataResponse
	_ = json.NewDecoder(wBadPattern.Body).Decode(&respBadPat)
	if len(respBadPat.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for broken regex in automata")
	}

	// Automata DFA step with no transition on symbol and edge sorting
	reqDeadStep := httptest.NewRequest(http.MethodPost, "/api/automata", strings.NewReader(`{"pattern":"a|a","input":"ab"}`))
	wDeadStep := httptest.NewRecorder()
	handler.ServeHTTP(wDeadStep, reqDeadStep)
	var respDeadStep automataResponse
	_ = json.NewDecoder(wDeadStep.Body).Decode(&respDeadStep)
	if respDeadStep.Accepted {
		t.Fatalf("expected rejected for pattern 'a|a' with input 'ab'")
	}

	// 6. CFG invalid grammar and maxSteps clamping
	reqBadCFG := httptest.NewRequest(http.MethodPost, "/api/theory/cfg", strings.NewReader(`{"variables":[], "terminals":[], "start":"S"}`))
	wBadCFG := httptest.NewRecorder()
	handler.ServeHTTP(wBadCFG, reqBadCFG)
	var respBadCFG map[string]any
	_ = json.NewDecoder(wBadCFG.Body).Decode(&respBadCFG)
	if respBadCFG["valid"] != false {
		t.Fatalf("expected invalid CFG response: %v", respBadCFG)
	}

	reqClampCFG := httptest.NewRequest(http.MethodPost, "/api/theory/cfg", strings.NewReader(`{
		"variables": ["S"], "terminals": ["a"], "start": "S",
		"productions": [{"lhs":"S", "rhs":["a"]}], "max_steps": -5
	}`))
	wClampCFG := httptest.NewRecorder()
	handler.ServeHTTP(wClampCFG, reqClampCFG)
	if wClampCFG.Code != http.StatusOK {
		t.Fatalf("expected 200 for clamped CFG, got %d", wClampCFG.Code)
	}

	// 7. PDA payload validations (testing every failure condition in validatePDAPayload and p.Validate)
	pdaTests := []string{
		`{"states":[]}`,                                                                                                        // empty states
		`{"states":["q0","q0"]}`,                                                                                               // duplicate states
		`{"states":["q0"], "start":"q_missing"}`,                                                                               // undeclared start
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":""}`,                                         // initial stack empty
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"X"}`,                                        // initial stack not in stack_alphabet
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z"}`,                                        // no acceptance mode
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "input_alphabet":[""]}`, // empty input alphabet symbol
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z","Z"], "initial_stack":"Z", "accept_by_empty_stack":true}`,     // duplicate stack alphabet symbol (triggers line 821)
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "input":["x"]}`, // input symbol not in input_alphabet
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "transitions":[{"from_state":"bad"}]}`, // undeclared state in transition
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "transitions":[{"from_state":"q0","to_state":"q0","input":"x","stack_top":"Z"}]}`, // transition input not in input_alphabet
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "transitions":[{"from_state":"q0","to_state":"q0","stack_top":"bad"}]}`,            // transition stack_top not in stack_alphabet
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "transitions":[{"from_state":"q0","to_state":"q0","stack_top":"Z","push":["bad"]}]}`, // transition push not in stack_alphabet
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "accept_by_empty_stack":true, "accept_states":["bad"]}`, // undeclared accept state
		// PDA validation error (p.Validate() error from undeclared target state)
		`{"states":["q0"], "start":"q0", "stack_alphabet":["Z"], "initial_stack":"Z", "input_alphabet":["a"], "accept_by_empty_stack":true, "transitions":[{"from_state":"q0","to_state":"undeclared","input":"a","stack_top":"Z","push":["Z"]}]}`,
	}
	for i, payload := range pdaTests {
		req := httptest.NewRequest(http.MethodPost, "/api/theory/pda", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var r map[string]any
		_ = json.NewDecoder(w.Body).Decode(&r)
		if r["valid"] != false {
			t.Fatalf("expected PDA validation error for case %d: %v", i, r)
		}
	}

	// Valid PDA with max_steps clamped to 50
	reqClampPDA := httptest.NewRequest(http.MethodPost, "/api/theory/pda", strings.NewReader(`{
		"states": ["q0"], "start": "q0", "stack_alphabet": ["Z"], "initial_stack": "Z",
		"input_alphabet": ["a"], "accept_by_empty_stack": true, "max_steps": -1
	}`))
	wClampPDA := httptest.NewRecorder()
	handler.ServeHTTP(wClampPDA, reqClampPDA)
	if wClampPDA.Code != http.StatusOK {
		t.Fatalf("expected 200 for clamped PDA")
	}

	// 8. TM payload validations (testing every failure condition in validateTMPayload, m.Validate, and m.Run)
	tmTests := []string{
		`{"states":[]}`,                                                                 // empty states
		`{"states":["q0"], "start":"q_missing"}`,                                        // undeclared start
		`{"states":["q0"], "start":"q0", "tape_alphabet":[""]}`,                         // empty tape symbol
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"X"}`,           // blank not in tape_alphabet
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "alphabet":[""]}`, // empty alphabet symbol
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "alphabet":["a"]}`, // alphabet symbol not in tape_alphabet
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "accept_states":["bad"]}`, // undeclared accept state
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "reject_states":["bad"]}`, // undeclared reject state
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "accept_states":["q0","q0"]}`, // duplicate halting state in accepts
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "reject_states":["q0","q0"]}`, // duplicate halting state in rejects
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "accept_states":["q0"], "reject_states":["q0"]}`, // both accept and reject (triggers line 1000)
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","move":5}]}`, // move out of range
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"bad","read":"_","write":"_","move":0}]}`, // transition undeclared state
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","to_state":"q0","read":"_","write":"_","move":0},{"from_state":"q0","to_state":"q0","read":"_","write":"_","move":0}]}`, // duplicate transition
		// TM machine validation error (m.Validate() error from undeclared target state)
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","to_state":"undeclared","read":"_","write":"_","move":0}]}`,
		// TM run error (input symbol not in tape_alphabet)
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "input":["not_in_tape"]}`,
	}
	for i, payload := range tmTests {
		req := httptest.NewRequest(http.MethodPost, "/api/theory/tm", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var r map[string]any
		_ = json.NewDecoder(w.Body).Decode(&r)
		if r["valid"] != false {
			t.Fatalf("expected TM validation error for case %d: %v", i, r)
		}
	}

	// 9. LBA payload validations (testing every failure condition in validateLBAPayload)
	lbaTests := []string{
		`{"states":[]}`,
		`{"states":["q0"], "start":"q_missing"}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":[""]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"X"}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "alphabet":[""]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "alphabet":["a"]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "accept_states":["bad"]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","move":5}]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"bad","read":"_","write":"_","move":0}]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","to_state":"q0","read":"_","write":"_","move":0},{"from_state":"q0","to_state":"q0","read":"_","write":"_","move":0}]}`,
		`{"states":["q0"], "start":"q0", "tape_alphabet":["a", "_"], "blank":"_", "alphabet":["a"], "input":["undeclared_symbol"]}`,
		// LBA machine validation error (m.Validate() error from undeclared target state)
		`{"states":["q0"], "start":"q0", "tape_alphabet":["_"], "blank":"_", "transitions":[{"from_state":"q0","to_state":"undeclared","read":"_","write":"_","move":0}]}`,
	}
	for i, payload := range lbaTests {
		req := httptest.NewRequest(http.MethodPost, "/api/theory/lba", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var r map[string]any
		_ = json.NewDecoder(w.Body).Decode(&r)
		if r["valid"] != false {
			t.Fatalf("expected LBA validation error for case %d: %v", i, r)
		}
	}

	// Valid LBA with reject states and clamped max_steps
	reqLBAReject := httptest.NewRequest(http.MethodPost, "/api/theory/lba", strings.NewReader(`{
		"states": ["q0", "qa", "qr"], "alphabet": ["a"], "tape_alphabet": ["a", "_"], "blank": "_", "start": "q0",
		"accept_states": ["qa"], "reject_states": ["qr"],
		"transitions": [{"from_state":"q0","read":"a","to_state":"qr","write":"a","move":0}],
		"input": ["a"], "max_steps": -1
	}`))
	wLBAReject := httptest.NewRecorder()
	handler.ServeHTTP(wLBAReject, reqLBAReject)
	if wLBAReject.Code != http.StatusOK {
		t.Fatalf("expected 200 for LBA with reject states")
	}

	// LBA run error (head move left at index 0)
	reqLBARunErr := httptest.NewRequest(http.MethodPost, "/api/theory/lba", strings.NewReader(`{
		"states": ["q0", "qa"], "alphabet": ["a"], "tape_alphabet": ["a", "_"], "blank": "_", "start": "q0",
		"accept_states": ["qa"], "reject_states": [],
		"transitions": [{"from_state":"q0","read":"a","to_state":"q0","write":"a","move":-1}],
		"input": ["a"], "max_steps": 5
	}`))
	wLBARunErr := httptest.NewRecorder()
	handler.ServeHTTP(wLBARunErr, reqLBARunErr)
	var respLBAErr map[string]any
	_ = json.NewDecoder(wLBARunErr.Body).Decode(&respLBAErr)
	if respLBAErr["valid"] != false {
		t.Fatalf("expected error from LBA tape left-bound exceeded: %v", respLBAErr)
	}

	// 10. CSG payload validations
	csgTests := []string{
		`{"variables":[""], "terminals":["a"], "start":"S"}`,                                                  // empty variable
		`{"variables":["S","S"], "terminals":["a"], "start":"S"}`,                                              // duplicate variable
		`{"variables":["S"], "terminals":[""], "start":"S"}`,                                                   // empty terminal
		`{"variables":["S"], "terminals":["a","a"], "start":"S"}`,                                              // duplicate terminal
		`{"variables":["S"], "terminals":["S"], "start":"S"}`,                                                  // terminal overlaps with variable
		`{"variables":["S"], "terminals":["a"], "start":"X"}`,                                                  // undeclared start
		`{"variables":["S"], "terminals":["a"], "start":"S", "productions":[{"lhs":[], "rhs":["a"]}]}`,        // empty LHS
		// CSG with maxSteps and maxLength clamping
		`{"variables":["S"], "terminals":["a"], "start":"S", "productions":[{"lhs":["S"],"rhs":["a"]}], "input":["a"], "max_steps":-1, "max_length":-1}`,
		`{"variables":["S"], "terminals":["a"], "start":"S", "productions":[{"lhs":["S"],"rhs":["a"]}], "input":[], "max_steps":100, "max_length":0}`,
	}
	for i, payload := range csgTests {
		req := httptest.NewRequest(http.MethodPost, "/api/theory/csg", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var r map[string]any
		_ = json.NewDecoder(w.Body).Decode(&r)
		if i < 7 && r["valid"] != false {
			t.Fatalf("expected CSG validation error for case %d: %v", i, r)
		}
	}

	// CSG with max_length > 100 clamped to 100
	reqClampCSG := httptest.NewRequest(http.MethodPost, "/api/theory/csg", strings.NewReader(`{
		"variables": ["S"], "terminals": ["a"], "start": "S",
		"productions": [{"lhs":["S"],"rhs":["a"]}], "input":["a"], "max_length": 150
	}`))
	wClampCSG := httptest.NewRecorder()
	handler.ServeHTTP(wClampCSG, reqClampCSG)
	if wClampCSG.Code != http.StatusOK {
		t.Fatalf("expected 200 for clamped CSG length")
	}
}

func TestApiDirectHelpers(t *testing.T) {
	// summarize(nil)
	if s := summarize(nil); s != nil {
		t.Fatalf("expected nil for summarize(nil)")
	}

	// exprToJSON edge cases: nil, Unknown
	if exprToJSON(nil) != nil {
		t.Fatalf("expected nil for exprToJSON(nil)")
	}
	unknown := exprToJSON(&ast.Identifier{})
	if unknown == nil || unknown.Type != "Unknown" {
		t.Fatalf("expected Unknown for pointer expr, got %#v", unknown)
	}

	// nfaGraph edge sorting with same from/to but different label
	nfaTie := regex.NFA{
		States: []int{0, 1},
		Transitions: map[int]map[string][]int{
			0: {"b": {1}, "a": {1}},
		},
	}
	nfaGraph(nfaTie)

	// uniqueSymbols & symbolSet
	if _, err := uniqueSymbols("test", []string{}); err == nil {
		t.Fatal("expected error on empty uniqueSymbols")
	}
	if _, err := uniqueSymbols("test", []string{""}); err == nil {
		t.Fatal("expected error on empty string in uniqueSymbols")
	}
	if _, err := uniqueSymbols("test", []string{"a", "a"}); err == nil {
		t.Fatal("expected error on duplicate in uniqueSymbols")
	}
	if _, err := symbolSet("test", []string{""}); err == nil {
		t.Fatal("expected error on empty string in symbolSet")
	}
	if _, err := symbolSet("test", []string{"a", "a"}); err == nil {
		t.Fatal("expected error on duplicate in symbolSet")
	}

	// contains false case
	if contains([]string{"a", "b"}, "c") {
		t.Fatal("expected false for contains non-member")
	}

	// allowedOrigin with empty string
	if allowedOrigin("") {
		t.Fatal("expected false for empty origin")
	}

	// allowedOrigin with default origins when env is unset
	t.Setenv("SENTINEL_ALLOWED_ORIGIN", "")
	if !allowedOrigin("http://localhost:3000") {
		t.Fatal("expected default localhost:3000 allowed")
	}
	if allowedOrigin("http://evil.com") {
		t.Fatal("expected evil.com disallowed")
	}
}

func makeHugeRequestsJSON(n int) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"method":"GET"}`)
	}
	sb.WriteString("]")
	return sb.String()
}
