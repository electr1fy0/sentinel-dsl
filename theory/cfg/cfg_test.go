package cfg

import "testing"

func TestFirstFollowAndDerivation(t *testing.T) {
	g := Grammar{
		Variables: []string{"S", "A", "B"},
		Terminals: []string{"a", "b", "c"},
		Start:     "S",
		Productions: []Production{
			{"S", []string{"A", "b"}},
			{"S", []string{"a", "A"}}, // derivation with leading terminal
			{"S", []string{"A"}},      // follow propagation from S to A
			{"S", []string{"A", "B"}}, // all symbols nullable in First()
			{"A", []string{}},
			{"A", []string{"a"}},
			{"A", []string{"c"}}, // brings 'c' into first[A] and then first[S]
			{"B", []string{}},
		},
	}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	first := g.First()
	if !first["A"][Epsilon] || !first["S"]["a"] || !first["S"]["b"] || !first["S"]["c"] || !first["S"][Epsilon] {
		t.Fatalf("FIRST=%v", first)
	}
	follow := g.Follow()
	if !follow["A"]["b"] || !follow["A"]["$"] {
		t.Fatalf("missing FOLLOW(A): %v", follow["A"])
	}
	forms := g.LeftmostDerivations(3)
	if len(forms) == 0 {
		t.Fatalf("expected leftmost derivations")
	}
}

func TestSimplify(t *testing.T) {
	g := Grammar{
		Variables: []string{"S", "A", "B", "C", "U", "Dead"},
		Terminals: []string{"a", "c"},
		Start:     "S",
		Productions: []Production{
			{"B", []string{"C"}}, // chained unit production
			{"A", []string{"B"}},
			{"S", []string{"A"}},
			{"A", []string{}},
			{"C", []string{"a"}},
			{"U", []string{"a"}},
			{"Dead", []string{"Dead"}}, // non-generating
			{"S", []string{"c", "Dead"}},
		},
	}
	s := g.Simplify()
	if len(s.Variables) != 1 || s.Variables[0] != "S" {
		t.Fatalf("variables=%v", s.Variables)
	}
	if len(s.Productions) == 0 {
		t.Fatal("simplification removed all rules")
	}
}

func TestGrammarValidationErrors(t *testing.T) {
	// Undeclared start variable
	g1 := Grammar{Variables: []string{"A"}, Terminals: []string{"a"}, Start: "S"}
	if err := g1.Validate(); err == nil {
		t.Fatal("expected error for undeclared start variable")
	}

	// Production LHS not a variable
	g2 := Grammar{Variables: []string{"S"}, Terminals: []string{"a"}, Start: "S", Productions: []Production{{"X", []string{"a"}}}}
	if err := g2.Validate(); err == nil {
		t.Fatal("expected error for non-variable LHS")
	}

	// Symbol in RHS neither variable nor terminal
	g3 := Grammar{Variables: []string{"S"}, Terminals: []string{"a"}, Start: "S", Productions: []Production{{"S", []string{"unknown"}}}}
	if err := g3.Validate(); err == nil {
		t.Fatal("expected error for unknown symbol in RHS")
	}
}

func TestDeriveAndLeftmostDerivations(t *testing.T) {
	g := Grammar{
		Variables: []string{"S", "A"},
		Terminals: []string{"a"},
		Start:     "S",
		Productions: []Production{
			{"S", []string{"A"}},
			{"A", []string{"a"}},
		},
	}

	// Derive when LHS not in form
	if _, ok := g.Derive([]string{"b", "c"}, Production{"S", []string{"a"}}); ok {
		t.Fatal("expected Derive to return false when LHS not in form")
	}

	// LeftmostDerivations negative maxSteps
	if d := g.LeftmostDerivations(-1); d != nil {
		t.Fatal("expected nil for negative maxSteps")
	}

	// LeftmostDerivations step limit hit
	d0 := g.LeftmostDerivations(0)
	if len(d0) != 0 {
		t.Fatalf("expected 0 terminal forms at step limit 0, got %d", len(d0))
	}
}

func TestFirstSequenceAndSorted(t *testing.T) {
	g := Grammar{
		Variables: []string{"S", "A", "B"},
		Terminals: []string{"a", "b"},
		Start:     "S",
	}

	first := map[string]map[string]bool{
		"S": {"a": true},
		"A": {Epsilon: true},
		"B": {"b": true}, // non-nullable
	}

	// Empty sequence
	fsEmpty := g.FirstSequence([]string{}, first)
	if !fsEmpty[Epsilon] {
		t.Fatal("expected epsilon in FirstSequence of empty slice")
	}

	// Sequence of nullable variable followed by terminal
	fsSeq := g.FirstSequence([]string{"A", "a"}, first)
	if !fsSeq["a"] {
		t.Fatal("expected 'a' in FirstSequence([A, a])")
	}

	// Sequence starting with non-nullable variable
	fsNonNullable := g.FirstSequence([]string{"B", "A"}, first)
	if !fsNonNullable["b"] || fsNonNullable[Epsilon] {
		t.Fatal("expected 'b' and not epsilon in FirstSequence([B, A])")
	}

	// Sequence of only nullable variable
	fsNullable := g.FirstSequence([]string{"A"}, first)
	if !fsNullable[Epsilon] {
		t.Fatal("expected epsilon in FirstSequence([A])")
	}

	// Sorted helper
	sorted := Sorted(map[string]bool{"b": true, "a": true})
	if len(sorted) != 2 || sorted[0] != "a" || sorted[1] != "b" {
		t.Fatalf("unexpected sorted result: %v", sorted)
	}
}
