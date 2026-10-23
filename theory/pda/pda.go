// Package pda implements bounded nondeterministic pushdown-automaton
// simulation. The empty string denotes an epsilon input or a no-symbol push.
package pda

import "fmt"

// TransitionKey selects a transition by current state, input symbol, and top
// stack symbol. Use Input == "" for epsilon input.
type TransitionKey struct{ State, Input, StackTop string }

// Transition is a move to a state that replaces StackTop with Push. Push is
// listed bottom-to-top: Push{"A","B"} leaves B on top.
type Transition struct {
	ToState string
	Push    []string
}

// Automaton is a PDA with acceptance by final state and/or empty stack.
type Automaton struct {
	States                                 []string
	InputAlphabet, StackAlphabet           []string
	Start, InitialStack                    string
	AcceptStates                           map[string]bool
	AcceptByFinalState, AcceptByEmptyStack bool
	Transitions                            map[TransitionKey][]Transition
}

// Configuration is an observable PDA configuration.
type Configuration struct {
	State    string
	Position int
	Stack    []string
}

// Result records one accepting computation and its configuration trace.
type Result struct {
	Accepted bool
	TimedOut bool
	Steps    int
	Trace    []Configuration
}

// Validate checks references in the PDA declaration.
func (a Automaton) Validate() error {
	states := set(a.States)
	if !states[a.Start] {
		return fmt.Errorf("start state %q is not declared", a.Start)
	}
	for s := range a.AcceptStates {
		if !states[s] {
			return fmt.Errorf("accept state %q is not declared", s)
		}
	}
	for k, ts := range a.Transitions {
		if !states[k.State] || k.StackTop == "" {
			return fmt.Errorf("invalid transition key %#v", k)
		}
		for _, tr := range ts {
			if !states[tr.ToState] {
				return fmt.Errorf("transition target %q is not declared", tr.ToState)
			}
		}
	}
	return nil
}

// Run searches configurations breadth-first up to maxSteps. It is complete
// for all accepting runs within that bound and avoids revisiting configurations.
func (a Automaton) Run(input []string, maxSteps int) Result {
	start := Configuration{State: a.Start, Position: 0, Stack: []string{a.InitialStack}}
	queue := []node{{cfg: start, steps: 0, trace: []Configuration{start}}}
	seen := map[string]bool{key(start): true}
	timedOut := false
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if a.accepts(cur.cfg, len(input)) {
			return Result{Accepted: true, Steps: cur.steps, Trace: cur.trace}
		}
		if cur.steps >= maxSteps {
			timedOut = true
			continue
		}
		top := ""
		if len(cur.cfg.Stack) > 0 {
			top = cur.cfg.Stack[len(cur.cfg.Stack)-1]
		}
		for k, trs := range a.Transitions {
			if k.State != cur.cfg.State || k.StackTop != top {
				continue
			}
			if k.Input != "" && (cur.cfg.Position >= len(input) || input[cur.cfg.Position] != k.Input) {
				continue
			}
			pos := cur.cfg.Position
			if k.Input != "" {
				pos++
			}
			for _, tr := range trs {
				stack := append([]string(nil), cur.cfg.Stack[:max(0, len(cur.cfg.Stack)-1)]...)
				stack = append(stack, tr.Push...)
				next := Configuration{tr.ToState, pos, stack}
				if !seen[key(next)] {
					seen[key(next)] = true
					trace := append(append([]Configuration(nil), cur.trace...), next)
					queue = append(queue, node{next, cur.steps + 1, trace})
				}
			}
		}
	}
	return Result{TimedOut: timedOut}
}

type node struct {
	cfg   Configuration
	steps int
	trace []Configuration
}

func (a Automaton) accepts(c Configuration, inputLength int) bool {
	if c.Position != inputLength {
		return false
	}
	return (a.AcceptByFinalState && a.AcceptStates[c.State]) || (a.AcceptByEmptyStack && len(c.Stack) == 0)
}
func key(c Configuration) string {
	top := ""
	for _, s := range c.Stack {
		top += "|" + s
	}
	return fmt.Sprintf("%s/%d%s", c.State, c.Position, top)
}
func set(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}
