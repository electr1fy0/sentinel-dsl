package ast

import "github.com/ayush/sentinel-dsl/internal/diagnostics"

type Span = diagnostics.Span

// Node is implemented by every syntax node.
type Node interface{ Span() Span }

// Policy is the top-level SentinelDSL document.
type Policy struct {
	Name    string
	Rules   []Rule
	Default *Action
	span    Span
}

func (p *Policy) Span() Span { return p.span }

type Rule struct {
	Name   string
	When   Expr
	Action Action
	span   Span
}

func (r Rule) Span() Span { return r.span }

type ActionKind uint8

const (
	Allow ActionKind = iota
	Deny
)

func (k ActionKind) String() string {
	if k == Deny {
		return "deny"
	}
	return "allow"
}

type Action struct {
	Kind    ActionKind
	Status  int
	Message string
	span    Span
}

func (a Action) Span() Span { return a.span }

type Expr interface {
	Node
	exprNode()
}

type Identifier struct {
	Name string
	span Span
}

func (i Identifier) Span() Span { return i.span }
func (Identifier) exprNode()    {}

type StringLiteral struct {
	Value string
	span  Span
}

func (s StringLiteral) Span() Span { return s.span }
func (StringLiteral) exprNode()    {}

type NumberLiteral struct {
	Value float64
	span  Span
}

func (n NumberLiteral) Span() Span { return n.span }
func (NumberLiteral) exprNode()    {}

type BoolLiteral struct {
	Value bool
	span  Span
}

func (b BoolLiteral) Span() Span { return b.span }
func (BoolLiteral) exprNode()    {}

type UnaryExpr struct {
	Op   UnaryOp
	Expr Expr
	span Span
}

type UnaryOp uint8

const Not UnaryOp = iota

func (o UnaryOp) String() string { return "not" }

func (u UnaryExpr) Span() Span { return u.span }
func (UnaryExpr) exprNode()    {}

type BinaryExpr struct {
	Left  Expr
	Op    BinaryOp
	Right Expr
	span  Span
}

type BinaryOp uint8

const (
	And BinaryOp = iota
	Or
)

func (o BinaryOp) String() string {
	if o == Or {
		return "or"
	}
	return "and"
}

func (b BinaryExpr) Span() Span { return b.span }
func (BinaryExpr) exprNode()    {}

type CompareExpr struct {
	Left  Expr
	Op    CompareOp
	Right Expr
	span  Span
}

type CompareOp uint8

const (
	Equal CompareOp = iota
	NotEqual
	Less
	LessEqual
	Greater
	GreaterEqual
	Contains
	Matches
	In
)

func (c CompareExpr) Span() Span { return c.span }
func (CompareExpr) exprNode()    {}

func (o CompareOp) String() string {
	return [...]string{"==", "!=", "<", "<=", ">", ">=", "contains", "matches", "in"}[o]
}

type ListExpr struct {
	Values []Expr
	span   Span
}

func (l ListExpr) Span() Span { return l.span }
func (ListExpr) exprNode()    {}

// Constructors are useful to callers that need to build policies programmatically.
func NewPolicy(name string, rules []Rule, defaultAction Action) *Policy {
	return &Policy{Name: name, Rules: rules, Default: &defaultAction}
}

func NewPolicyAt(name string, rules []Rule, defaultAction *Action, span Span) *Policy {
	return &Policy{Name: name, Rules: rules, Default: defaultAction, span: span}
}

func NewRule(name string, when Expr, action Action) Rule {
	return Rule{Name: name, When: when, Action: action}
}

func NewRuleAt(name string, when Expr, action Action, span Span) Rule {
	return Rule{Name: name, When: when, Action: action, span: span}
}

func NewAction(kind ActionKind) Action { return Action{Kind: kind} }
func NewActionAt(kind ActionKind, status int, message string, span Span) Action {
	return Action{Kind: kind, Status: status, Message: message, span: span}
}

func NewIdentifier(name string) Identifier              { return Identifier{Name: name} }
func NewIdentifierAt(name string, span Span) Identifier { return Identifier{Name: name, span: span} }
func NewStringLiteralAt(value string, span Span) StringLiteral {
	return StringLiteral{Value: value, span: span}
}
func NewNumberLiteralAt(value float64, span Span) NumberLiteral {
	return NumberLiteral{Value: value, span: span}
}
func NewBoolLiteralAt(value bool, span Span) BoolLiteral {
	return BoolLiteral{Value: value, span: span}
}
func NewUnaryAt(op UnaryOp, expr Expr, span Span) UnaryExpr {
	return UnaryExpr{Op: op, Expr: expr, span: span}
}
func NewBinaryAt(left Expr, op BinaryOp, right Expr, span Span) BinaryExpr {
	return BinaryExpr{Left: left, Op: op, Right: right, span: span}
}
func NewCompareAt(left Expr, op CompareOp, right Expr, span Span) CompareExpr {
	return CompareExpr{Left: left, Op: op, Right: right, span: span}
}
func NewListAt(values []Expr, span Span) ListExpr { return ListExpr{Values: values, span: span} }
