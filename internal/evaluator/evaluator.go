package evaluator

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ayush/sentinel-dsl/internal/ast"
)

// Request is the request data exposed to SentinelDSL conditions.
type Request struct {
	Method     string
	Path       string
	Host       string
	Scheme     string
	RemoteAddr string
	UserAgent  string
	Role       string
	Headers    http.Header
	Query      map[string]string
	Fields     map[string]string
}

func RequestFromHTTP(r *http.Request) Request {
	query := make(map[string]string, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}
	scheme := r.URL.Scheme
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return Request{Method: r.Method, Path: r.URL.Path, Host: r.Host, Scheme: scheme, RemoteAddr: r.RemoteAddr, UserAgent: r.UserAgent(), Role: r.Header.Get("X-Role"), Headers: r.Header, Query: query, Fields: map[string]string{}}
}

type TraceEntry struct {
	Rule    string
	Matched bool
	Detail  string
}

// Result is a policy decision and its explain trace.
type Result struct {
	Allowed bool
	Action  ast.Action
	Rule    *ast.Rule
	Default bool
	Trace   []TraceEntry
}

// Evaluate applies rules in source order and stops at the first match.
func Evaluate(policy *ast.Policy, request Request) Result {
	result := Result{Action: ast.NewAction(ast.Deny)}
	if policy == nil {
		result.Trace = []TraceEntry{{Detail: "no policy; default deny"}}
		return result
	}
	for i := range policy.Rules {
		rule := &policy.Rules[i]
		matched, detail := evaluateExpr(rule.When, request)
		result.Trace = append(result.Trace, TraceEntry{Rule: rule.Name, Matched: matched, Detail: detail})
		if matched {
			result.Rule, result.Action = rule, rule.Action
			result.Allowed = rule.Action.Kind == ast.Allow
			return result
		}
	}
	result.Default = true
	if policy.Default != nil {
		result.Action = *policy.Default
	}
	result.Allowed = result.Action.Kind == ast.Allow
	result.Trace = append(result.Trace, TraceEntry{Rule: "default", Matched: true, Detail: "no rule matched"})
	return result
}

// Explain is an explicit alias for Evaluate for callers that want trace intent.
func Explain(policy *ast.Policy, request Request) Result { return Evaluate(policy, request) }

func evaluateExpr(expr ast.Expr, request Request) (bool, string) {
	if expr == nil {
		return false, "missing condition"
	}
	value, detail := valueOf(expr, request)
	if boolean, ok := value.(bool); ok {
		return boolean, detail
	}
	return truthy(value), detail
}

func valueOf(expr ast.Expr, request Request) (any, string) {
	switch e := expr.(type) {
	case ast.Identifier:
		value := resolve(e.Name, request)
		return value, fmt.Sprintf("%s = %q", e.Name, value)
	case ast.StringLiteral:
		return e.Value, strconv.Quote(e.Value)
	case ast.NumberLiteral:
		return e.Value, strconv.FormatFloat(e.Value, 'f', -1, 64)
	case ast.BoolLiteral:
		return e.Value, strconv.FormatBool(e.Value)
	case ast.ListExpr:
		values := make([]any, 0, len(e.Values))
		details := make([]string, 0, len(e.Values))
		for _, item := range e.Values {
			value, detail := valueOf(item, request)
			values = append(values, value)
			details = append(details, detail)
		}
		return values, "[" + strings.Join(details, ", ") + "]"
	case ast.UnaryExpr:
		v, detail := valueOf(e.Expr, request)
		return !truthy(v), "not (" + detail + ")"
	case ast.BinaryExpr:
		left, leftDetail := valueOf(e.Left, request)
		if e.Op == ast.And && !truthy(left) {
			return false, "(" + leftDetail + ") and (short-circuit)"
		}
		if e.Op == ast.Or && truthy(left) {
			return true, "(" + leftDetail + ") or (short-circuit)"
		}
		right, rightDetail := valueOf(e.Right, request)
		if e.Op == ast.And {
			return truthy(left) && truthy(right), "(" + leftDetail + ") and (" + rightDetail + ")"
		}
		return truthy(left) || truthy(right), "(" + leftDetail + ") or (" + rightDetail + ")"
	case ast.CompareExpr:
		left, leftDetail := valueOf(e.Left, request)
		right, rightDetail := valueOf(e.Right, request)
		matched := compare(left, e.Op, right)
		return matched, fmt.Sprintf("%s %s %s = %t", leftDetail, e.Op, rightDetail, matched)
	default:
		return false, "unsupported expression"
	}
}

func resolve(name string, request Request) any {
	switch name {
	case "method":
		return request.Method
	case "path":
		return request.Path
	case "host":
		return request.Host
	case "scheme":
		return request.Scheme
	case "remote_addr":
		return request.RemoteAddr
	case "user_agent":
		return request.UserAgent
	case "role":
		return request.Role
	}
	if key, ok := indexed(name, "headers"); ok {
		return request.Headers.Get(key)
	}
	if key, ok := indexed(name, "query"); ok {
		return request.Query[key]
	}
	if key, ok := indexed(name, "fields"); ok {
		return request.Fields[key]
	}
	return ""
}

func indexed(name, prefix string) (string, bool) {
	start := len(prefix) + 1
	if !strings.HasPrefix(name, prefix+"[") || !strings.HasSuffix(name, "]") || len(name) <= start {
		return "", false
	}
	return strings.Trim(name[start:len(name)-1], "\"'"), true
}

func compare(left any, op ast.CompareOp, right any) bool {
	if op == ast.In {
		items, ok := right.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if compare(left, ast.Equal, item) {
				return true
			}
		}
		return false
	}
	if op == ast.Contains || op == ast.Matches {
		l, lok := left.(string)
		r, rok := right.(string)
		if !lok || !rok {
			return false
		}
		if op == ast.Contains {
			return strings.Contains(l, r)
		}
		matched, err := regexp.MatchString(r, l)
		return err == nil && matched
	}
	if lf, lok := number(left); lok {
		if rf, rok := number(right); rok {
			return compareNumbers(lf, op, rf)
		}
	}
	ls, lok := left.(string)
	rs, rok := right.(string)
	if lok && rok {
		return compareStrings(ls, op, rs)
	}
	lb, lok := left.(bool)
	rb, rok := right.(bool)
	if lok && rok {
		if op == ast.Equal {
			return lb == rb
		}
		if op == ast.NotEqual {
			return lb != rb
		}
	}
	return op == ast.NotEqual
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	}
	return 0, false
}
func compareNumbers(left float64, op ast.CompareOp, right float64) bool {
	switch op {
	case ast.Equal:
		return left == right
	case ast.NotEqual:
		return left != right
	case ast.Less:
		return left < right
	case ast.LessEqual:
		return left <= right
	case ast.Greater:
		return left > right
	case ast.GreaterEqual:
		return left >= right
	}
	return false
}
func compareStrings(left string, op ast.CompareOp, right string) bool {
	switch op {
	case ast.Equal:
		return left == right
	case ast.NotEqual:
		return left != right
	case ast.Less:
		return left < right
	case ast.LessEqual:
		return left <= right
	case ast.Greater:
		return left > right
	case ast.GreaterEqual:
		return left >= right
	}
	return false
}
func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	default:
		return value != nil
	}
}
