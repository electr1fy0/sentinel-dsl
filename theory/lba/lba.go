// Package lba implements a deterministic linear-bounded automaton. The tape
// is exactly the input length (with a minimum of one cell), so transitions may
// never move beyond its two end markers.
package lba

import "fmt"

// Move is a head movement.
type Move int

const (
	Left  Move = -1
	Stay  Move = 0
	Right Move = 1
)

// TransitionKey identifies an LBA transition.
type TransitionKey struct{ State, Read string }

// Transition writes one symbol, moves, and enters a state.
type Transition struct {
	ToState, Write string
	Move           Move
}

// Machine is a deterministic LBA.
type Machine struct {
	States                     []string
	Alphabet, TapeAlphabet     []string
	Blank, Start               string
	AcceptStates, RejectStates map[string]bool
	Transitions                map[TransitionKey]Transition
}

// Configuration is a copyable machine snapshot.
type Configuration struct {
	State string
	Head  int
	Tape  []string
}

// Result is the outcome of a bounded run.
type Result struct {
	Accepted, Rejected, Halted, TimedOut bool
	Steps                                int
	Configuration                        Configuration
	Trace                                []Configuration
	Err                                  error
}

// Validate checks machine references and tape alphabet membership.
func (m Machine) Validate() error {
	states := set(m.States)
	alpha := set(m.TapeAlphabet)
	if !states[m.Start] {
		return fmt.Errorf("start state %q is not declared", m.Start)
	}
	if !alpha[m.Blank] {
		return fmt.Errorf("blank %q is not in tape alphabet", m.Blank)
	}
	for s := range m.AcceptStates {
		if !states[s] {
			return fmt.Errorf("accept state %q is not declared", s)
		}
	}
	for k, t := range m.Transitions {
		if !states[k.State] || !alpha[k.Read] || !alpha[t.Write] || !states[t.ToState] {
			return fmt.Errorf("invalid transition %#v -> %#v", k, t)
		}
	}
	return nil
}

// Run executes at most maxSteps transitions. Any missing transition halts and
// accepts or rejects according to the current state; otherwise it is a normal
// nonaccepting halt. Trace includes the initial and final configurations.
func (m Machine) Run(input []string, maxSteps int) Result {
	tape := append([]string(nil), input...)
	if len(tape) == 0 {
		tape = []string{m.Blank}
	}
	for _, symbol := range tape {
		if !set(m.TapeAlphabet)[symbol] {
			return Result{Err: fmt.Errorf("input symbol %q is not in tape alphabet", symbol)}
		}
	}
	c := Configuration{State: m.Start, Head: 0, Tape: tape}
	trace := []Configuration{clone(c)}
	for steps := 0; steps < maxSteps; steps++ {
		if m.AcceptStates[c.State] {
			return Result{Accepted: true, Halted: true, Steps: steps, Configuration: clone(c), Trace: trace}
		}
		if m.RejectStates[c.State] {
			return Result{Rejected: true, Halted: true, Steps: steps, Configuration: clone(c), Trace: trace}
		}
		tr, ok := m.Transitions[TransitionKey{State: c.State, Read: c.Tape[c.Head]}]
		if !ok {
			return Result{Accepted: m.AcceptStates[c.State], Rejected: m.RejectStates[c.State], Halted: true, Steps: steps, Configuration: clone(c), Trace: trace}
		}
		c.Tape[c.Head] = tr.Write
		c.State = tr.ToState
		c.Head += int(tr.Move)
		if c.Head < 0 || c.Head >= len(c.Tape) {
			return Result{Halted: true, Steps: steps + 1, Configuration: clone(c), Trace: trace, Err: fmt.Errorf("head moved outside bounded tape")}
		}
		trace = append(trace, clone(c))
	}
	if m.AcceptStates[c.State] {
		return Result{Accepted: true, Halted: true, Steps: maxSteps, Configuration: clone(c), Trace: trace}
	}
	if m.RejectStates[c.State] {
		return Result{Rejected: true, Halted: true, Steps: maxSteps, Configuration: clone(c), Trace: trace}
	}
	return Result{TimedOut: true, Steps: maxSteps, Configuration: clone(c), Trace: trace}
}
func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}
func clone(c Configuration) Configuration {
	return Configuration{c.State, c.Head, append([]string(nil), c.Tape...)}
}
