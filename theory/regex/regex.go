// Package regex parses a deliberately small regular-expression language and
// compiles it to an epsilon-NFA using Thompson's construction.
package regex

import (
	"fmt"
	"sort"
)

// Kind identifies an expression node.
type Kind uint8
const ( Literal Kind = iota; Epsilon; Empty; Concatenation; Alternation; Star; Plus; Optional )

// Expr is an immutable-style regular-expression syntax tree.
type Expr struct { Kind Kind; Value string; Left, Right *Expr }

// Parse accepts literals, parentheses, |, *, +, ?, and ε. A backslash quotes
// the next character; adjacent atoms concatenate.
func Parse(pattern string) (*Expr, error) { p:=parser{input:[]rune(pattern)}; e,err:=p.parseAlternation(); if err!=nil{return nil,err}; if p.pos!=len(p.input){return nil,fmt.Errorf("unexpected character %q",p.input[p.pos])}; return e,nil }
type parser struct { input []rune; pos int }
func (p *parser) parseAlternation() (*Expr,error) { left,err:=p.parseConcat(); if err!=nil{return nil,err}; for p.peek()=='|' { p.pos++; right,e:=p.parseConcat(); if e!=nil{return nil,e}; left=&Expr{Kind:Alternation,Left:left,Right:right} }; return left,nil }
func (p *parser) parseConcat() (*Expr,error) { var nodes []*Expr; for p.pos<len(p.input) && p.peek()!=')' && p.peek()!='|' { atom,err:=p.parsePostfix(); if err!=nil{return nil,err}; nodes=append(nodes,atom) }; if len(nodes)==0{return &Expr{Kind:Epsilon},nil}; out:=nodes[0]; for _,node:=range nodes[1:] {out=&Expr{Kind:Concatenation,Left:out,Right:node}}; return out,nil }
func (p *parser) parsePostfix() (*Expr,error) { atom,err:=p.parseAtom(); if err!=nil{return nil,err}; for p.pos<len(p.input) { switch p.peek(){case '*':atom=&Expr{Kind:Star,Left:atom};p.pos++;case '+':atom=&Expr{Kind:Plus,Left:atom};p.pos++;case '?':atom=&Expr{Kind:Optional,Left:atom};p.pos++;default:return atom,nil} }; return atom,nil }
func (p *parser) parseAtom() (*Expr,error) { if p.pos>=len(p.input){return nil,fmt.Errorf("expected expression")}; ch:=p.input[p.pos]; if ch=='(' {p.pos++; e,err:=p.parseAlternation(); if err!=nil{return nil,err}; if p.pos>=len(p.input)||p.input[p.pos]!=')'{return nil,fmt.Errorf("unclosed group")};p.pos++;return e,nil}; if ch==')'||ch=='|'||ch=='*'||ch=='+'||ch=='?' {return nil,fmt.Errorf("unexpected character %q",ch)}; p.pos++; if ch=='\\' {if p.pos>=len(p.input){return nil,fmt.Errorf("dangling escape")};ch=p.input[p.pos];p.pos++}; if ch=='ε' {return &Expr{Kind:Epsilon},nil}; if ch=='∅' {return &Expr{Kind:Empty},nil}; return &Expr{Kind:Literal,Value:string(ch)},nil }
func (p *parser) peek() rune { if p.pos>=len(p.input){return 0};return p.input[p.pos] }

// NFA is the self-contained Thompson output. State IDs are dense integers.
type NFA struct { Start, Accept int; States []int; Alphabet []string; Transitions map[int]map[string][]int; Epsilon map[int][]int }

// Thompson compiles an expression to an epsilon-NFA.
func Thompson(e *Expr) NFA { b:=builder{nfa:NFA{Transitions:map[int]map[string][]int{},Epsilon:map[int][]int{}}}; start,end:=b.build(e);b.nfa.Start=start;b.nfa.Accept=end;b.nfa.States=append([]int(nil),b.states...);sort.Strings(b.nfa.Alphabet);return b.nfa }
type builder struct { nfa NFA; states []int; next int; alphabet map[string]bool }
func (b *builder) state() int {id:=b.next;b.next++;b.states=append(b.states,id);return id}
func (b *builder) edge(from int,symbol string,to int) {if symbol==""{b.nfa.Epsilon[from]=append(b.nfa.Epsilon[from],to)}else{if b.nfa.Transitions[from]==nil{b.nfa.Transitions[from]=map[string][]int{}};b.nfa.Transitions[from][symbol]=append(b.nfa.Transitions[from][symbol],to);if b.alphabet==nil{b.alphabet=map[string]bool{}};b.alphabet[symbol]=true}}
func (b *builder) build(e *Expr)(int,int){s,z:=b.state(),b.state();switch e.Kind{case Literal:b.edge(s,e.Value,z);case Epsilon:b.edge(s,"",z);case Empty:case Concatenation:a,c:=b.build(e.Left);d,f:=b.build(e.Right);b.edge(c,"",d);s,z=a,f;case Alternation:a,c:=b.build(e.Left);d,f:=b.build(e.Right);b.edge(s,"",a);b.edge(s,"",d);b.edge(c,"",z);b.edge(f,"",z);case Star:a,c:=b.build(e.Left);b.edge(s,"",z);b.edge(s,"",a);b.edge(c,"",a);b.edge(c,"",z);case Plus:a,c:=b.build(e.Left);b.edge(s,"",a);b.edge(c,"",a);b.edge(c,"",z);case Optional:a,c:=b.build(e.Left);b.edge(s,"",z);b.edge(s,"",a);b.edge(c,"",z)};for symbol:=range b.alphabet{found:=false;for _,x:=range b.nfa.Alphabet{if x==symbol{found=true}};if !found{b.nfa.Alphabet=append(b.nfa.Alphabet,symbol)}};return s,z}

// EpsilonClosure computes the closure of a set of Thompson states.
func (n NFA) EpsilonClosure(states map[int]bool) map[int]bool {out:=map[int]bool{};q:=[]int{};for s:=range states{out[s]=true;q=append(q,s)};for len(q)>0{s:=q[0];q=q[1:];for _,x:=range n.Epsilon[s]{if !out[x]{out[x]=true;q=append(q,x)}}};return out}
// Run recognizes a sequence of literal symbols.
func (n NFA) Run(input []string) bool {cur:=n.EpsilonClosure(map[int]bool{n.Start:true});for _,sym:=range input{next:=map[int]bool{};for s:=range cur{for _,x:=range n.Transitions[s][sym]{next[x]=true}};cur=n.EpsilonClosure(next)};return cur[n.Accept]}
