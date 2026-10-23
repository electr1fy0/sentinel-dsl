// Package csg contains a bounded recognizer for context-sensitive grammars.
// Since unrestricted derivation search is undecidable, Recognition always
// takes explicit sentential-form and step bounds.
package csg

import "strings"

// Production is a context-sensitive rewrite of a whole substring.
type Production struct { LHS, RHS []string }
// Grammar is a noncontracting grammar, optionally allowing Start -> ε.
type Grammar struct { Variables, Terminals []string; Start string; Productions []Production }
// Options bounds the breadth-first recognition search.
type Options struct { MaxLength, MaxSteps int }
// Result describes bounded recognition.
type Result struct { Accepted bool; Steps int; Derivation [][]string }

// Recognize searches from Start to the exact input. A production is applicable
// at every matching position. Rules that shorten a form are ignored except a
// start-only epsilon rule.
func (g Grammar) Recognize(input []string, opts Options) Result {if opts.MaxLength<=0{opts.MaxLength=len(input)};start:=[]string{g.Start};queue:=[]item{{start,0,[][]string{start}}};seen:=map[string]bool{key(start):true};for len(queue)>0{cur:=queue[0];queue=queue[1:];if equal(cur.form,input){return Result{true,cur.steps,cur.path}};if cur.steps>=opts.MaxSteps{continue};for _,p:=range g.Productions{if len(p.RHS)<len(p.LHS)&&!(len(p.LHS)==1&&p.LHS[0]==g.Start&&len(p.RHS)==0){continue};for i:=0;i+len(p.LHS)<=len(cur.form);i++{if !equal(cur.form[i:i+len(p.LHS)],p.LHS){continue};next:=append([]string{},cur.form[:i]...);next=append(next,p.RHS...);next=append(next,cur.form[i+len(p.LHS):]...);if len(next)>opts.MaxLength{continue};k:=key(next);if !seen[k]{seen[k]=true;path:=append(append([][]string(nil),cur.path...),next);queue=append(queue,item{next,cur.steps+1,path})}}}};return Result{}}
type item struct{form []string;steps int;path [][]string}
func equal(a,b []string)bool{if len(a)!=len(b){return false};for i:=range a{if a[i]!=b[i]{return false}};return true}
func key(x []string)string{return strings.Join(x,"\x00")}
