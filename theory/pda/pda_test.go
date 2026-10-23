package pda

import "testing"

func TestPDAStepLimitIsReported(t *testing.T) {
	a := Automaton{
		States:             []string{"q"},
		Start:              "q",
		InitialStack:       "Z",
		AcceptByFinalState: true,
		Transitions: map[TransitionKey][]Transition{
			{"q", "a", "Z"}: {{"q", []string{"Z"}}},
		},
	}
	r := a.Run([]string{"a"}, 1)
	if !r.TimedOut {
		t.Fatalf("expected timeout, result=%+v", r)
	}
}

func TestBalancedPDA(t *testing.T) {
	a := Automaton{
		States:             []string{"q"},
		Start:              "q",
		InitialStack:       "Z",
		AcceptByEmptyStack: true,
		Transitions: map[TransitionKey][]Transition{
			{"q", "(", "Z"}: {{"q", []string{"Z", "("}}},
			{"q", "(", "("}: {{"q", []string{"(", "("}}},
			{"q", ")", "("}: {{"q", nil}},
			{"q", "", "Z"}:  {{"q", nil}},
		},
	}
	if !a.Run([]string{"(", "(", ")", ")"}, 10).Accepted {
		t.Fatal("balanced input rejected")
	}
	if a.Run([]string{"(", ")", ")"}, 10).Accepted {
		t.Fatal("unbalanced input accepted")
	}
}

func TestPDAValidationErrors(t *testing.T) {
	// Undeclared start state
	a1 := Automaton{States: []string{"q0"}, Start: "bad"}
	if err := a1.Validate(); err == nil {
		t.Fatal("expected error for undeclared start")
	}

	// Undeclared accept state
	a2 := Automaton{States: []string{"q0"}, Start: "q0", AcceptStates: map[string]bool{"bad": true}}
	if err := a2.Validate(); err == nil {
		t.Fatal("expected error for undeclared accept")
	}

	// Invalid transition key (undeclared state or empty stack top)
	a3 := Automaton{States: []string{"q0"}, Start: "q0", Transitions: map[TransitionKey][]Transition{
		{"bad", "a", "Z"}: {{"q0", nil}},
	}}
	if err := a3.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition state")
	}
	a3b := Automaton{States: []string{"q0"}, Start: "q0", Transitions: map[TransitionKey][]Transition{
		{"q0", "a", ""}: {{"q0", nil}},
	}}
	if err := a3b.Validate(); err == nil {
		t.Fatal("expected error for empty stack top")
	}

	// Undeclared transition target
	a4 := Automaton{States: []string{"q0"}, Start: "q0", Transitions: map[TransitionKey][]Transition{
		{"q0", "a", "Z"}: {{"bad", nil}},
	}}
	if err := a4.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition target")
	}

	// Valid PDA
	aValid := Automaton{
		States:             []string{"q0"},
		Start:              "q0",
		InitialStack:       "Z",
		AcceptStates:       map[string]bool{"q0": true},
		AcceptByFinalState: true,
		Transitions: map[TransitionKey][]Transition{
			{"q0", "a", "Z"}: {{"q0", []string{"Z"}}},
		},
	}
	if err := aValid.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestPDARunAndAcceptEdgeCases(t *testing.T) {
	// Accept by final state with trace
	aFinal := Automaton{
		States:             []string{"q0", "q1"},
		Start:              "q0",
		InitialStack:       "Z",
		AcceptStates:       map[string]bool{"q1": true},
		AcceptByFinalState: true,
		Transitions: map[TransitionKey][]Transition{
			{"q0", "a", "Z"}: {{"q1", []string{"Z"}}},
		},
	}
	resFinal := aFinal.Run([]string{"a"}, 5)
	if !resFinal.Accepted || len(resFinal.Trace) != 2 {
		t.Fatalf("expected accepted final state with 2 trace entries: %+v", resFinal)
	}

	// Input rejected without timeout (queue empty, not accepted)
	resReject := aFinal.Run([]string{"b"}, 5)
	if resReject.Accepted || resReject.TimedOut {
		t.Fatalf("expected reject without timeout: %+v", resReject)
	}

	// Position != inputLength in accepts()
	aPremature := Automaton{
		States:             []string{"q0"},
		Start:              "q0",
		InitialStack:       "Z",
		AcceptStates:       map[string]bool{"q0": true},
		AcceptByFinalState: true,
	}
	// Initial configuration has pos=0 != len(["a"])
	if aPremature.accepts(Configuration{State: "q0", Position: 0, Stack: []string{"Z"}}, 1) {
		t.Fatal("should not accept when position != inputLength")
	}

	// Transitions when stack becomes empty
	aEmptyStack := Automaton{
		States: []string{"q0"},
		Start:  "q0",
		Transitions: map[TransitionKey][]Transition{
			{"q0", "a", ""}: {{"q0", nil}}, // transition on empty stack
		},
	}
	resEmpty := aEmptyStack.Run([]string{"a"}, 5)
	if resEmpty.Accepted {
		t.Fatal("expected reject on empty stack")
	}
}
