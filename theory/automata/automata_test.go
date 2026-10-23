package automata

import (
	"testing"
)

func TestNFAClosureSubsetAndMinimize(t *testing.T) {
	n := NFA{
		States:      []string{"q0", "q1", "q2"},
		Alphabet:    []string{"a", "b"},
		Start:       "q0",
		Accept:      map[string]bool{"q2": true},
		Transitions: map[string]map[string][]string{"q1": {"b": {"q2"}}},
		Epsilon:     map[string][]string{"q0": {"q1"}},
	}
	if !n.Run([]string{"b"}) || n.Run([]string{"a"}) {
		t.Fatal("unexpected NFA result")
	}
	// Run with input that leads to non-empty non-accepting state set
	nReject := NFA{
		States:      []string{"q0", "q1"},
		Alphabet:    []string{"a"},
		Start:       "q0",
		Accept:      map[string]bool{},
		Transitions: map[string]map[string][]string{"q0": {"a": {"q1"}}},
	}
	if nReject.Run([]string{"a"}) {
		t.Fatal("expected reject on non-accepting state")
	}

	d := n.SubsetConstruction()
	if !d.Run([]string{"b"}) || d.Run([]string{"a"}) {
		t.Fatal("unexpected subset DFA result")
	}
	if len(d.Minimize().States) == 0 {
		t.Fatal("minimization produced no states")
	}
}

func TestDFATraceAndValidation(t *testing.T) {
	d := DFA{
		States:      []string{"even", "odd"},
		Alphabet:    []string{"1"},
		Start:       "even",
		Accept:      map[string]bool{"even": true},
		Transitions: map[string]map[string]string{"even": {"1": "odd"}, "odd": {"1": "even"}},
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	trace, err := d.Trace([]string{"1", "1"})
	if err != nil || len(trace) != 3 {
		t.Fatalf("trace = %#v, err = %v", trace, err)
	}

	// Trace error on missing transition
	_, errMissing := d.Trace([]string{"2"})
	if errMissing == nil {
		t.Fatal("expected error on missing trace transition")
	}

	// Run on missing transition
	if d.Run([]string{"2"}) {
		t.Fatal("expected false on missing transition in Run")
	}
}

func TestDFAValidationErrors(t *testing.T) {
	// Missing start state
	d1 := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: ""}
	if err := d1.Validate(); err == nil {
		t.Fatal("expected error for empty start")
	}
	d1b := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "bad"}
	if err := d1b.Validate(); err == nil {
		t.Fatal("expected error for undeclared start")
	}

	// Undeclared accept state
	d2 := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Accept: map[string]bool{"bad": true}}
	if err := d2.Validate(); err == nil {
		t.Fatal("expected error for undeclared accept")
	}

	// Undeclared transition source
	d3 := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string]string{"bad": {"a": "q0"}}}
	if err := d3.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition source")
	}

	// Undeclared alphabet symbol
	d4 := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string]string{"q0": {"bad": "q0"}}}
	if err := d4.Validate(); err == nil {
		t.Fatal("expected error for undeclared alphabet symbol")
	}

	// Undeclared transition target
	d5 := DFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string]string{"q0": {"a": "bad"}}}
	if err := d5.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition target")
	}
}

func TestDFACompleteAndSinkCollision(t *testing.T) {
	// DFA that already has sink state name declared
	d := DFA{
		States:      []string{"q0", "__sink__"},
		Alphabet:    []string{"a"},
		Start:       "q0",
		Accept:      map[string]bool{"q0": true},
		Transitions: map[string]map[string]string{"q0": {"a": "q0"}},
	}
	comp := d.Complete()
	if len(comp.States) < 3 {
		t.Fatalf("expected sink collision to generate unique sink: %v", comp.States)
	}

	// Complete DFA with no missing transitions (hasMissing == false)
	comp2 := comp.Complete()
	if len(comp2.States) != len(comp.States) {
		t.Fatalf("expected already complete DFA to not add states")
	}
}

func TestDFAMinimizeEdgeCases(t *testing.T) {
	// DFA with unreachable states and all states non-accepting (len(accept) == 0)
	dNoAccept := DFA{
		States:      []string{"q0", "qUnreachable"},
		Alphabet:    []string{"a"},
		Start:       "q0",
		Accept:      map[string]bool{},
		Transitions: map[string]map[string]string{"q0": {"a": "q0"}, "qUnreachable": {"a": "qUnreachable"}},
	}
	minNoAccept := dNoAccept.Minimize()
	if len(minNoAccept.States) == 0 {
		t.Fatal("expected states in minimized DFA")
	}

	// DFA with all states accepting (len(other) == 0 before complete, though complete adds rejecting sink)
	dAllAccept := DFA{
		States:      []string{"q0"},
		Alphabet:    []string{"a"},
		Start:       "q0",
		Accept:      map[string]bool{"q0": true},
		Transitions: map[string]map[string]string{"q0": {"a": "q0"}},
	}
	minAllAccept := dAllAccept.Minimize()
	if !minAllAccept.Run([]string{"a"}) {
		t.Fatal("expected all accept DFA to accept")
	}
}

func TestNFAValidationErrors(t *testing.T) {
	// Empty or undeclared start
	n1 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: ""}
	if err := n1.Validate(); err == nil {
		t.Fatal("expected error for empty start")
	}
	n1b := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "bad"}
	if err := n1b.Validate(); err == nil {
		t.Fatal("expected error for undeclared start")
	}

	// Undeclared accept
	n2 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Accept: map[string]bool{"bad": true}}
	if err := n2.Validate(); err == nil {
		t.Fatal("expected error for undeclared accept")
	}

	// Undeclared transition source
	n3 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string][]string{"bad": {"a": {"q0"}}}}
	if err := n3.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition source")
	}

	// Undeclared symbol
	n4 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string][]string{"q0": {"bad": {"q0"}}}}
	if err := n4.Validate(); err == nil {
		t.Fatal("expected error for undeclared symbol")
	}

	// Undeclared transition target
	n5 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Transitions: map[string]map[string][]string{"q0": {"a": {"bad"}}}}
	if err := n5.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition target")
	}

	// Undeclared epsilon source
	n6 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Epsilon: map[string][]string{"bad": {"q0"}}}
	if err := n6.Validate(); err == nil {
		t.Fatal("expected error for undeclared epsilon source")
	}

	// Undeclared epsilon target
	n7 := NFA{States: []string{"q0"}, Alphabet: []string{"a"}, Start: "q0", Epsilon: map[string][]string{"q0": {"bad"}}}
	if err := n7.Validate(); err == nil {
		t.Fatal("expected error for undeclared epsilon target")
	}

	// Valid NFA
	nValid := NFA{
		States:      []string{"q0"},
		Alphabet:    []string{"a"},
		Start:       "q0",
		Accept:      map[string]bool{"q0": true},
		Transitions: map[string]map[string][]string{"q0": {"a": {"q0"}}},
		Epsilon:     map[string][]string{"q0": {"q0"}},
	}
	if err := nValid.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
