# SentinelDSL Specification

## Scope

SentinelDSL describes finite HTTP request authorization policies. A policy has a name, an ordered list of rules, and one default action. A rule has a boolean condition and an allow/deny action.

## Execution

Rules are evaluated in source order. The first true condition determines the decision. If no condition is true, the default action determines the decision. Missing or invalid policy data is deny-by-default at integration boundaries.

## Request fields

The standard fields are `method`, `path`, `host`, `scheme`, `remote_addr`, `user_agent`, and `role`. Indexed maps are available as `headers["name"]`, `query["name"]`, and `fields["name"]`.

## Actions

Actions are `allow` or `deny`, optionally followed by an integer status and a quoted message. Status values are checked to be in the HTTP range 100–599. Middleware defaults denied requests to status 403 when no status is supplied.

## Termination and decidability

Policies are finite trees over a finite rule list. There are no loops, recursion, dynamic code, or unbounded policy iteration. Evaluation visits each rule at most once and each expression tree node at most a finite number of times. Consequently policy validity and evaluation are decidable, and evaluation terminates.

The `theory` directory demonstrates stronger machines for educational comparison; those machines are not part of the production policy runtime.
