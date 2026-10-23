package tm

import "testing"

func TestTMIncrementUnary(t *testing.T) {
	m := Machine{
		States:       []string{"scan", "accept"},
		TapeAlphabet: []string{"1", "_"},
		Blank:        "_",
		Start:        "scan",
		AcceptStates: map[string]bool{"accept": true},
		Transitions: map[TransitionKey]Transition{
			{"scan", "1"}: {"scan", "1", Right},
			{"scan", "_"}: {"accept", "1", Stay},
		},
	}
	r := m.Run([]string{"1", "1"}, 10, true)
	if !r.Accepted || r.Configuration.Tape[2] != "1" || len(r.Trace) != 4 {
		t.Fatalf("result=%+v", r)
	}
}

func TestTMStepLimit(t *testing.T) {
	m := Machine{
		States:       []string{"q"},
		TapeAlphabet: []string{"0", "_"},
		Blank:        "_",
		Start:        "q",
		Transitions: map[TransitionKey]Transition{
			{"q", "_"}: {"q", "_", Right},
		},
	}
	if !m.Run(nil, 3, false).TimedOut {
		t.Fatal("expected timeout")
	}
}

func TestTMAcceptsOnLastAllowedTransition(t *testing.T) {
	m := Machine{
		States:       []string{"scan", "accept"},
		TapeAlphabet: []string{"1", "_"},
		Blank:        "_",
		Start:        "scan",
		AcceptStates: map[string]bool{"accept": true},
		Transitions: map[TransitionKey]Transition{
			{"scan", "1"}: {"accept", "1", Stay},
		},
	}
	r := m.Run([]string{"1"}, 1, true)
	if !r.Accepted || r.TimedOut {
		t.Fatalf("result=%+v", r)
	}
}

func TestTMValidationErrors(t *testing.T) {
	// Undeclared start
	m1 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"_"}, Blank: "_", Start: "bad"}
	if err := m1.Validate(); err == nil {
		t.Fatal("expected error for undeclared start")
	}

	// Blank not in tape alphabet
	m2 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"a"}, Blank: "_", Start: "q0"}
	if err := m2.Validate(); err == nil {
		t.Fatal("expected error for blank not in tape alphabet")
	}

	// Undeclared accept state
	m3 := Machine{States: []string{"q0"}, TapeAlphabet: []string{"_"}, Blank: "_", Start: "q0", AcceptStates: map[string]bool{"bad": true}}
	if err := m3.Validate(); err == nil {
		t.Fatal("expected error for undeclared accept state")
	}

	// Invalid transitions: state, read, write, target
	m4a := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{{"bad", "_"}: {"q0", "_", Stay}},
	}
	if err := m4a.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition state")
	}

	m4b := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{{"q0", "bad"}: {"q0", "_", Stay}},
	}
	if err := m4b.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition read")
	}

	m4c := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{{"q0", "_"}: {"q0", "bad", Stay}},
	}
	if err := m4c.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition write")
	}

	m4d := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{{"q0", "_"}: {"bad", "_", Stay}},
	}
	if err := m4d.Validate(); err == nil {
		t.Fatal("expected error for undeclared transition target")
	}

	// Valid machine
	mValid := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{{"q0", "_"}: {"q0", "_", Stay}},
	}
	if err := mValid.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestTMRunEdgeCases(t *testing.T) {
	// Symbol not in tape alphabet
	m := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"_"},
		Blank:        "_",
		Start:        "q0",
	}
	rErr := m.Run([]string{"bad"}, 5, false)
	if rErr.Err == nil {
		t.Fatal("expected error for symbol not in tape alphabet")
	}

	// Reject states hit during loop
	mReject := Machine{
		States:       []string{"q0", "qr"},
		TapeAlphabet: []string{"0", "_"},
		Blank:        "_",
		Start:        "q0",
		RejectStates: map[string]bool{"qr": true},
		Transitions: map[TransitionKey]Transition{
			{"q0", "0"}: {"qr", "0", Stay},
		},
	}
	rRej := mReject.Run([]string{"0"}, 5, true)
	if !rRej.Rejected || !rRej.Halted {
		t.Fatalf("expected reject on qr state: %+v", rRej)
	}

	// Missing transition halts
	mMissing := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"0", "_"},
		Blank:        "_",
		Start:        "q0",
		Transitions:  map[TransitionKey]Transition{},
	}
	rMiss := mMissing.Run([]string{"0"}, 5, false)
	if !rMiss.Halted || rMiss.Accepted || rMiss.Rejected {
		t.Fatalf("expected non-accepting halt on missing transition: %+v", rMiss)
	}

	// Erasing to blank (tr.Write == m.Blank) and moving left (c.Head < c.MinIndex)
	mEraseAndLeft := Machine{
		States:       []string{"q0", "q1", "qAccept"},
		TapeAlphabet: []string{"0", "_"},
		Blank:        "_",
		Start:        "q0",
		AcceptStates: map[string]bool{"qAccept": true},
		Transitions: map[TransitionKey]Transition{
			{"q0", "0"}: {"q1", "_", Left},
			{"q1", "_"}: {"qAccept", "_", Stay},
		},
	}
	rErase := mEraseAndLeft.Run([]string{"0"}, 5, true)
	if !rErase.Accepted || rErase.Configuration.MinIndex != -1 || rErase.Configuration.Tape[0] != "" {
		t.Fatalf("expected erased cell and minIndex -1: %+v", rErase)
	}

	// Reject state at maxSteps limit
	mRejMax := Machine{
		States:       []string{"q0"},
		TapeAlphabet: []string{"0", "_"},
		Blank:        "_",
		Start:        "q0",
		RejectStates: map[string]bool{"q0": true},
	}
	rRejMax := mRejMax.Run([]string{"0"}, 0, false)
	if !rRejMax.Rejected || !rRejMax.Halted {
		t.Fatalf("expected rejected at maxSteps 0: %+v", rRejMax)
	}
}
