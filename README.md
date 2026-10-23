# SentinelDSL

SentinelDSL is a small, terminating policy language and Go reference implementation for HTTP authorization. It turns a policy file into an AST, validates it, evaluates rules in source order, and emits an explainable decision trace.

## Quick start

```sh
export PATH=/usr/local/go/bin:$PATH
go test ./...
go run ./cmd/sentinel check examples/policy.sntl
go run ./cmd/sentinel explain examples/policy.sntl examples/request.json
go run ./cmd/sentinel automata build '(a|b)*'
```

The CLI supports:

```text
sentinel check policy.sntl
sentinel compile policy.sntl
sentinel eval policy.sntl request.json
sentinel explain policy.sntl request.json
sentinel automata build|minimize|test pattern [input]
sentinel serve policy.sntl [addr]
```

The runtime language intentionally excludes loops, recursion, user-defined functions, dynamic execution, network/filesystem access, and unbounded iteration. Rules are first-match wins and the explicit default action is required by semantic validation.

## Repository map

- `cmd/sentinel`: CLI and HTTP demo server.
- `internal/lexer`, `parser`, `ast`, `semantic`: compiler front-end.
- `internal/evaluator`, `middleware`: policy execution and `net/http` integration.
- `theory/*`: independent DFA/NFA/regex/CFG/PDA/CSG/LBA/TM teaching laboratory.
- `examples`: runnable policy and request fixtures.
- `docs`: architecture, grammar, algorithms, and review evidence.

## Security model

The default posture is deny. A policy can only inspect the fixed request fields exposed by `evaluator.Request`; it cannot perform I/O or invoke arbitrary Go code. Regular-expression matching uses Go's standard regular-expression engine, and every policy has a finite rule list.

## Browser API

`sentinel serve` (and the embedded API handler) exposes JSON endpoints for the
studio. Policy endpoints include `POST /api/validate`, `/api/evaluate`,
`/api/evaluate/batch`, `/api/explain`, `/api/ast`, and `/api/lexer`. Theory
endpoints include `POST /api/automata` and `/api/theory/cfg`, `/pda`, `/tm`,
`/lba`, and `/csg`. Theory simulations are bounded by the supplied
`max_steps`; CSG recognition also accepts `max_length`.

Cross-origin requests are restricted to local development origins by default
(`http://localhost:3000` and `http://127.0.0.1:3000`). Set
`SENTINEL_ALLOWED_ORIGIN` to a comma-separated allowlist in a deployed
environment, for example `https://studio.example`. Same-origin requests do
not require CORS configuration.
