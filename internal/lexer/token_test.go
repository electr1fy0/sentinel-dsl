package lexer

import (
	"strings"
	"testing"
)

func TestLexLocationsAndComments(t *testing.T) {
	tokens, diagnostics := Lex("# comment\n// second comment\npolicy p { rule r when method == \"GET\" then allow }")
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if len(tokens) < 10 || tokens[0].Kind != Policy {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
	if tokens[0].Span.Start.Line != 3 || tokens[0].Span.Start.Column != 1 {
		t.Fatalf("policy location = %#v", tokens[0].Span.Start)
	}
	if tokens[8].Kind != String || tokens[8].Value != "GET" {
		t.Fatalf("string token = %#v", tokens[8])
	}
}

func TestLexStringEscape(t *testing.T) {
	tokens, diagnostics := Lex(`policy p { default deny "hello\n\r\t\\\"\'world" }`)
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	expected := "hello\n\r\t\\\"'world"
	if tokens[5].Value != expected {
		t.Fatalf("value = %q, expected %q", tokens[5].Value, expected)
	}

	// Single quoted string
	tokens2, diag2 := Lex(`'single quoted'`)
	if diag2.HasErrors() {
		t.Fatalf("unexpected error: %v", diag2)
	}
	if len(tokens2) != 2 || tokens2[0].Value != "single quoted" {
		t.Fatalf("single quote token = %#v", tokens2)
	}

	// String with unknown escape
	_, diag3 := Lex(`"bad \z escape"`)
	if !diag3.HasErrors() {
		t.Fatalf("expected error for unknown escape")
	}

	// Unterminated string with newline
	_, diag4 := Lex("\"newline\nunterminated\"")
	if !diag4.HasErrors() {
		t.Fatalf("expected error for unterminated string with newline")
	}

	// Unterminated string with EOF
	_, diag5 := Lex(`"eof unterminated`)
	if !diag5.HasErrors() {
		t.Fatalf("expected error for unterminated string with EOF")
	}
}

func TestLexTokensAndKeywords(t *testing.T) {
	src := `
		policy rule when then default allow deny
		and or not contains matches in true false
		_ident my-ident123 456 12.34
		{ } ( ) [ ] , . ; == != < <= > >=
	`
	tokens, diags := Lex(src)
	if diags.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	kinds := make([]TokenKind, 0, len(tokens))
	for _, tok := range tokens {
		kinds = append(kinds, tok.Kind)
	}

	expected := []TokenKind{
		Policy, Rule, When, Then, Default, Allow, Deny,
		And, Or, Not, Contains, Matches, In, True, False,
		Identifier, Identifier, Number, Number,
		LBrace, RBrace, LParen, RParen, LBracket, RBracket, Comma, Dot, Semicolon,
		EqualEqual, NotEqual, Less, LessEqual, Greater, GreaterEqual,
		EOF,
	}

	if len(kinds) != len(expected) {
		t.Fatalf("token count mismatch: got %d, expected %d: %#v", len(kinds), len(expected), kinds)
	}
	for i := range expected {
		if kinds[i] != expected[i] {
			t.Errorf("token %d mismatch: got %v (%s), expected %v", i, kinds[i], kinds[i], expected[i])
		}
	}
}

func TestLexErrors(t *testing.T) {
	// Single '!' without '='
	_, diag1 := Lex("!")
	if !diag1.HasErrors() {
		t.Fatalf("expected error for lone '!'")
	}

	// Single '=' without '='
	_, diag2 := Lex("=")
	if !diag2.HasErrors() {
		t.Fatalf("expected error for lone '='")
	}

	// Unexpected character
	_, diag3 := Lex("@")
	if !diag3.HasErrors() {
		t.Fatalf("expected error for '@'")
	}
}

func TestTokenKindString(t *testing.T) {
	for k := TokenKind(0); k <= False; k++ {
		str := k.String()
		if str == "unknown" {
			t.Fatalf("unexpected unknown for token kind %d", k)
		}
	}
	if TokenKind(255).String() != "unknown" {
		t.Fatalf("expected unknown for TokenKind(255)")
	}

	tok := Token{Kind: Identifier, Text: "foo"}
	if !strings.Contains(tok.String(), "identifier(\"foo\")") {
		t.Fatalf("unexpected tok.String(): %q", tok.String())
	}
}

func TestParseNumber(t *testing.T) {
	n, err := ParseNumber("123.45")
	if err != nil || n != 123.45 {
		t.Fatalf("unexpected ParseNumber result: %v, %v", n, err)
	}
	_, err = ParseNumber("abc")
	if err == nil {
		t.Fatalf("expected error for invalid number")
	}

	// Direct scanner boundary test
	s := &scanner{}
	if r := s.advance(); r != 0 {
		t.Fatalf("expected 0 from advance at EOF, got %v", r)
	}
	if r := s.peek(); r != 0 {
		t.Fatalf("expected 0 from peek at EOF, got %v", r)
	}
}
