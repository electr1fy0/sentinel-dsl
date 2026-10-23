package csg

import "testing"

func TestBoundedRecognition(t *testing.T) {
	g := Grammar{
		Start:     "S",
		Variables: []string{"S", "A", "B"},
		Terminals: []string{"a", "b"},
		Productions: []Production{
			{[]string{"S"}, []string{"a", "A"}},
			{[]string{"A"}, []string{"b"}},
		},
	}
	r := g.Recognize([]string{"a", "b"}, Options{MaxLength: 4, MaxSteps: 3})
	if !r.Accepted || len(r.Derivation) != 3 {
		t.Fatalf("result=%+v", r)
	}
	if g.Recognize([]string{"a", "a"}, Options{MaxLength: 4, MaxSteps: 3}).Accepted {
		t.Fatal("invalid string accepted")
	}
}

func TestCSGEdgeCases(t *testing.T) {
	g := Grammar{
		Start:     "S",
		Variables: []string{"S", "A", "B"},
		Terminals: []string{"a"},
		Productions: []Production{
			// 1. Start epsilon rule (allowed contracting rule)
			{[]string{"S"}, []string{}},
			// 2. Non-start contracting rule (ignored)
			{[]string{"A", "B"}, []string{"a"}},
			// 3. Normal expanding rule
			{[]string{"S"}, []string{"a", "a", "a"}},
		},
	}

	// Test recognizing empty string via Start -> ε with MaxLength <= 0 default
	rEmpty := g.Recognize([]string{}, Options{MaxLength: 0, MaxSteps: 5})
	if !rEmpty.Accepted {
		t.Fatal("expected empty string accepted via Start -> ε")
	}

	// Test MaxLength limit exceeded
	rTooLong := g.Recognize([]string{"a", "a", "a"}, Options{MaxLength: 2, MaxSteps: 5})
	if rTooLong.Accepted {
		t.Fatal("expected string longer than MaxLength to be rejected")
	}

	// Test MaxSteps limit hit
	rTimeout := g.Recognize([]string{"a", "a", "a"}, Options{MaxLength: 10, MaxSteps: 0})
	if rTimeout.Accepted {
		t.Fatal("expected maxSteps 0 to reject non-start form")
	}

	// Test equal helper
	if equal([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("expected unequal lengths to return false")
	}
	if equal([]string{"a", "b"}, []string{"a", "c"}) {
		t.Fatal("expected unequal content to return false")
	}
	if !equal([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatal("expected equal slices to return true")
	}
}
