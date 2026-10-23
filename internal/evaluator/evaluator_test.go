package evaluator

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/parser"
)

func TestEvaluateFirstMatchAndDefaultDeny(t *testing.T) {
	policy, diagnostics := parser.Parse(`policy api {
  rule broad when method == "GET" then deny
  rule narrow when method == "GET" and path == "/health" then allow
  default deny
}`)
	if diagnostics.HasErrors() {
		t.Fatalf("parse diagnostics: %v", diagnostics)
	}
	result := Evaluate(policy, Request{Method: "GET", Path: "/health"})
	if result.Allowed || result.Rule == nil || result.Rule.Name != "broad" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Trace) != 1 || !result.Trace[0].Matched {
		t.Fatalf("trace = %#v", result.Trace)
	}
	result = Evaluate(policy, Request{Method: "POST", Path: "/health"})
	if result.Allowed || !result.Default || len(result.Trace) != 3 {
		t.Fatalf("default result = %#v", result)
	}
}

func TestEvaluateHeadersAndRegex(t *testing.T) {
	policy, diagnostics := parser.Parse(`policy api { rule r when headers["x-role"] == "admin" and path matches "^/v[0-9]+" then allow default deny }`)
	if diagnostics.HasErrors() {
		t.Fatalf("parse diagnostics: %v", diagnostics)
	}
	result := Evaluate(policy, Request{Path: "/v2/users", Headers: http.Header{"X-Role": []string{"admin"}}})
	if !result.Allowed {
		t.Fatalf("result = %#v", result)
	}
}

func TestEvaluateMembership(t *testing.T) {
	policy, diagnostics := parser.Parse(`policy api { rule r when role in ["admin", "support"] then allow; default deny; }`)
	if diagnostics.HasErrors() {
		t.Fatalf("parse diagnostics: %v", diagnostics)
	}
	if !Evaluate(policy, Request{Role: "support"}).Allowed {
		t.Fatal("expected support role to be allowed")
	}
	if Evaluate(policy, Request{Role: "guest"}).Allowed {
		t.Fatal("expected guest role to be denied")
	}
}

func TestRequestFromHTTP(t *testing.T) {
	// Scheme from URL
	req1, _ := http.NewRequest("GET", "https://example.com/api?a=1&b=", nil)
	req1.Header.Set("X-Role", "editor")
	req1.Header.Set("User-Agent", "TestAgent")
	req1.RemoteAddr = "10.0.0.1:1234"
	r1 := RequestFromHTTP(req1)
	if r1.Scheme != "https" || r1.Role != "editor" || r1.Query["a"] != "1" || r1.Query["b"] != "" {
		t.Fatalf("unexpected r1: %+v", r1)
	}

	// Scheme empty with TLS
	req2, _ := http.NewRequest("POST", "/path", nil)
	req2.URL.Scheme = ""
	req2.TLS = &tls.ConnectionState{}
	r2 := RequestFromHTTP(req2)
	if r2.Scheme != "https" {
		t.Fatalf("expected https for TLS, got %q", r2.Scheme)
	}

	// Scheme empty without TLS
	req3, _ := http.NewRequest("POST", "/path", nil)
	req3.URL.Scheme = ""
	req3.TLS = nil
	r3 := RequestFromHTTP(req3)
	if r3.Scheme != "http" {
		t.Fatalf("expected http for non-TLS, got %q", r3.Scheme)
	}
}

func TestEvaluateNilAndDefaultPolicy(t *testing.T) {
	resNil := Evaluate(nil, Request{})
	if resNil.Allowed || len(resNil.Trace) != 1 || resNil.Trace[0].Detail != "no policy; default deny" {
		t.Fatalf("unexpected resNil: %+v", resNil)
	}

	// Policy with allow default
	policyAllow, _ := parser.Parse(`policy p { default allow; }`)
	resAllow := Explain(policyAllow, Request{})
	if !resAllow.Allowed || !resAllow.Default {
		t.Fatalf("expected default allow: %+v", resAllow)
	}

	// Policy without default action struct (Default == nil)
	policyNoDef := &ast.Policy{Name: "no-def"}
	resNoDef := Evaluate(policyNoDef, Request{})
	if resNoDef.Allowed {
		t.Fatalf("expected denied for no-def")
	}
}

func TestEvaluateResolutions(t *testing.T) {
	req := Request{
		Method:     "GET",
		Path:       "/test",
		Host:       "localhost",
		Scheme:     "http",
		RemoteAddr: "127.0.0.1",
		UserAgent:  "go-test",
		Role:       "admin",
		Headers:    http.Header{"X-Custom": []string{"val"}},
		Query:      map[string]string{"q": "search"},
		Fields:     map[string]string{"tenant": "acme"},
	}

	for _, field := range []string{"method", "path", "host", "scheme", "remote_addr", "user_agent", "role"} {
		val := resolve(field, req)
		if val == "" {
			t.Fatalf("expected non-empty resolution for %s", field)
		}
	}
	if resolve("headers['x-custom']", req) != "val" {
		t.Fatalf("failed to resolve header")
	}
	if resolve("query[\"q\"]", req) != "search" {
		t.Fatalf("failed to resolve query")
	}
	if resolve("fields['tenant']", req) != "acme" {
		t.Fatalf("failed to resolve fields")
	}
	if resolve("unknown_var", req) != "" {
		t.Fatalf("expected empty for unknown var")
	}

	// indexed edge cases
	if k, ok := indexed("headers[", "headers"); ok || k != "" {
		t.Fatalf("expected false for headers[")
	}
	if k, ok := indexed("query]foo[", "query"); ok || k != "" {
		t.Fatalf("expected false for malformed")
	}
	if k, ok := indexed("headers[]", "headers"); !ok || k != "" {
		t.Fatalf("expected true and empty key for headers[]")
	}
}

func TestEvaluateShortCircuitsAndLiterals(t *testing.T) {
	// And short-circuit (left is false)
	pAnd, _ := parser.Parse(`policy p { rule r when false and true then allow; default deny; }`)
	rAnd := Evaluate(pAnd, Request{})
	if rAnd.Allowed || len(rAnd.Trace) < 1 || rAnd.Trace[0].Matched {
		t.Fatalf("unexpected rAnd: %+v", rAnd)
	}

	// Or short-circuit (left is true)
	pOr, _ := parser.Parse(`policy p { rule r when true or false then allow; default deny; }`)
	rOr := Evaluate(pOr, Request{})
	if !rOr.Allowed || len(rOr.Trace) < 1 || !rOr.Trace[0].Matched {
		t.Fatalf("unexpected rOr: %+v", rOr)
	}

	// Or right evaluated (left is false, right is true)
	pOrRight, _ := parser.Parse(`policy p { rule r when false or true then allow; default deny; }`)
	rOrRight := Evaluate(pOrRight, Request{})
	if !rOrRight.Allowed {
		t.Fatalf("expected allowed for or right")
	}

	// Unary not
	pNot, _ := parser.Parse(`policy p { rule r when not false then allow; default deny; }`)
	if !Evaluate(pNot, Request{}).Allowed {
		t.Fatalf("expected allowed for not false")
	}

	// List literal and truthy check
	pList, _ := parser.Parse(`policy p { rule r when [1, "two", true] in [] then deny; default allow; }`)
	if !Evaluate(pList, Request{}).Allowed {
		t.Fatalf("expected allowed for list test")
	}
}

func TestEvaluateComparisons(t *testing.T) {
	// Compare numbers
	numOps := []struct {
		op       ast.CompareOp
		l, r     float64
		expected bool
	}{
		{ast.Equal, 5, 5, true},
		{ast.Equal, 5, 6, false},
		{ast.NotEqual, 5, 6, true},
		{ast.NotEqual, 5, 5, false},
		{ast.Less, 4, 5, true},
		{ast.Less, 5, 4, false},
		{ast.LessEqual, 5, 5, true},
		{ast.LessEqual, 6, 5, false},
		{ast.Greater, 6, 5, true},
		{ast.Greater, 5, 6, false},
		{ast.GreaterEqual, 5, 5, true},
		{ast.GreaterEqual, 4, 5, false},
		{ast.In, 5, 5, false}, // default case
	}
	for _, tc := range numOps {
		got := compareNumbers(tc.l, tc.op, tc.r)
		if got != tc.expected {
			t.Fatalf("compareNumbers(%v, %v, %v) = %v, expected %v", tc.l, tc.op, tc.r, got, tc.expected)
		}
	}

	// String numbers comparison through compare()
	if !compare("10", ast.Greater, "9") { // as numbers 10 > 9
		t.Fatal("expected 10 > 9 as numbers")
	}
	if !compare("10", ast.Equal, 10.0) {
		t.Fatal("expected string '10' == 10.0")
	}

	// Compare strings
	strOps := []struct {
		op       ast.CompareOp
		l, r     string
		expected bool
	}{
		{ast.Equal, "a", "a", true},
		{ast.Equal, "a", "b", false},
		{ast.NotEqual, "a", "b", true},
		{ast.NotEqual, "a", "a", false},
		{ast.Less, "a", "b", true},
		{ast.Less, "b", "a", false},
		{ast.LessEqual, "a", "a", true},
		{ast.LessEqual, "b", "a", false},
		{ast.Greater, "b", "a", true},
		{ast.Greater, "a", "b", false},
		{ast.GreaterEqual, "b", "a", true},
		{ast.GreaterEqual, "a", "b", false},
		{ast.In, "a", "b", false}, // default case
	}
	for _, tc := range strOps {
		got := compareStrings(tc.l, tc.op, tc.r)
		if got != tc.expected {
			t.Fatalf("compareStrings(%q, %v, %q) = %v, expected %v", tc.l, tc.op, tc.r, got, tc.expected)
		}
	}

	// Booleans compare
	if !compare(true, ast.Equal, true) {
		t.Fatal("expected true == true")
	}
	if compare(true, ast.Equal, false) {
		t.Fatal("expected true != false for Equal")
	}
	if !compare(true, ast.NotEqual, false) {
		t.Fatal("expected true != false")
	}
	if compare(true, ast.NotEqual, true) {
		t.Fatal("expected not (true != true)")
	}
	// Boolean with other op
	if compare(true, ast.Less, true) {
		t.Fatal("expected false for bool Less")
	}

	// In operator with non-slice right
	if compare("x", ast.In, "not_a_slice") {
		t.Fatal("expected false for in non-slice")
	}

	// Contains / Matches with non-strings
	if compare(123, ast.Contains, "foo") {
		t.Fatal("expected false for Contains on non-string")
	}
	if compare("foo", ast.Contains, 123) {
		t.Fatal("expected false for Contains with non-string")
	}
	if compare("hello", ast.Contains, "ell") != true {
		t.Fatal("expected contains true")
	}
	if compare("hello", ast.Contains, "xyz") != false {
		t.Fatal("expected contains false")
	}

	// Regex match invalid regex
	if compare("test", ast.Matches, "[invalid-regex") {
		t.Fatal("expected false for invalid regex")
	}
}

func TestEvaluatorEdgeCases(t *testing.T) {
	// evaluateExpr nil condition
	matched, detail := evaluateExpr(nil, Request{})
	if matched || detail != "missing condition" {
		t.Fatalf("unexpected for nil expr: %v, %q", matched, detail)
	}

	// evaluateExpr non-boolean expression (e.g. StringLiteral)
	matched2, detail2 := evaluateExpr(ast.NewStringLiteralAt("true_str", ast.Span{}), Request{})
	if !matched2 || detail2 != "\"true_str\"" {
		t.Fatalf("unexpected for string expr: %v, %q", matched2, detail2)
	}

	// valueOf default unsupported expression
	val, det := valueOf(&ast.Identifier{}, Request{})
	if val != false || det != "unsupported expression" {
		t.Fatalf("unexpected for unsupported expr: %v, %q", val, det)
	}

	// number() with non-number type
	if _, ok := number(true); ok {
		t.Fatal("expected number(true) to be false")
	}

	// truthy() all branches
	if !truthy(true) || truthy(false) {
		t.Fatal("failed truthy bool")
	}
	if !truthy("yes") || truthy("") {
		t.Fatal("failed truthy string")
	}
	if !truthy(1.5) || truthy(0.0) {
		t.Fatal("failed truthy float")
	}
	if !truthy([]int{1}) || truthy(nil) {
		t.Fatal("failed truthy default")
	}
}
