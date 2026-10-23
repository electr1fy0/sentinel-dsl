package parser

import (
	"strings"
	"testing"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/lexer"
)

func TestParseBooleanPrecedence(t *testing.T) {
	policy, diagnostics := Parse(`policy api {
  rule "read" when method == "GET" or method == "HEAD" and path == "/v1" then allow
  default deny 403 "blocked"
}`)
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if policy.Name != "api" || len(policy.Rules) != 1 || policy.Default == nil {
		t.Fatalf("policy = %#v", policy)
	}
	or, ok := policy.Rules[0].When.(ast.BinaryExpr)
	if !ok || or.Op != ast.Or {
		t.Fatalf("root = %#v", policy.Rules[0].When)
	}
	if _, ok := or.Right.(ast.BinaryExpr); !ok {
		t.Fatalf("and should bind tighter: %#v", or.Right)
	}
	if policy.Default.Status != 403 || policy.Default.Message != "blocked" {
		t.Fatalf("default = %#v", policy.Default)
	}
}

func TestParseReportsMissingDefaultAndBrace(t *testing.T) {
	_, diagnostics := Parse(`policy broken { rule r when method == "GET" then allow`)
	if !diagnostics.HasErrors() {
		t.Fatal("expected syntax diagnostic")
	}
}

func TestParseSampleMembershipAndSemicolons(t *testing.T) {
	policy, diagnostics := Parse(`policy UserAccess {
  rule ReadUsers when method == "GET" and path matches "/api/users/[0-9]+" and role in ["admin", "support"] then allow;
  rule DeleteUsers when method == "DELETE" then deny "Delete operation not permitted";
  default deny "No matching rule";
}`)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	if len(policy.Rules) != 2 || policy.Default == nil {
		t.Fatalf("policy = %#v", policy)
	}
}

func TestParseAllOperatorsAndExpressions(t *testing.T) {
	src := `policy Full {
		; ;
		rule r1 when not (a != b and c < 10 or d <= 20 and e > 30 or f >= 40 and g contains "x") then allow 200;
		rule r2 when req.field.name == "ok" and headers["auth"] == "bearer" and headers[custom] == "val" then allow;
		rule r3 when [1, 2] == [] then deny;
		default allow;
	}`
	policy, diags := Parse(src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(policy.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(policy.Rules))
	}
}

func TestParseErrorsAndEdgeCases(t *testing.T) {
	// Multiple default actions
	_, d1 := Parse(`policy p { default allow; default deny; }`)
	if !d1.HasErrors() {
		t.Fatal("expected error for multiple defaults")
	}

	// Unexpected token in policy body triggering synchronizeTopLevel
	_, d2 := Parse(`policy p { invalid_stmt; rule r when true then allow; }`)
	if !d2.HasErrors() {
		t.Fatal("expected error for invalid_stmt")
	}

	// Extra token after policy closing brace
	_, d3 := Parse(`policy p { default allow; } extra`)
	if !d3.HasErrors() {
		t.Fatal("expected error for token after policy")
	}

	// Double closing brace
	p4, d4 := Parse(`policy p { default allow; }}`)
	if d4.HasErrors() {
		t.Fatalf("unexpected error for double brace: %v", d4)
	}
	if p4.Name != "p" {
		t.Fatalf("expected policy p")
	}

	// Missing policy name
	_, d5 := Parse(`policy { default allow; }`)
	if !d5.HasErrors() {
		t.Fatal("expected error for missing policy name")
	}

	// Missing rule name
	_, d6 := Parse(`policy p { rule when true then allow; }`)
	if !d6.HasErrors() {
		t.Fatal("expected error for missing rule name")
	}

	// Invalid action kind
	_, d7 := Parse(`policy p { default invalidAction; }`)
	if !d7.HasErrors() {
		t.Fatal("expected error for invalid action")
	}

	// Action status float instead of integer
	_, d8 := Parse(`policy p { default deny 403.5 "error"; }`)
	if !d8.HasErrors() {
		t.Fatal("expected error for non-integer action status")
	}

	// Invalid bracket index (number instead of string/ident)
	_, d9 := Parse(`policy p { rule r when headers[123] == "x" then allow; default allow; }`)
	if !d9.HasErrors() {
		t.Fatal("expected error for non-string bracket index")
	}

	// Unexpected primary expression token
	_, d10 := Parse(`policy p { rule r when + == 1 then allow; default allow; }`)
	if !d10.HasErrors() {
		t.Fatal("expected error for unexpected primary token")
	}
}

func TestParserDirectHelpers(t *testing.T) {
	// String() method
	p := &parser{}
	if !strings.Contains(p.String(), "parser at token 0") {
		t.Fatalf("unexpected String(): %q", p.String())
	}

	// current() beyond tokens length
	toks, _ := lexer.Lex("policy p {}")
	p2 := &parser{tokens: toks, index: 100}
	cur := p2.current()
	if cur.Kind != lexer.EOF {
		t.Fatalf("expected EOF at index > len(tokens), got %v", cur)
	}

	// parseAction with invalid number token
	p3 := &parser{tokens: []lexer.Token{
		{Kind: lexer.Deny},
		{Kind: lexer.Number, Value: "invalid_num"},
	}}
	act := p3.parseAction()
	if act.Kind != ast.Deny || !p3.diagnostics.HasErrors() {
		t.Fatalf("expected error from invalid number token in action")
	}

	// parsePrimary with invalid number token
	p4 := &parser{tokens: []lexer.Token{
		{Kind: lexer.Number, Value: "bad_number"},
	}}
	expr := p4.parsePrimary()
	if _, ok := expr.(ast.NumberLiteral); !ok || !p4.diagnostics.HasErrors() {
		t.Fatalf("expected error from bad number literal")
	}
}
