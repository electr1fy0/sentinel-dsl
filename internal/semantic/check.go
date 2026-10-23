package semantic

import (
	"fmt"
	"strings"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/diagnostics"
)

// Validate performs policy-wide and expression-level semantic checks.
func Validate(policy *ast.Policy) diagnostics.Diagnostics {
	var ds diagnostics.Diagnostics
	if policy == nil {
		ds.Errorf(diagnostics.Span{}, "policy is nil")
		return ds
	}
	if strings.TrimSpace(policy.Name) == "" {
		ds.Errorf(policy.Span(), "policy name cannot be empty")
	}
	if policy.Default == nil {
		ds.Errorf(policy.Span(), "policy must define a default action")
	}
	seen := map[string]bool{}
	for _, rule := range policy.Rules {
		if rule.Name == "" {
			ds.Errorf(rule.Span(), "rule name cannot be empty")
		}
		if seen[rule.Name] {
			ds.Errorf(rule.Span(), "duplicate rule %q", rule.Name)
		}
		seen[rule.Name] = true
		if rule.When == nil {
			ds.Errorf(rule.Span(), "rule %q has no condition", rule.Name)
		} else {
			checkExpr(rule.When, &ds)
		}
		checkAction(rule.Action, &ds)
	}
	if policy.Default != nil {
		checkAction(*policy.Default, &ds)
	}
	return ds
}

func checkAction(action ast.Action, ds *diagnostics.Diagnostics) {
	if action.Status != 0 && (action.Status < 100 || action.Status > 599) {
		ds.Errorf(action.Span(), "action status must be between 100 and 599")
	}
	if action.Kind != ast.Allow && action.Kind != ast.Deny {
		ds.Errorf(action.Span(), "unknown action")
	}
}

type valueType uint8

const (
	tUnknown valueType = iota
	tString
	tNumber
	tBool
	tList
)

func checkExpr(expr ast.Expr, ds *diagnostics.Diagnostics) valueType {
	switch e := expr.(type) {
	case ast.Identifier:
		if !knownField(e.Name) {
			ds.Errorf(e.Span(), "unknown request field %q", e.Name)
		}
		return fieldType(e.Name)
	case ast.StringLiteral:
		return tString
	case ast.NumberLiteral:
		return tNumber
	case ast.BoolLiteral:
		return tBool
	case ast.ListExpr:
		for _, item := range e.Values {
			checkExpr(item, ds)
		}
		return tList
	case ast.UnaryExpr:
		if checkExpr(e.Expr, ds) != tBool {
			ds.Errorf(e.Span(), "not requires a boolean expression")
		}
		return tBool
	case ast.BinaryExpr:
		if checkExpr(e.Left, ds) != tBool {
			ds.Errorf(e.Left.Span(), "%s requires boolean operands", e.Op)
		}
		if checkExpr(e.Right, ds) != tBool {
			ds.Errorf(e.Right.Span(), "%s requires boolean operands", e.Op)
		}
		return tBool
	case ast.CompareExpr:
		left, right := checkExpr(e.Left, ds), checkExpr(e.Right, ds)
		if e.Op == ast.In {
			if right != tList {
				ds.Errorf(e.Right.Span(), "in requires a list")
			}
		} else if left != tUnknown && right != tUnknown && left != right && !(e.Op == ast.Equal || e.Op == ast.NotEqual) {
			ds.Errorf(e.Span(), "cannot compare %s with %s", typeName(left), typeName(right))
		}
		if (e.Op == ast.Contains || e.Op == ast.Matches) && left != tString {
			ds.Errorf(e.Left.Span(), "%s requires a string field", e.Op)
		}
		if e.Op == ast.Matches && right != tString {
			ds.Errorf(e.Right.Span(), "matches requires a string pattern")
		}
		return tBool
	default:
		ds.Errorf(expr.Span(), "unsupported expression %T", expr)
		return tUnknown
	}
}

func knownField(name string) bool {
	if strings.HasPrefix(name, "headers[") || strings.HasPrefix(name, "query[") || strings.HasPrefix(name, "fields[") {
		return true
	}
	switch name {
	case "method", "path", "host", "scheme", "remote_addr", "user_agent", "role":
		return true
	}
	return false
}

func fieldType(name string) valueType {
	if name == "method" || name == "path" || name == "host" || name == "scheme" || name == "remote_addr" || name == "user_agent" || name == "role" || strings.HasPrefix(name, "headers[") || strings.HasPrefix(name, "query[") || strings.HasPrefix(name, "fields[") {
		return tString
	}
	return tUnknown
}

func typeName(t valueType) string {
	switch t {
	case tString:
		return "string"
	case tNumber:
		return "number"
	case tBool:
		return "boolean"
	case tList:
		return "list"
	default:
		return fmt.Sprintf("type %d", t)
	}
}
