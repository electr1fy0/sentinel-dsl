package regex

import "testing"

func TestParseAndThompson(t *testing.T) {
	e, err := Parse("a(b|c)*")
	if err != nil {
		t.Fatal(err)
	}
	n := Thompson(e)
	if !n.Run([]string{"a"}) || !n.Run([]string{"a", "b", "c", "b"}) || n.Run([]string{"b"}) {
		t.Fatal("unexpected regex language")
	}
}

func TestQuotedLiteralAndEmpty(t *testing.T) {
	e, err := Parse(`\|`)
	if err != nil || !Thompson(e).Run([]string{"|"}) {
		t.Fatal("quoted alternation was not parsed")
	}
	e, err = Parse("ε")
	if err != nil || !Thompson(e).Run(nil) {
		t.Fatal("epsilon was not accepted")
	}
	// Empty language symbol ∅
	eEmpty, err := Parse("∅")
	if err != nil {
		t.Fatal(err)
	}
	nEmpty := Thompson(eEmpty)
	if nEmpty.Run(nil) || nEmpty.Run([]string{"a"}) {
		t.Fatal("empty language should reject all inputs")
	}
}

func TestPlusOptionalAndRepeatedSymbols(t *testing.T) {
	// Plus operator (+)
	ePlus, err := Parse("a+")
	if err != nil {
		t.Fatal(err)
	}
	nPlus := Thompson(ePlus)
	if nPlus.Run(nil) || !nPlus.Run([]string{"a"}) || !nPlus.Run([]string{"a", "a"}) {
		t.Fatal("unexpected behavior for a+")
	}

	// Optional operator (?)
	eOpt, err := Parse("a?")
	if err != nil {
		t.Fatal(err)
	}
	nOpt := Thompson(eOpt)
	if !nOpt.Run(nil) || !nOpt.Run([]string{"a"}) || nOpt.Run([]string{"a", "a"}) {
		t.Fatal("unexpected behavior for a?")
	}

	// Repeated symbols in alphabet
	eRep, err := Parse("aa*a")
	if err != nil {
		t.Fatal(err)
	}
	nRep := Thompson(eRep)
	if !nRep.Run([]string{"a", "a"}) {
		t.Fatal("expected aa to be accepted")
	}

	// Empty pattern
	eEmptyStr, err := Parse("")
	if err != nil || !Thompson(eEmptyStr).Run(nil) {
		t.Fatal("empty pattern should accept empty input")
	}
}

func TestParseErrors(t *testing.T) {
	errorPatterns := []string{
		"a)",       // unexpected character
		"(",        // expected expression
		"(a",       // unclosed group
		"(*",       // unexpected character '*' in group
		"((*",      // error in nested group alternation
		")",        // unexpected character ')'
		"*",        // unexpected character '*'
		"+",        // unexpected character '+'
		"?",        // unexpected character '?'
		"a|*",      // error in alternation right operand
		`\`,        // dangling escape
	}
	for _, p := range errorPatterns {
		if _, err := Parse(p); err == nil {
			t.Fatalf("expected error for pattern %q", p)
		}
	}

	// Direct call to parseAtom when pos >= len(input)
	pEmpty := &parser{}
	if _, err := pEmpty.parseAtom(); err == nil {
		t.Fatal("expected error from parseAtom on empty parser")
	}
}
