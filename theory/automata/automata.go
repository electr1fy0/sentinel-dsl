// Package automata contains small, deterministic implementations of finite
// automata algorithms. Symbols are strings so callers can model either bytes,
// tokens, or Unicode graphemes without changing the algorithms.
package automata

import (
	"fmt"
	"sort"
	"strings"
)

// DFA is a deterministic finite automaton. Missing transitions reject.
type DFA struct {
	States      []string
	Alphabet    []string
	Start       string
	Accept      map[string]bool
	Transitions map[string]map[string]string
}

// Validate checks state, alphabet, and transition references.
func (d DFA) Validate() error {
	states, alphabet := stringSet(d.States), stringSet(d.Alphabet)
	if d.Start == "" || !states[d.Start] { return fmt.Errorf("start state %q is not declared", d.Start) }
	for state := range d.Accept { if !states[state] { return fmt.Errorf("accept state %q is not declared", state) } }
	for from, row := range d.Transitions {
		if !states[from] { return fmt.Errorf("transition source %q is not declared", from) }
		for symbol, to := range row {
			if !alphabet[symbol] { return fmt.Errorf("symbol %q is not in alphabet", symbol) }
			if !states[to] { return fmt.Errorf("transition target %q is not declared", to) }
		}
	}
	return nil
}

// Run consumes the complete input and reports whether the final state accepts.
func (d DFA) Run(input []string) bool {
	state := d.Start
	for _, symbol := range input {
		row := d.Transitions[state]
		next, ok := row[symbol]
		if !ok { return false }
		state = next
	}
	return d.Accept[state]
}

// Trace returns the states visited, including the start and final states. It
// stops at a missing transition and returns an error in that case.
func (d DFA) Trace(input []string) ([]string, error) {
	state := d.Start
	trace := []string{state}
	for _, symbol := range input {
		next, ok := d.Transitions[state][symbol]
		if !ok { return trace, fmt.Errorf("no transition from %q on %q", state, symbol) }
		state, trace = next, append(trace, next)
	}
	return trace, nil
}

// Complete returns an equivalent DFA with a rejecting sink for missing moves.
func (d DFA) Complete() DFA {
	out := cloneDFA(d)
	sink := "__sink__"
	used := stringSet(out.States)
	for used[sink] { sink += "_" }
	hasMissing := false
	for _, state := range out.States {
		if out.Transitions[state] == nil { out.Transitions[state] = map[string]string{} }
		for _, symbol := range out.Alphabet {
			if _, ok := out.Transitions[state][symbol]; !ok { out.Transitions[state][symbol], hasMissing = sink, true }
		}
	}
	if hasMissing {
		out.States = append(out.States, sink)
		out.Transitions[sink] = map[string]string{}
		for _, symbol := range out.Alphabet { out.Transitions[sink][symbol] = sink }
	}
	return out
}

// NFA is a nondeterministic finite automaton. The empty string denotes an
// epsilon edge in Epsilon, not an input symbol.
type NFA struct {
	States      []string
	Alphabet    []string
	Start       string
	Accept      map[string]bool
	Transitions map[string]map[string][]string
	Epsilon     map[string][]string
}

// Validate checks the NFA declaration.
func (n NFA) Validate() error {
	states, alphabet := stringSet(n.States), stringSet(n.Alphabet)
	if n.Start == "" || !states[n.Start] { return fmt.Errorf("start state %q is not declared", n.Start) }
	for state := range n.Accept { if !states[state] { return fmt.Errorf("accept state %q is not declared", state) } }
	for from, row := range n.Transitions {
		if !states[from] { return fmt.Errorf("transition source %q is not declared", from) }
		for symbol, targets := range row {
			if !alphabet[symbol] { return fmt.Errorf("symbol %q is not in alphabet", symbol) }
			for _, to := range targets { if !states[to] { return fmt.Errorf("transition target %q is not declared", to) } }
		}
	}
	for from, targets := range n.Epsilon {
		if !states[from] { return fmt.Errorf("epsilon source %q is not declared", from) }
		for _, to := range targets { if !states[to] { return fmt.Errorf("epsilon target %q is not declared", to) } }
	}
	return nil
}

// EpsilonClosure computes the reflexive-transitive closure of states.
func (n NFA) EpsilonClosure(states map[string]bool) map[string]bool {
	closure := cloneSet(states)
	queue := make([]string, 0, len(states))
	for state := range states { queue = append(queue, state) }
	for len(queue) > 0 {
		state := queue[0]; queue = queue[1:]
		for _, next := range n.Epsilon[state] {
			if !closure[next] { closure[next] = true; queue = append(queue, next) }
		}
	}
	return closure
}

// Move follows one input symbol and then applies epsilon closure.
func (n NFA) Move(states map[string]bool, symbol string) map[string]bool {
	next := map[string]bool{}
	for state := range n.EpsilonClosure(states) {
		for _, target := range n.Transitions[state][symbol] { next[target] = true }
	}
	return n.EpsilonClosure(next)
}

// Run consumes the complete input using epsilon transitions as needed.
func (n NFA) Run(input []string) bool {
	current := n.EpsilonClosure(map[string]bool{n.Start: true})
	for _, symbol := range input { current = n.Move(current, symbol); if len(current) == 0 { return false } }
	for state := range current { if n.Accept[state] { return true } }
	return false
}

// SubsetConstruction converts an NFA to a DFA. DFA state names are canonical
// sets such as "{q0,q1}"; the empty set is "{}".
func (n NFA) SubsetConstruction() DFA {
	startSet := n.EpsilonClosure(map[string]bool{n.Start: true})
	start := setName(startSet)
	d := DFA{Alphabet: append([]string(nil), n.Alphabet...), Start: start, Accept: map[string]bool{}, Transitions: map[string]map[string]string{}}
	seen := map[string]bool{start: true}; sets := map[string]map[string]bool{start: startSet}; queue := []string{start}
	for len(queue) > 0 {
		name := queue[0]; queue = queue[1:]
		d.States = append(d.States, name); d.Transitions[name] = map[string]string{}
		for state := range sets[name] { if n.Accept[state] { d.Accept[name] = true; break } }
		for _, symbol := range n.Alphabet {
			next := n.Move(sets[name], symbol); nextName := setName(next); d.Transitions[name][symbol] = nextName
			if !seen[nextName] { seen[nextName] = true; sets[nextName] = next; queue = append(queue, nextName) }
		}
	}
	return d
}

// Minimize returns a language-equivalent DFA with unreachable states removed.
// It uses partition refinement and preserves the original transition alphabet.
func (d DFA) Minimize() DFA {
	d = d.Complete()
	reachable := map[string]bool{d.Start: true}; queue := []string{d.Start}
	for len(queue) > 0 { state := queue[0]; queue = queue[1:]; for _, next := range d.Transitions[state] { if !reachable[next] { reachable[next] = true; queue = append(queue, next) } } }
	accept, other := []string{}, []string{}
	for _, state := range d.States { if !reachable[state] { continue }; if d.Accept[state] { accept = append(accept, state) } else { other = append(other, state) } }
	parts := [][]string{}; if len(accept)>0 { parts=append(parts,accept) }; if len(other)>0 { parts=append(parts,other) }
	for changed := true; changed; {
		changed = false; partOf := map[string]int{}; for i, part := range parts { for _, state := range part { partOf[state]=i } }
		newParts := make([][]string, 0, len(parts))
		for _, part := range parts {
			groups := map[string][]string{}; order := []string{}
			for _, state := range part { var b strings.Builder; for _, symbol := range d.Alphabet { b.WriteString(fmt.Sprintf("%d,",partOf[d.Transitions[state][symbol]])) }; key:=b.String(); if _,ok:=groups[key]; !ok { order=append(order,key) }; groups[key]=append(groups[key],state) }
			if len(order)>1 { changed=true }; for _, key := range order { newParts=append(newParts,groups[key]) }
		}
		parts = newParts
	}
	partOf := map[string]int{}; for i, part := range parts { for _, state := range part { partOf[state]=i } }
	name := func(i int) string { members:=append([]string(nil),parts[i]...); sort.Strings(members); return "{"+strings.Join(members,"|")+"}" }
	out := DFA{Alphabet:append([]string(nil),d.Alphabet...),Accept:map[string]bool{},Transitions:map[string]map[string]string{}}
	for i, part := range parts { nn:=name(i); out.States=append(out.States,nn); if i==partOf[d.Start] { out.Start=nn }; if d.Accept[part[0]] { out.Accept[nn]=true }; out.Transitions[nn]=map[string]string{}; for _,symbol:=range d.Alphabet { out.Transitions[nn][symbol]=name(partOf[d.Transitions[part[0]][symbol]]) } }
	return out
}

func stringSet(items []string) map[string]bool { out:=map[string]bool{}; for _,item:=range items { out[item]=true }; return out }
func cloneSet(in map[string]bool) map[string]bool { out:=map[string]bool{}; for k,v:=range in { out[k]=v }; return out }
func setName(states map[string]bool) string { items:=make([]string,0,len(states)); for state:=range states { items=append(items,state) }; sort.Strings(items); return "{"+strings.Join(items,",")+"}" }
func cloneDFA(in DFA) DFA { out:=in; out.States=append([]string(nil),in.States...); out.Alphabet=append([]string(nil),in.Alphabet...); out.Accept=cloneSet(in.Accept); out.Transitions=map[string]map[string]string{}; for s,row:=range in.Transitions { out.Transitions[s]=map[string]string{}; for a,t:=range row { out.Transitions[s][a]=t } }; return out }
