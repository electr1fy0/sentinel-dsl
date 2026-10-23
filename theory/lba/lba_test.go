package lba

import "testing"

func TestLBATogglesAndAccepts(t *testing.T) {
	m := Machine{
		States:       []string{"q", "accept"},
		TapeAlphabet: []string{"0", "1"},
		Blank:        "0",
		Start:        "q",
		AcceptStates: map[string]bool{"accept": true},
		Transitions:  map[TransitionKey]Transition{{"q", "1"}: {"accept", "1", Stay}},
	}
	r := m.Run([]string{"1"}, 2)
	if !r.Accepted || r.Steps != 1 {
		t.Fatalf("result=%+v", r)
	}
}

func TestLBAIsBounded(t *testing.T) {
	m := Machine{
		States:       []string{"q"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q",
		Transitions:  map[TransitionKey]Transition{{"q", "0"}: {"q", "0", Left}},
	}
	r := m.Run([]string{"0"}, 2)
	if r.Err == nil {
		t.Fatal("expected boundary error")
	}

	// Move right beyond tape length
	mRight := Machine{
		States:       []string{"q"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q",
		Transitions:  map[TransitionKey]Transition{{"q", "0"}: {"q", "0", Right}},
	}
	rRight := mRight.Run([]string{"0"}, 2)
	if rRight.Err == nil {
		t.Fatal("expected right boundary error")
	}
}

func TestLBAAcceptsOnLastAllowedTransition(t *testing.T) {
	m := Machine{
		States:       []string{"q", "accept"},
		TapeAlphabet: []string{"0", "1"},
		Blank:        "0",
		Start:        "q",
		AcceptStates: map[string]bool{"accept": true},
		Transitions:  map[TransitionKey]Transition{{"q", "1"}: {"accept", "1", Stay}},
	}
	r := m.Run([]string{"1"}, 1)
	if !r.Accepted || r.TimedOut {
		t.Fatalf("result=%+v", r)
	}
}

func TestLBAValidationErrors(t *testing.T) {
	// Undeclared start
	m1 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"_"}, Blank: "_", Start: "bad"}
	if err := m1.Validate(); err == nil {
		t.Fatal("expected error for undeclared start")
	}

	// Blank not in tape alphabet
	m2 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"a"}, Blank: "_", Start: "q0"}
	if err := m2.Validate(); err == nil {
		t.Fatal("expected error for blank not in alphabet")
	}

	// Undeclared accept state
	m3 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"_"}, Blank: "_", Start: "q0", AcceptStates: map[string]bool{"bad": true}}
	if err := m3.Validate(); err == nil {
		t.Fatal("expected error for undeclared accept state")
	}

	// Invalid transition: undeclared state / tape symbol
	m4 := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions: map[TransitionKey]Transition{
			{"bad", "_"}: {"q0", "_", Stay},
		},
	}
	if err := m4.Validate(); err == nil {
		t.Fatal("expected error for invalid transition state")
	}
	m4b := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions: map[TransitionKey]Transition{
			{"q0", "bad"}: {"q0", "_", Stay},
		},
	}
	if err := m4b.Validate(); err == nil {
		t.Fatal("expected error for invalid transition read")
	}
	m4c := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions: map[TransitionKey]Transition{
			{"q0", "_"}: {"q0", "bad", Stay},
		},
	}
	if err := m4c.Validate(); err == nil {
		t.Fatal("expected error for invalid transition write")
	}
	m4d := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions: map[TransitionKey]Transition{
			{"q0", "_"}: {"bad", "_", Stay},
		},
	}
	if err := m4d.Validate(); err == nil {
		t.Fatal("expected error for invalid transition target")
	}

	// Valid machine
	mValid := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		AcceptStates: map[string]bool{"q0": true},
		Transitions: map[TransitionKey]Transition{
			{"q0", "_"}: {"q0", "_", Stay},
		},
	}
	if err := mValid.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestLBARunEdgeCases(t *testing.T) {
	// Empty input (sets tape = [m.Blank])
	mEmpty := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		AcceptStates: map[string]bool{"q0": true},
	}
	rEmpty := mEmpty.Run([]string{}, 5)
	if !rEmpty.Accepted || len(rEmpty.Configuration.Tape) != 1 || rEmpty.Configuration.Tape[0] != "_" {
		t.Fatalf("unexpected result on empty input: %+v", rEmpty)
	}

	// Symbol not in tape alphabet
	mInvalidSymbol := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
	}
	rInvalid := mInvalidSymbol.Run([]string{"bad_symbol"}, 5)
	if rInvalid.Err == nil {
		t.Fatal("expected error for symbol not in tape alphabet")
	}

	// Reject states hit during loop
	mReject := Machine{
		States:       []string{"q0", "qr"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q0",
		RejectStates: map[string]bool{"qr": true},
		Transitions: map[TransitionKey]Transition{
			{"q0", "0"}: {"qr", "0", Stay},
		},
	}
	rRej := mReject.Run([]string{"0"}, 5)
	if !rRej.Rejected || !rRej.Halted {
		t.Fatalf("expected reject on qr state: %+v", rRej)
	}

	// Missing transition halts
	mMissing := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{},
	}
	rMiss := mMissing.Run([]string{"0"}, 5)
	if !rMiss.Halted || rMiss.Accepted || rMiss.Rejected {
		t.Fatalf("expected non-accepting halt on missing transition: %+v", rMiss)
	}

	// Timeout (steps == maxSteps without halt)
	mLoop := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q0",
		Transitions: map[TransitionKey]Transition{
			{"q0", "0"}: {"q0", "0", Stay},
		},
	}
	rTimeout := mLoop.Run([]string{"0"}, 3)
	if !rTimeout.TimedOut {
		t.Fatalf("expected timed out result: %+v", rTimeout)
	}

	// Reject state at maxSteps
	mRejMax := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"0"},
		Blank:        "0",
		Start:        "q0",
		RejectStates: map[string]bool{"q0": true},
	}
	rRejMax := mRejMax.Run([]string{"0"}, 0)
	if !rRejMax.Rejected || !rRejMax.Halted {
		t.Fatalf("expected rejected at maxSteps 0: %+v", rRejMax)
	}
}
