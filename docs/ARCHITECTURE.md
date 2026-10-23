# Architecture and syllabus evidence

```text
policy.sntl -> lexer -> tokens -> recursive-descent parser -> AST
                                      |                       |
                                      v                       v
                               syntax diagnostics      semantic validator
                                                              |
request -> evaluator (first-match) -> decision + trace -> net/http middleware

theory/regex -> Thompson epsilon-NFA -> subset DFA -> minimization
theory/cfg  -> derivations/FIRST/FOLLOW -> PDA demonstration
theory/csg  -> bounded recognizer -> LBA simulator -> TM simulator
```

| TOC concept | Evidence in this repository |
|---|---|
| DFA | `theory/automata.DFA`, transition simulator, completion, minimization |
| NFA / epsilon-NFA | `theory/automata.NFA`, closure and subset construction |
| Regular expressions | `theory/regex.Parse` and Thompson construction |
| CFG | `theory/cfg.Grammar`, derivations, FIRST/FOLLOW, simplification |
| PDA | bounded nondeterministic stack machine in `theory/pda` |
| Context-sensitive grammar | bounded recognizer in `theory/csg` |
| LBA | bounded tape machine in `theory/lba` |
| Turing machine | sparse-tape deterministic machine in `theory/tm` |
| Decidability | finite semantic validation and bounded production evaluator |
| Undecidability | documented comparison: unrestricted computation is deliberately outside the runtime |

The production compiler uses regular expressions only as bounded request predicates. The LBA and TM artifacts are deliberately isolated in `theory` so the project demonstrates the syllabus without claiming that policy execution requires unrestricted computation.
