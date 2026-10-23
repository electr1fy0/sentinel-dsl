// Package tm implements a deterministic single-tape Turing-machine simulator
// with explicit step bounds and optional complete configuration traces.
package tm

import "fmt"

// Move is a tape-head movement.
type Move int

const (
	Left  Move = -1
	Stay  Move = 0
	Right Move = 1
)

// TransitionKey selects a transition.
type TransitionKey struct{ State, Read string }

// Transition is the action of a transition.
type Transition struct {
	ToState, Write string
	Move           Move
}

// Machine is a deterministic Turing machine. Tape is conceptually infinite in
// both directions and unvisited cells contain Blank.
type Machine struct {
	States                     []string
	Alphabet, TapeAlphabet     []string
	Blank, Start               string
	AcceptStates, RejectStates map[string]bool
	Transitions                map[TransitionKey]Transition
}

// Configuration is a snapshot. Tape stores only the finite nonblank window
// from MinIndex through MaxIndex, with Head using absolute tape coordinates.
type Configuration struct {
	State              string
	Head               int
	Tape               map[int]string
	MinIndex, MaxIndex int
}

// Result describes a run and includes the complete trace when requested.
type Result struct {
	Accepted, Rejected, Halted, TimedOut bool
	Steps                                int
	Configuration                        Configuration
	Trace                                []Configuration
	Err                                  error
}

// Validate checks state and alphabet references.
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

// Run executes up to maxSteps transitions. includeTrace controls whether all
// configurations are copied into Result.Trace; the final configuration is
// always returned.
func (m Machine) Run(input []string, maxSteps int, includeTrace bool) Result {
	tape := map[int]string{}
	alpha := set(m.TapeAlphabet)
	for i, symbol := range input {
		if !alpha[symbol] {
			return Result{Err: fmt.Errorf("input symbol %q is not in tape alphabet", symbol)}
		}
		if symbol != m.Blank {
			tape[i] = symbol
		}
	}
	max := len(input) - 1
	if max < 0 {
		max = 0
	}
	c := Configuration{State: m.Start, Head: 0, Tape: tape, MinIndex: 0, MaxIndex: max}
	trace := []Configuration{}
	if includeTrace {
		trace = append(trace, clone(c))
	}
	for steps := 0; steps < maxSteps; steps++ {
		if m.AcceptStates[c.State] {
			return finish(Result{Accepted: true, Halted: true, Steps: steps}, c, trace)
		}
		if m.RejectStates[c.State] {
			return finish(Result{Rejected: true, Halted: true, Steps: steps}, c, trace)
		}
		read := c.Tape[c.Head]
		if read == "" {
			read = m.Blank
		}
		tr, ok := m.Transitions[TransitionKey{c.State, read}]
		if !ok {
			return finish(Result{Accepted: m.AcceptStates[c.State], Rejected: m.RejectStates[c.State], Halted: true, Steps: steps}, c, trace)
		}
		if tr.Write == m.Blank {
			delete(c.Tape, c.Head)
		} else {
			c.Tape[c.Head] = tr.Write
		}
		c.State = tr.ToState
		c.Head += int(tr.Move)
		if c.Head < c.MinIndex {
			c.MinIndex = c.Head
		}
		if c.Head > c.MaxIndex {
			c.MaxIndex = c.Head
		}
		if includeTrace {
			trace = append(trace, clone(c))
		}
	}
	if m.AcceptStates[c.State] {
		return finish(Result{Accepted: true, Halted: true, Steps: maxSteps}, c, trace)
	}
	if m.RejectStates[c.State] {
		return finish(Result{Rejected: true, Halted: true, Steps: maxSteps}, c, trace)
	}
	return finish(Result{TimedOut: true, Steps: maxSteps}, c, trace)
}
func finish(r Result, c Configuration, trace []Configuration) Result {
	r.Configuration = clone(c)
	r.Trace = trace
	return r
}
func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}
func clone(c Configuration) Configuration {
	t := map[int]string{}
	for i, s := range c.Tape {
		t[i] = s
	}
	return Configuration{c.State, c.Head, t, c.MinIndex, c.MaxIndex}
}
