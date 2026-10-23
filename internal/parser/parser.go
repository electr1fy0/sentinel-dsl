package parser

import (
	"fmt"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/diagnostics"
	"github.com/ayush/sentinel-dsl/internal/lexer"
)

// Parse parses one policy and returns all recoverable syntax diagnostics.
func Parse(source string) (*ast.Policy, diagnostics.Diagnostics) {
	tokens, ds := lexer.Lex(source)
	p := &parser{tokens: tokens, diagnostics: ds}
	policy := p.parsePolicy()
	return policy, p.diagnostics
}

type parser struct {
	tokens      []lexer.Token
	index       int
	diagnostics diagnostics.Diagnostics
}

func (p *parser) current() lexer.Token {
	if p.index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.index]
}
func (p *parser) advance() lexer.Token {
	t := p.current()
	if t.Kind != lexer.EOF {
		p.index++
	}
	return t
}
func (p *parser) at(kind lexer.TokenKind) bool { return p.current().Kind == kind }
func (p *parser) accept(kind lexer.TokenKind) bool {
	if p.at(kind) {
		p.advance()
		return true
	}
	return false
}
func (p *parser) expect(kind lexer.TokenKind) lexer.Token {
	if p.at(kind) {
		return p.advance()
	}
	t := p.current()
	p.diagnostics.Errorf(t.Span, "expected %s, got %s", kind, t.Kind)
	return lexer.Token{Kind: kind, Span: t.Span}
}

func (p *parser) parsePolicy() *ast.Policy {
	start := p.current().Span.Start
	p.expect(lexer.Policy)
	name := p.parseName("policy name")
	p.expect(lexer.LBrace)
	var rules []ast.Rule
	var defaultAction *ast.Action
	for !p.at(lexer.RBrace) && !p.at(lexer.EOF) {
		if p.accept(lexer.Semicolon) {
			continue
		}
		switch p.current().Kind {
		case lexer.Rule:
			rules = append(rules, p.parseRule())
		case lexer.Default:
			if defaultAction != nil {
				p.diagnostics.Errorf(p.current().Span, "policy has more than one default action")
			}
			p.advance()
			action := p.parseAction()
			p.accept(lexer.Semicolon)
			defaultAction = &action
		default:
			p.diagnostics.Errorf(p.current().Span, "expected rule or default")
			p.synchronizeTopLevel()
		}
	}
	end := p.expect(lexer.RBrace).Span.End
	if p.at(lexer.RBrace) {
		end = p.advance().Span.End
	}
	if !p.at(lexer.EOF) {
		p.diagnostics.Errorf(p.current().Span, "unexpected token after policy")
	}
	return ast.NewPolicyAt(name, rules, defaultAction, diagnostics.Span{Start: start, End: end})
}

func (p *parser) parseRule() ast.Rule {
	start := p.advance().Span.Start
	name := p.parseName("rule name")
	p.expect(lexer.When)
	condition := p.parseOr()
	p.accept(lexer.Then)
	action := p.parseAction()
	p.accept(lexer.Semicolon)
	return ast.NewRuleAt(name, condition, action, diagnostics.Span{Start: start, End: action.Span().End})
}

func (p *parser) parseName(what string) string {
	t := p.current()
	if t.Kind == lexer.Identifier || t.Kind == lexer.String {
		p.advance()
		return t.Value
	}
	p.diagnostics.Errorf(t.Span, "expected %s", what)
	return ""
}

func (p *parser) parseAction() ast.Action {
	t := p.current()
	var kind ast.ActionKind
	switch t.Kind {
	case lexer.Allow:
		kind = ast.Allow
	case lexer.Deny:
		kind = ast.Deny
	default:
		p.diagnostics.Errorf(t.Span, "expected allow or deny action")
		return ast.NewActionAt(ast.Deny, 0, "", t.Span)
	}
	p.advance()
	status := 0
	message := ""
	if p.at(lexer.Number) {
		value, err := lexer.ParseNumber(p.advance().Value)
		if err != nil {
			p.diagnostics.Errorf(t.Span, "invalid action status: %v", err)
		} else if value != float64(int(value)) {
			p.diagnostics.Errorf(t.Span, "action status must be an integer")
		} else {
			status = int(value)
		}
	}
	if p.at(lexer.String) {
		message = p.advance().Value
	}
	return ast.NewActionAt(kind, status, message, diagnostics.Span{Start: t.Span.Start, End: p.tokens[p.index-1].Span.End})
}

func (p *parser) synchronizeTopLevel() {
	for !p.at(lexer.EOF) && !p.at(lexer.Rule) && !p.at(lexer.Default) && !p.at(lexer.RBrace) {
		p.advance()
	}
}

func (p *parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.accept(lexer.Or) {
		op := p.tokens[p.index-1]
		right := p.parseAnd()
		left = ast.NewBinaryAt(left, ast.Or, right, diagnostics.Span{Start: left.Span().Start, End: right.Span().End})
		_ = op
	}
	return left
}

func (p *parser) parseAnd() ast.Expr {
	left := p.parseUnary()
	for p.accept(lexer.And) {
		right := p.parseUnary()
		left = ast.NewBinaryAt(left, ast.And, right, diagnostics.Span{Start: left.Span().Start, End: right.Span().End})
	}
	return left
}

func (p *parser) parseUnary() ast.Expr {
	if p.accept(lexer.Not) {
		start := p.tokens[p.index-1].Span.Start
		expr := p.parseUnary()
		return ast.NewUnaryAt(ast.Not, expr, diagnostics.Span{Start: start, End: expr.Span().End})
	}
	if p.accept(lexer.LParen) {
		expr := p.parseOr()
		p.expect(lexer.RParen)
		return expr
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() ast.Expr {
	left := p.parsePrimary()
	op, ok := p.comparisonOp(p.current().Kind)
	if !ok {
		return left
	}
	p.advance()
	right := p.parsePrimary()
	return ast.NewCompareAt(left, op, right, diagnostics.Span{Start: left.Span().Start, End: right.Span().End})
}

func (p *parser) comparisonOp(kind lexer.TokenKind) (ast.CompareOp, bool) {
	switch kind {
	case lexer.EqualEqual:
		return ast.Equal, true
	case lexer.NotEqual:
		return ast.NotEqual, true
	case lexer.Less:
		return ast.Less, true
	case lexer.LessEqual:
		return ast.LessEqual, true
	case lexer.Greater:
		return ast.Greater, true
	case lexer.GreaterEqual:
		return ast.GreaterEqual, true
	case lexer.Contains:
		return ast.Contains, true
	case lexer.Matches:
		return ast.Matches, true
	case lexer.In:
		return ast.In, true
	default:
		return ast.Equal, false
	}
}

func (p *parser) parsePrimary() ast.Expr {
	t := p.current()
	switch t.Kind {
	case lexer.Identifier:
		p.advance()
		name := t.Value
		for p.accept(lexer.Dot) {
			part := p.expect(lexer.Identifier)
			name += "." + part.Value
		}
		for p.accept(lexer.LBracket) {
			key := p.current()
			if key.Kind != lexer.String && key.Kind != lexer.Identifier {
				p.diagnostics.Errorf(key.Span, "expected field key")
			} else {
				p.advance()
				name += "[" + key.Value + "]"
			}
			p.expect(lexer.RBracket)
		}
		return ast.NewIdentifierAt(name, diagnostics.Span{Start: t.Span.Start, End: p.tokens[p.index-1].Span.End})
	case lexer.String:
		p.advance()
		return ast.NewStringLiteralAt(t.Value, t.Span)
	case lexer.Number:
		p.advance()
		value, err := lexer.ParseNumber(t.Value)
		if err != nil {
			p.diagnostics.Errorf(t.Span, "invalid number: %v", err)
		}
		return ast.NewNumberLiteralAt(value, t.Span)
	case lexer.True, lexer.False:
		p.advance()
		return ast.NewBoolLiteralAt(t.Kind == lexer.True, t.Span)
	case lexer.LBracket:
		start := p.advance().Span.Start
		values := []ast.Expr{}
		for !p.at(lexer.RBracket) && !p.at(lexer.EOF) {
			values = append(values, p.parsePrimary())
			if !p.accept(lexer.Comma) {
				break
			}
		}
		end := p.expect(lexer.RBracket).Span.End
		return ast.NewListAt(values, diagnostics.Span{Start: start, End: end})
	default:
		p.diagnostics.Errorf(t.Span, "expected expression, got %s", t.Kind)
		p.advance()
		return ast.NewBoolLiteralAt(false, t.Span)
	}
}

func (p *parser) String() string { return fmt.Sprintf("parser at token %d", p.index) }
