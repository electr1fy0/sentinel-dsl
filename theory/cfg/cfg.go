// Package cfg provides representations and classroom algorithms for context-
// free grammars. A symbol is a string; variables are identified by Grammar's
// Variables set and every other symbol in a RHS is treated as a terminal.
package cfg

import (
	"fmt"
	"sort"
	"strings"
)

// Epsilon is the empty RHS. It is not a terminal.
const Epsilon = ""

// Production is a context-free rule LHS -> RHS.
type Production struct {
	LHS string   `json:"lhs"`
	RHS []string `json:"rhs"`
}

// Grammar is a finite context-free grammar.
type Grammar struct {
	Variables   []string     `json:"variables"`
	Terminals   []string     `json:"terminals"`
	Start       string       `json:"start"`
	Productions []Production `json:"productions"`
}

// Validate verifies symbols and production references.
func (g Grammar) Validate() error {
	vars := set(g.Variables)
	terms := set(g.Terminals)
	if !vars[g.Start] {
		return fmt.Errorf("start variable %q is not declared", g.Start)
	}
	for _, p := range g.Productions {
		if !vars[p.LHS] {
			return fmt.Errorf("production lhs %q is not a variable", p.LHS)
		}
		for _, s := range p.RHS {
			if !vars[s] && !terms[s] {
				return fmt.Errorf("symbol %q is neither variable nor terminal", s)
			}
		}
	}
	return nil
}

// Derive applies a production to the first matching variable in a form.
func (g Grammar) Derive(form []string, p Production) ([]string, bool) {
	for i, s := range form {
		if s == p.LHS {
			out := make([]string, 0, len(form)-1+len(p.RHS))
			out = append(out, form[:i]...)
			out = append(out, p.RHS...)
			out = append(out, form[i+1:]...)
			return out, true
		}
	}
	return nil, false
}

// LeftmostDerivations returns all terminal forms reachable by at most maxSteps
// leftmost rewrites. It deduplicates forms and prevents runaway duplicate work.
func (g Grammar) LeftmostDerivations(maxSteps int) [][]string {
	if maxSteps < 0 {
		return nil
	}
	out := [][]string{}
	start := []string{g.Start}
	queue := []struct {
		form  []string
		steps int
	}{{start, 0}}
	seen := map[string]bool{formKey(start): true}
	vars := set(g.Variables)
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		hasVar := false
		for _, s := range item.form {
			if vars[s] {
				hasVar = true
				break
			}
		}
		if !hasVar {
			out = append(out, append([]string(nil), item.form...))
			continue
		}
		if item.steps == maxSteps {
			continue
		}
		index := 0
		for index < len(item.form) && !vars[item.form[index]] {
			index++
		}
		for _, p := range g.Productions {
			if p.LHS != item.form[index] {
				continue
			}
			next, _ := g.Derive(item.form, p)
			key := formKey(next)
			if !seen[key] {
				seen[key] = true
				queue = append(queue, struct {
					form  []string
					steps int
				}{next, item.steps + 1})
			}
		}
	}
	return out
}

// First computes FIRST sets. The empty string in FIRST[X] means X is nullable.
func (g Grammar) First() map[string]map[string]bool {
	first := map[string]map[string]bool{}
	for _, v := range g.Variables {
		first[v] = map[string]bool{}
	}
	changed := true
	vars := set(g.Variables)
	for changed {
		changed = false
		for _, p := range g.Productions {
			nullable := true
			if len(p.RHS) == 0 {
				if !first[p.LHS][Epsilon] {
					first[p.LHS][Epsilon] = true
					changed = true
				}
				continue
			}
			for _, sym := range p.RHS {
				if !vars[sym] {
					if !first[p.LHS][sym] {
						first[p.LHS][sym] = true
						changed = true
					}
					nullable = false
					break
				}
				for token := range first[sym] {
					if token != Epsilon && !first[p.LHS][token] {
						first[p.LHS][token] = true
						changed = true
					}
				}
				if !first[sym][Epsilon] {
					nullable = false
					break
				}
			}
			if nullable && !first[p.LHS][Epsilon] {
				first[p.LHS][Epsilon] = true
				changed = true
			}
		}
	}
	return first
}

// FirstSequence computes FIRST of a sequence using precomputed FIRST sets.
func (g Grammar) FirstSequence(sequence []string, first map[string]map[string]bool) map[string]bool {
	vars := set(g.Variables)
	out := map[string]bool{}
	nullable := true
	if len(sequence) == 0 {
		return map[string]bool{Epsilon: true}
	}
	for _, sym := range sequence {
		if !vars[sym] {
			out[sym] = true
			nullable = false
			break
		}
		for token := range first[sym] {
			if token != Epsilon {
				out[token] = true
			}
		}
		if !first[sym][Epsilon] {
			nullable = false
			break
		}
	}
	if nullable {
		out[Epsilon] = true
	}
	return out
}

// Follow computes FOLLOW sets, putting the end marker "$" in FOLLOW(Start).
func (g Grammar) Follow() map[string]map[string]bool {
	vars := set(g.Variables)
	first := g.First()
	follow := map[string]map[string]bool{}
	for _, v := range g.Variables {
		follow[v] = map[string]bool{}
	}
	follow[g.Start]["$"] = true
	changed := true
	for changed {
		changed = false
		for _, p := range g.Productions {
			for i, sym := range p.RHS {
				if !vars[sym] {
					continue
				}
				suffix := p.RHS[i+1:]
				fs := g.FirstSequence(suffix, first)
				for token := range fs {
					if token != Epsilon && !follow[sym][token] {
						follow[sym][token] = true
						changed = true
					}
				}
				if len(suffix) == 0 || fs[Epsilon] {
					for token := range follow[p.LHS] {
						if !follow[sym][token] {
							follow[sym][token] = true
							changed = true
						}
					}
				}
			}
		}
	}
	return follow
}

// Simplify removes epsilon productions (except Start -> ε), unit productions,
// and symbols that cannot contribute to a terminal derivation or cannot be
// reached from Start. The returned grammar is a fresh value.
func (g Grammar) Simplify() Grammar {
	vars := set(g.Variables)
	nullable := map[string]bool{}
	changed := true
	for changed {
		changed = false
		for _, p := range g.Productions {
			ok := len(p.RHS) == 0
			if !ok {
				ok = true
				for _, s := range p.RHS {
					if !vars[s] || !nullable[s] {
						ok = false
						break
					}
				}
			}
			if ok && !nullable[p.LHS] {
				nullable[p.LHS] = true
				changed = true
			}
		}
	}
	prods := []Production{}
	seen := map[string]bool{}
	add := func(p Production) {
		key := p.LHS + "->" + strings.Join(p.RHS, " ")
		if !seen[key] {
			seen[key] = true
			prods = append(prods, p)
		}
	}
	for _, p := range g.Productions {
		positions := []int{}
		for i, s := range p.RHS {
			if nullable[s] {
				positions = append(positions, i)
			}
		}
		for mask := 0; mask < (1 << len(positions)); mask++ {
			skip := map[int]bool{}
			for j, pos := range positions {
				if mask&(1<<j) != 0 {
					skip[pos] = true
				}
			}
			rhs := []string{}
			for i, s := range p.RHS {
				if !skip[i] {
					rhs = append(rhs, s)
				}
			}
			if len(rhs) > 0 || p.LHS == g.Start {
				add(Production{p.LHS, rhs})
			}
		}
	}
	unit := map[string]map[string]bool{}
	for _, v := range g.Variables {
		unit[v] = map[string]bool{v: true}
	}
	changed = true
	for changed {
		changed = false
		for _, p := range prods {
			if len(p.RHS) == 1 && vars[p.RHS[0]] && !unit[p.LHS][p.RHS[0]] {
				unit[p.LHS][p.RHS[0]] = true
				changed = true
				for x := range unit[p.RHS[0]] {
					if !unit[p.LHS][x] {
						unit[p.LHS][x] = true
					}
				}
			}
		}
	}
	noUnit := []Production{}
	for _, a := range g.Variables {
		for b := range unit[a] {
			for _, p := range prods {
				if p.LHS == b && (len(p.RHS) != 1 || !vars[p.RHS[0]]) {
					noUnit = append(noUnit, Production{a, append([]string(nil), p.RHS...)})
				}
			}
		}
	}
	generating := map[string]bool{}
	changed = true
	for changed {
		changed = false
		for _, p := range noUnit {
			ok := true
			for _, s := range p.RHS {
				if vars[s] && !generating[s] {
					ok = false
				}
			}
			if ok && !generating[p.LHS] {
				generating[p.LHS] = true
				changed = true
			}
		}
	}
	reachable := map[string]bool{g.Start: true}
	changed = true
	for changed {
		changed = false
		for _, p := range noUnit {
			if !reachable[p.LHS] {
				continue
			}
			for _, s := range p.RHS {
				if vars[s] && !reachable[s] {
					reachable[s] = true
					changed = true
				}
			}
		}
	}
	final := []Production{}
	for _, p := range noUnit {
		ok := generating[p.LHS] && reachable[p.LHS]
		for _, s := range p.RHS {
			if vars[s] && !generating[s] || !vars[s] && !set(g.Terminals)[s] {
				ok = false
			}
		}
		if ok {
			final = append(final, p)
		}
	}
	out := g
	out.Productions = final
	out.Variables = []string{}
	for _, v := range g.Variables {
		if reachable[v] && generating[v] {
			out.Variables = append(out.Variables, v)
		}
	}
	return out
}

func set(items []string) map[string]bool {
	out := map[string]bool{}
	for _, x := range items {
		out[x] = true
	}
	return out
}
func formKey(form []string) string { return strings.Join(form, "\x00") }

// Sorted returns a stable copy useful when displaying set-valued results.
func Sorted(values map[string]bool) []string {
	out := []string{}
	for v := range values {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
