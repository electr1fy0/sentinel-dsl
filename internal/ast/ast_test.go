package ast

import (
	"testing"

	"github.com/ayush/sentinel-dsl/internal/diagnostics"
)

func TestASTNodesAndSpans(t *testing.T) {
	span := Span{
		Start: diagnostics.Position{Offset: 0, Line: 1, Column: 1},
		End:   diagnostics.Position{Offset: 10, Line: 1, Column: 11},
	}

	// ActionKind String
	if Allow.String() != "allow" {
		t.Fatalf("expected 'allow', got %q", Allow.String())
	}
	if Deny.String() != "deny" {
		t.Fatalf("expected 'deny', got %q", Deny.String())
	}

	// Action
	act1 := NewAction(Allow)
	if act1.Kind != Allow {
		t.Fatalf("expected Allow kind")
	}
	act2 := NewActionAt(Deny, 403, "forbidden", span)
	if act2.Span() != span || act2.Status != 403 || act2.Message != "forbidden" {
		t.Fatalf("unexpected action: %+v", act2)
	}

	// Identifiers
	id1 := NewIdentifier("user")
	if id1.Name != "user" {
		t.Fatalf("expected 'user'")
	}
	id2 := NewIdentifierAt("role", span)
	if id2.Span() != span {
		t.Fatalf("expected span match")
	}
	id2.exprNode()

	// Literals
	strLit := NewStringLiteralAt("admin", span)
	if strLit.Span() != span || strLit.Value != "admin" {
		t.Fatalf("unexpected string literal")
	}
	strLit.exprNode()

	numLit := NewNumberLiteralAt(42.5, span)
	if numLit.Span() != span || numLit.Value != 42.5 {
		t.Fatalf("unexpected number literal")
	}
	numLit.exprNode()

	boolLit := NewBoolLiteralAt(true, span)
	if boolLit.Span() != span || !boolLit.Value {
		t.Fatalf("unexpected bool literal")
	}
	boolLit.exprNode()

	// Unary
	uOp := Not
	if uOp.String() != "not" {
		t.Fatalf("expected 'not', got %q", uOp.String())
	}
	uExpr := NewUnaryAt(uOp, boolLit, span)
	if uExpr.Span() != span || uExpr.Op != Not {
		t.Fatalf("unexpected unary expr")
	}
	uExpr.exprNode()

	// Binary
	if And.String() != "and" {
		t.Fatalf("expected 'and', got %q", And.String())
	}
	if Or.String() != "or" {
		t.Fatalf("expected 'or', got %q", Or.String())
	}
	bExpr := NewBinaryAt(boolLit, And, boolLit, span)
	if bExpr.Span() != span || bExpr.Op != And {
		t.Fatalf("unexpected binary expr")
	}
	bExpr.exprNode()

	// Compare ops
	ops := []struct {
		op   CompareOp
		name string
	}{
		{Equal, "=="},
		{NotEqual, "!="},
		{Less, "<"},
		{LessEqual, "<="},
		{Greater, ">"},
		{GreaterEqual, ">="},
		{Contains, "contains"},
		{Matches, "matches"},
		{In, "in"},
	}
	for _, tc := range ops {
		if tc.op.String() != tc.name {
			t.Fatalf("expected %q, got %q", tc.name, tc.op.String())
		}
	}
	cExpr := NewCompareAt(id1, Equal, strLit, span)
	if cExpr.Span() != span || cExpr.Op != Equal {
		t.Fatalf("unexpected compare expr")
	}
	cExpr.exprNode()

	// List
	listExpr := NewListAt([]Expr{strLit, numLit}, span)
	if listExpr.Span() != span || len(listExpr.Values) != 2 {
		t.Fatalf("unexpected list expr")
	}
	listExpr.exprNode()

	// Rule
	rule1 := NewRule("r1", cExpr, act1)
	if rule1.Name != "r1" {
		t.Fatalf("unexpected rule name")
	}
	rule2 := NewRuleAt("r2", cExpr, act2, span)
	if rule2.Span() != span {
		t.Fatalf("unexpected rule span")
	}

	// Policy
	p1 := NewPolicy("p1", []Rule{rule1}, act1)
	if p1.Name != "p1" || p1.Default == nil || p1.Default.Kind != Allow {
		t.Fatalf("unexpected policy")
	}
	p2 := NewPolicyAt("p2", []Rule{rule2}, &act2, span)
	if p2.Span() != span || p2.Default == nil || p2.Default.Kind != Deny {
		t.Fatalf("unexpected policy")
	}
}
