package semantic

import (
	"testing"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/diagnostics"
	"github.com/ayush/sentinel-dsl/internal/parser"
)


func TestValidateNilAndBasics(t *testing.T) {
	// Nil policy
	dNil := Validate(nil)
	if !dNil.HasErrors() {
		t.Fatal("expected error for nil policy")
	}

	// Empty policy name and missing default
	pEmpty := &ast.Policy{Name: "   "}
	dEmpty := Validate(pEmpty)
	if len(dEmpty) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d: %v", len(dEmpty), dEmpty)
	}

	// Empty rule name, duplicate rule, and nil condition
	pRules := &ast.Policy{
		Name: "test",
		Rules: []ast.Rule{
			{Name: "", When: nil, Action: ast.NewAction(ast.Allow)},
			{Name: "dup", When: nil, Action: ast.NewAction(ast.Allow)},
			{Name: "dup", When: nil, Action: ast.NewAction(ast.Allow)},
		},
		Default: &ast.Action{Kind: ast.Deny},
	}
	dRules := Validate(pRules)
	if len(dRules) < 4 {
		t.Fatalf("expected at least 4 errors for broken rules, got %d: %v", len(dRules), dRules)
	}
}

func TestCheckActionErrors(t *testing.T) {
	var ds diagnostics.Diagnostics
	checkAction(ast.Action{Kind: ast.Allow, Status: 50}, &ds)
	checkAction(ast.Action{Kind: ast.Deny, Status: 600}, &ds)
	checkAction(ast.Action{Kind: ast.ActionKind(99), Status: 200}, &ds)
	if len(ds) != 3 {
		t.Fatalf("expected 3 action errors, got %d: %v", len(ds), ds)
	}
}

func TestCheckExpressions(t *testing.T) {
	// Valid complete policy with boolean literals and all fields
	src := `policy ValidPolicy {
		rule r1 when method == "GET" and path contains "/api" and role in ["admin"] then allow 200;
		rule r2 when not (remote_addr == "127.0.0.1") or scheme != "https" then deny 403;
		rule r3 when host matches "^example\\.com$" and headers["x-token"] == "secret" and query["ref"] == "home" and fields["user"] == "alice" and user_agent == "agent" and (true or false) then allow;
		default deny;
	}`
	p, d := parser.Parse(src)
	if d.HasErrors() {
		t.Fatalf("parse failed: %v", d)
	}
	valDiags := Validate(p)
	if valDiags.HasErrors() {
		t.Fatalf("validation failed: %v", valDiags)
	}

	// Unknown request field identifier
	pUnknownField, _ := parser.Parse(`policy p { rule r when nonexistent_field == "val" then allow; default allow; }`)
	if !Validate(pUnknownField).HasErrors() {
		t.Fatal("expected error for unknown request field")
	}

	// Unary not on non-boolean
	pUnary, _ := parser.Parse(`policy p { rule r when not "string" then allow; default allow; }`)
	if !Validate(pUnary).HasErrors() {
		t.Fatal("expected error for not on non-boolean")
	}

	// Binary operands non-boolean
	pBin, _ := parser.Parse(`policy p { rule r when "a" and 123 then allow; default allow; }`)
	if len(Validate(pBin)) < 2 {
		t.Fatal("expected 2 errors for non-boolean binary operands")
	}

	// In requires a list
	pIn, _ := parser.Parse(`policy p { rule r when method in "not_a_list" then allow; default allow; }`)
	if !Validate(pIn).HasErrors() {
		t.Fatal("expected error for in with non-list")
	}

	// Comparison type mismatch with <
	pComp, _ := parser.Parse(`policy p { rule r when method < 123 then allow; default allow; }`)
	if !Validate(pComp).HasErrors() {
		t.Fatal("expected error for comparing string and number")
	}

	// Contains and Matches left operand not string
	pCont, _ := parser.Parse(`policy p { rule r when 123 contains "foo" or 123 matches "bar" then allow; default allow; }`)
	if len(Validate(pCont)) < 2 {
		t.Fatal("expected errors for contains/matches on non-string left")
	}

	// Matches right operand not string
	pMatchRight, _ := parser.Parse(`policy p { rule r when method matches 123 then allow; default allow; }`)
	if !Validate(pMatchRight).HasErrors() {
		t.Fatal("expected error for matches on non-string pattern")
	}

	// Unsupported expression type (pointer to AST node)
	pUnsupported := &ast.Policy{
		Name: "unsupported",
		Rules: []ast.Rule{
			{Name: "r", When: &ast.Identifier{}, Action: ast.NewAction(ast.Allow)},
		},
		Default: &ast.Action{Kind: ast.Allow},
	}
	if !Validate(pUnsupported).HasErrors() {
		t.Fatal("expected error for unsupported expression")
	}
}

func TestTypeNameAndFieldType(t *testing.T) {
	if typeName(tString) != "string" {
		t.Fatalf("expected string")
	}
	if typeName(tNumber) != "number" {
		t.Fatalf("expected number")
	}
	if typeName(tBool) != "boolean" {
		t.Fatalf("expected boolean")
	}
	if typeName(tList) != "list" {
		t.Fatalf("expected list")
	}
	if typeName(tUnknown) != "type 0" {
		t.Fatalf("expected type 0")
	}
	if fieldType("unknown_field") != tUnknown {
		t.Fatalf("expected unknown for unknown_field")
	}
}
