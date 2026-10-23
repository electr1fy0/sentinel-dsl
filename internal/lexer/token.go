package lexer

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/ayush/sentinel-dsl/internal/diagnostics"
)

type TokenKind uint8

const (
	EOF TokenKind = iota
	Identifier
	String
	Number
	LBrace
	RBrace
	LParen
	RParen
	LBracket
	RBracket
	Comma
	Dot
	Semicolon
	EqualEqual
	NotEqual
	Less
	LessEqual
	Greater
	GreaterEqual
	And
	Or
	Not
	Contains
	Matches
	In
	Policy
	Rule
	When
	Then
	Default
	Allow
	Deny
	True
	False
)

func (k TokenKind) String() string {
	names := [...]string{"eof", "identifier", "string", "number", "{", "}", "(", ")", "[", "]", ",", ".", ";", "==", "!=", "<", "<=", ">", ">=", "and", "or", "not", "contains", "matches", "in", "policy", "rule", "when", "then", "default", "allow", "deny", "true", "false"}
	if int(k) < len(names) {
		return names[k]
	}
	return "unknown"
}

type Token struct {
	Kind  TokenKind
	Text  string
	Value string
	Span  diagnostics.Span
}

func (t Token) String() string { return fmt.Sprintf("%s(%q)", t.Kind, t.Text) }

var keywords = map[string]TokenKind{
	"policy": Policy, "rule": Rule, "when": When, "then": Then, "default": Default,
	"allow": Allow, "deny": Deny, "and": And, "or": Or, "not": Not,
	"contains": Contains, "matches": Matches, "in": In, "true": True, "false": False,
}

// Lex converts source into tokens and returns recoverable lexical diagnostics.
func Lex(source string) ([]Token, diagnostics.Diagnostics) {
	l := &scanner{source: []rune(source), line: 1, column: 1}
	var tokens []Token
	var ds diagnostics.Diagnostics
	for {
		t := l.next(&ds)
		tokens = append(tokens, t)
		if t.Kind == EOF {
			return tokens, ds
		}
	}
}

type scanner struct {
	source []rune
	index  int
	offset int
	line   int
	column int
}

func (s *scanner) position() diagnostics.Position {
	return diagnostics.Position{Offset: s.offset, Line: s.line, Column: s.column}
}

func (s *scanner) advance() rune {
	if s.index >= len(s.source) {
		return 0
	}
	r := s.source[s.index]
	s.index++
	s.offset += len(string(r))
	if r == '\n' {
		s.line, s.column = s.line+1, 1
	} else {
		s.column++
	}
	return r
}

func (s *scanner) peek() rune {
	if s.index >= len(s.source) {
		return 0
	}
	return s.source[s.index]
}

func (s *scanner) match(want rune) bool {
	if s.peek() != want {
		return false
	}
	s.advance()
	return true
}

func (s *scanner) next(ds *diagnostics.Diagnostics) Token {
	for {
		if unicode.IsSpace(s.peek()) {
			s.advance()
			continue
		}
		if s.peek() == '#' || (s.peek() == '/' && s.index+1 < len(s.source) && s.source[s.index+1] == '/') {
			for s.peek() != 0 && s.peek() != '\n' {
				s.advance()
			}
			continue
		}
		break
	}
	start := s.position()
	startIndex := s.index
	r := s.peek()
	if r == 0 {
		return Token{Kind: EOF, Span: diagnostics.Span{Start: start, End: start}}
	}
	if unicode.IsLetter(r) || r == '_' {
		for unicode.IsLetter(s.peek()) || unicode.IsDigit(s.peek()) || s.peek() == '_' || s.peek() == '-' {
			s.advance()
		}
		text := string(s.source[startIndex:s.index])
		if kind, ok := keywords[text]; ok {
			return s.token(kind, text, start)
		}
		return s.token(Identifier, text, start)
	}
	if unicode.IsDigit(r) {
		dot := false
		for unicode.IsDigit(s.peek()) || (!dot && s.peek() == '.') {
			if s.peek() == '.' {
				dot = true
			}
			s.advance()
		}
		text := string(s.source[startIndex:s.index])
		return s.tokenValue(Number, text, text, start)
	}
	if r == '"' || r == '\'' {
		return s.scanString(start, ds)
	}
	s.advance()
	kind := TokenKind(0)
	switch r {
	case '{':
		kind = LBrace
	case '}':
		kind = RBrace
	case '(':
		kind = LParen
	case ')':
		kind = RParen
	case '[':
		kind = LBracket
	case ']':
		kind = RBracket
	case ',':
		kind = Comma
	case '.':
		kind = Dot
	case ';':
		kind = Semicolon
	case '!':
		if s.match('=') {
			kind = NotEqual
		}
	case '=':
		if s.match('=') {
			kind = EqualEqual
		}
	case '<':
		if s.match('=') {
			kind = LessEqual
		} else {
			kind = Less
		}
	case '>':
		if s.match('=') {
			kind = GreaterEqual
		} else {
			kind = Greater
		}
	}
	if kind == 0 {
		ds.Errorf(diagnostics.Span{Start: start, End: s.position()}, "unexpected character %q", r)
		return s.token(EOF, "", start)
	}
	return s.tokenValue(kind, string(s.source[startIndex:s.index]), string(s.source[startIndex:s.index]), start)
}

func (s *scanner) scanString(start diagnostics.Position, ds *diagnostics.Diagnostics) Token {
	quote := s.advance()
	var value []rune
	for {
		r := s.peek()
		if r == 0 || r == '\n' {
			ds.Errorf(diagnostics.Span{Start: start, End: s.position()}, "unterminated string")
			break
		}
		if r == quote {
			s.advance()
			break
		}
		if r == '\\' {
			s.advance()
			escaped := s.peek()
			s.advance()
			switch escaped {
			case 'n':
				value = append(value, '\n')
			case 'r':
				value = append(value, '\r')
			case 't':
				value = append(value, '\t')
			case '\\', '"', '\'':
				value = append(value, escaped)
			default:
				ds.Errorf(diagnostics.Span{Start: start, End: s.position()}, "unknown escape \\\\%c", escaped)
			}
			continue
		}
		value = append(value, s.advance())
	}
	return s.tokenValue(String, string(value), string(value), start)
}

func (s *scanner) token(kind TokenKind, text string, start diagnostics.Position) Token {
	return s.tokenValue(kind, text, text, start)
}
func (s *scanner) tokenValue(kind TokenKind, text, value string, start diagnostics.Position) Token {
	return Token{Kind: kind, Text: text, Value: value, Span: diagnostics.Span{Start: start, End: s.position()}}
}

func ParseNumber(text string) (float64, error) { return strconv.ParseFloat(text, 64) }
