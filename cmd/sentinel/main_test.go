package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUsage(t *testing.T) {
	usage()
}

func TestWriteJSON(t *testing.T) {
	if err := writeJSON(map[string]string{"foo": "bar"}); err != nil {
		t.Fatalf("expected writeJSON to succeed, got %v", err)
	}

	// Unserializable type (channel) should cause encode to fail
	if err := writeJSON(make(chan int)); err == nil {
		t.Fatalf("expected writeJSON with channel to fail, got nil")
	}
}

func TestLoadPolicy(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.sntl")
	if err := os.WriteFile(validPath, []byte(`policy TestPolicy { rule R1 when method == "GET" then allow; default deny; }`), 0644); err != nil {
		t.Fatal(err)
	}

	syntaxErrPath := filepath.Join(dir, "syntax_err.sntl")
	if err := os.WriteFile(syntaxErrPath, []byte(`policy Broken { rule ???`), 0644); err != nil {
		t.Fatal(err)
	}

	semanticErrPath := filepath.Join(dir, "semantic_err.sntl")
	if err := os.WriteFile(semanticErrPath, []byte(`policy Dup { rule R when method == "GET" then allow; rule R when method == "POST" then allow; }`), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Missing file
	if _, err := loadPolicy(filepath.Join(dir, "nonexistent.sntl")); err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	// 2. Syntax error
	if _, err := loadPolicy(syntaxErrPath); err == nil {
		t.Fatal("expected syntax error")
	}

	// 3. Semantic error
	if _, err := loadPolicy(semanticErrPath); err == nil {
		t.Fatal("expected semantic error")
	}

	// 4. Success
	p, err := loadPolicy(validPath)
	if err != nil || p == nil {
		t.Fatalf("unexpected loadPolicy error: %v", err)
	}
}

func TestCommandCheck(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.sntl")
	if err := os.WriteFile(validPath, []byte(`policy TestPolicy { rule R1 when method == "GET" then allow; default deny; }`), 0644); err != nil {
		t.Fatal(err)
	}

	syntaxErrPath := filepath.Join(dir, "syntax_err.sntl")
	if err := os.WriteFile(syntaxErrPath, []byte(`policy Broken { rule ???`), 0644); err != nil {
		t.Fatal(err)
	}

	// Wrong arg count
	if err := commandCheck(nil); err == nil {
		t.Fatal("expected error with no args")
	}
	if err := commandCheck([]string{"a", "b"}); err == nil {
		t.Fatal("expected error with 2 args")
	}

	// Nonexistent file
	if err := commandCheck([]string{filepath.Join(dir, "nope.sntl")}); err == nil {
		t.Fatal("expected read file error")
	}

	// Diagnostics returned (syntax error)
	if err := commandCheck([]string{syntaxErrPath}); err == nil {
		t.Fatal("expected diagnostics error")
	}

	// Success
	if err := commandCheck([]string{validPath}); err != nil {
		t.Fatalf("unexpected commandCheck error: %v", err)
	}
}

func TestCommandCompile(t *testing.T) {
	dir := t.TempDir()

	validWithDefault := filepath.Join(dir, "with_default.sntl")
	if err := os.WriteFile(validWithDefault, []byte(`policy WithDef { rule R1 when method == "GET" then allow; default deny; }`), 0644); err != nil {
		t.Fatal(err)
	}

	// Wrong arg count
	if err := commandCompile(nil); err == nil {
		t.Fatal("expected error with no args")
	}

	// Nonexistent file
	if err := commandCompile([]string{filepath.Join(dir, "missing.sntl")}); err == nil {
		t.Fatal("expected error with nonexistent file")
	}

	// Valid with default
	if err := commandCompile([]string{validWithDefault}); err != nil {
		t.Fatalf("unexpected error with default: %v", err)
	}
}

func TestCommandEvaluate(t *testing.T) {
	dir := t.TempDir()

	policyPath := filepath.Join(dir, "policy.sntl")
	if err := os.WriteFile(policyPath, []byte(`
policy AuthPolicy {
  rule AllowAdmin when method == "GET" and role == "admin" then allow;
  rule DenyDelete when method == "DELETE" then deny "no deletes";
  default deny "default fallback";
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	allowedReqPath := filepath.Join(dir, "allowed.json")
	if err := os.WriteFile(allowedReqPath, []byte(`{
  "method": "GET",
  "path": "/admin",
  "role": "admin",
  "headers": {"X-Trace": ["123"]},
  "query": {"q": "test"},
  "fields": {"f": "v"}
}`), 0644); err != nil {
		t.Fatal(err)
	}

	deniedReqPath := filepath.Join(dir, "denied.json")
	if err := os.WriteFile(deniedReqPath, []byte(`{"method": "DELETE", "path": "/admin"}`), 0644); err != nil {
		t.Fatal(err)
	}

	defaultReqPath := filepath.Join(dir, "default.json")
	if err := os.WriteFile(defaultReqPath, []byte(`{"method": "POST", "path": "/unknown"}`), 0644); err != nil {
		t.Fatal(err)
	}

	badJSONPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badJSONPath, []byte(`invalid json`), 0644); err != nil {
		t.Fatal(err)
	}

	// Wrong arg count
	if err := commandEvaluate(false, []string{"one"}); err == nil {
		t.Fatal("expected error for wrong arg count")
	}

	// Load policy failure
	if err := commandEvaluate(false, []string{filepath.Join(dir, "missing.sntl"), allowedReqPath}); err == nil {
		t.Fatal("expected error for missing policy")
	}

	// Missing request file
	if err := commandEvaluate(false, []string{policyPath, filepath.Join(dir, "missing.json")}); err == nil {
		t.Fatal("expected error for missing request file")
	}

	// Bad json request
	if err := commandEvaluate(false, []string{policyPath, badJSONPath}); err == nil {
		t.Fatal("expected error for invalid json")
	}

	// Allowed with explain=false
	if err := commandEvaluate(false, []string{policyPath, allowedReqPath}); err != nil {
		t.Fatalf("unexpected error for allowed eval: %v", err)
	}

	// Denied with explain=true (Rule != nil, Allowed == false)
	if err := commandEvaluate(true, []string{policyPath, deniedReqPath}); err != nil {
		t.Fatalf("unexpected error for denied explain: %v", err)
	}

	// Default fallback (Rule == nil)
	if err := commandEvaluate(false, []string{policyPath, defaultReqPath}); err != nil {
		t.Fatalf("unexpected error for default fallback eval: %v", err)
	}
}

func TestCommandAutomata(t *testing.T) {
	// Wrong arg count < 2
	if err := commandAutomata(nil); err == nil {
		t.Fatal("expected error with no args")
	}
	if err := commandAutomata([]string{"build"}); err == nil {
		t.Fatal("expected error with 1 arg")
	}

	// Regex parse error
	if err := commandAutomata([]string{"build", "("}); err == nil {
		t.Fatal("expected regex parse error")
	}

	// Build
	if err := commandAutomata([]string{"build", "a*b"}); err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	// Minimize
	if err := commandAutomata([]string{"minimize", "a*b"}); err != nil {
		t.Fatalf("unexpected minimize error: %v", err)
	}

	// Test with wrong args count != 3
	if err := commandAutomata([]string{"test", "a*b"}); err == nil {
		t.Fatal("expected test usage error")
	}
	if err := commandAutomata([]string{"test", "a*b", "ab", "extra"}); err == nil {
		t.Fatal("expected test usage error with 4 args")
	}

	// Test matches
	if err := commandAutomata([]string{"test", "a*b", "aab"}); err != nil {
		t.Fatalf("unexpected test error: %v", err)
	}

	// Test doesn't match
	if err := commandAutomata([]string{"test", "a*b", "c"}); err != nil {
		t.Fatalf("unexpected test error: %v", err)
	}

	// Unknown subcommand
	if err := commandAutomata([]string{"unknown", "a*b"}); err == nil {
		t.Fatal("expected error for unknown automata command")
	}
}

func TestAppendTransition(t *testing.T) {
	// row == nil
	row := appendTransition(nil, "a", "1")
	if len(row["a"]) != 1 || row["a"][0] != "1" {
		t.Fatalf("unexpected result: %v", row)
	}

	// row != nil
	row = appendTransition(row, "a", "2")
	if len(row["a"]) != 2 || row["a"][1] != "2" {
		t.Fatalf("unexpected result: %v", row)
	}
}

func TestCommandServe(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.sntl")
	if err := os.WriteFile(policyPath, []byte(`policy TestServe { rule R when method == "GET" then allow; default deny; }`), 0644); err != nil {
		t.Fatal(err)
	}

	// Wrong arg count
	if err := commandServe(nil); err == nil {
		t.Fatal("expected error with 0 args")
	}
	if err := commandServe([]string{"a", "b", "c"}); err == nil {
		t.Fatal("expected error with 3 args")
	}

	// Nonexistent policy
	if err := commandServe([]string{filepath.Join(dir, "nope.sntl")}); err == nil {
		t.Fatal("expected error for missing policy")
	}

	// Save and restore original httpListenAndServe
	origListen := httpListenAndServe
	defer func() { httpListenAndServe = origListen }()

	// Mock httpListenAndServe to verify handler registration and dispatch
	var capturedAddr string
	var serverCalled bool
	httpListenAndServe = func(addr string, handler http.Handler) error {
		capturedAddr = addr
		serverCalled = true

		// Test the protected handler branch
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK from protected handler, got %d", rec.Code)
		}
		if rec.Body.String() != "allowed\n" {
			t.Errorf("expected 'allowed\n', got %q", rec.Body.String())
		}

		// Test API handler
		reqAPI := httptest.NewRequest("GET", "/api/tokens", nil)
		recAPI := httptest.NewRecorder()
		handler.ServeHTTP(recAPI, reqAPI)

		return nil
	}

	// Case 1: default addr :8080 (1 arg)
	if err := commandServe([]string{policyPath}); err != nil {
		t.Fatalf("unexpected commandServe error: %v", err)
	}
	if !serverCalled || capturedAddr != ":8080" {
		t.Fatalf("expected addr :8080, got %s", capturedAddr)
	}

	// Case 2: custom addr (2 args)
	serverCalled = false
	if err := commandServe([]string{policyPath, ":9090"}); err != nil {
		t.Fatalf("unexpected commandServe error: %v", err)
	}
	if !serverCalled || capturedAddr != ":9090" {
		t.Fatalf("expected addr :9090, got %s", capturedAddr)
	}

	// Case 3: mock returns error
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return errors.New("listen error")
	}
	if err := commandServe([]string{policyPath}); err == nil {
		t.Fatal("expected listen error")
	}

	// Call original listen with invalid address to exercise httpListenAndServe variable declaration
	if err := origListen("invalid:addr:123", nil); err == nil {
		t.Fatal("expected error from invalid listen addr")
	}
}

func TestMainFunction(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.sntl")
	if err := os.WriteFile(policyPath, []byte(`policy TestMain { rule R when method == "GET" then allow; default deny; }`), 0644); err != nil {
		t.Fatal(err)
	}
	reqPath := filepath.Join(dir, "req.json")
	if err := os.WriteFile(reqPath, []byte(`{"method": "GET", "path": "/"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origArgs := os.Args
	origExit := osExit
	origListen := httpListenAndServe
	defer func() {
		os.Args = origArgs
		osExit = origExit
		httpListenAndServe = origListen
	}()

	var lastExitCode int
	osExit = func(code int) {
		lastExitCode = code
	}
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return nil
	}

	runMainWith := func(args ...string) int {
		lastExitCode = 0
		os.Args = append([]string{"sentinel"}, args...)
		main()
		return lastExitCode
	}

	// 1. len(os.Args) < 2 -> exit 2
	if code := runMainWith(); code != 2 {
		t.Fatalf("expected exit code 2 with no args, got %d", code)
	}

	// 2. check
	if code := runMainWith("check", policyPath); code != 0 {
		t.Fatalf("expected exit code 0 for check, got %d", code)
	}

	// 3. compile
	if code := runMainWith("compile", policyPath); code != 0 {
		t.Fatalf("expected exit code 0 for compile, got %d", code)
	}

	// 4. eval
	if code := runMainWith("eval", policyPath, reqPath); code != 0 {
		t.Fatalf("expected exit code 0 for eval, got %d", code)
	}

	// 5. explain
	if code := runMainWith("explain", policyPath, reqPath); code != 0 {
		t.Fatalf("expected exit code 0 for explain, got %d", code)
	}

	// 6. automata
	if code := runMainWith("automata", "build", "a*b"); code != 0 {
		t.Fatalf("expected exit code 0 for automata, got %d", code)
	}

	// 7. serve
	if code := runMainWith("serve", policyPath); code != 0 {
		t.Fatalf("expected exit code 0 for serve, got %d", code)
	}

	// 8. help, -h, --help
	if code := runMainWith("help"); code != 0 {
		t.Fatalf("expected exit code 0 for help, got %d", code)
	}
	if code := runMainWith("-h"); code != 0 {
		t.Fatalf("expected exit code 0 for -h, got %d", code)
	}
	if code := runMainWith("--help"); code != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", code)
	}

	// 9. unknown command -> exit 1
	if code := runMainWith("unknown"); code != 1 {
		t.Fatalf("expected exit code 1 for unknown command, got %d", code)
	}

	// 10. command failure (e.g. check on nonexistent file) -> exit 1
	if code := runMainWith("check", filepath.Join(dir, "missing.sntl")); code != 1 {
		t.Fatalf("expected exit code 1 for failing command, got %d", code)
	}
}
