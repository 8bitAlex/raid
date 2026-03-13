# Coding Standards

## Core Principles

- **Readable over clever** — code is read far more often than written.
- **Single Responsibility** — every function, type, and package does one thing.
- **Self-documenting** — names and structure should make intent obvious without comments.
- **DRY** — don't repeat yourself; but don't over-abstract prematurely either.
- **No util/helper/common/misc packages** — these are catch-all bins that accumulate debt. Put code in the package whose domain it belongs to.
- **Short functions** — if a function needs a comment to explain what a section does, that section should be its own function.
- **Lean parameter lists** — more than three parameters is a signal to introduce a type.

## Naming

- Names reveal intent: `usersByID` not `m`, `retryCount` not `n`.
- Follow ecosystem conventions: Go uses `MixedCaps`, not `snake_case`.
- No obscure abbreviations: `request` not `req`, `response` not `resp` (except where the convention is universal and unambiguous, e.g. `ctx`, `err`).
- Package names are singular, lowercase, and noun-based: `profile`, `repo`, not `profiles`, `repoUtils`.

## Comments

- Every exported type, function, and constant gets a doc comment.
- Comments explain *why*, not *what* — the code already shows what.
- Never commit commented-out code. Delete it; version control remembers.
- Don't annotate the obvious: `// increment i` above `i++` adds noise, not signal.

## Testing

- Everything gets tests. No exceptions for "simple" code.
- Test behavior, not implementation — tests should survive refactors.
- Failing tests must be self-explanatory: the failure message should tell you what broke and why, without reading the test body.
- Table-driven tests are preferred in Go for covering multiple cases cleanly.
- Avoid mocking internals; mock at boundaries (I/O, network, time).

## Go-Specific

- **Explicit error handling** — never ignore an error with `_` unless the reason is documented.
- **Composition over inheritance** — embed interfaces and structs; avoid deep type hierarchies.
- **Errors are values** — wrap with context using `fmt.Errorf("doing X: %w", err)`; return, don't panic.
- **No `init()` side effects** — prefer explicit initialization at the call site.
- **Interface segregation** — define small, focused interfaces at the point of use, not in the package that implements them.
- **No util/common/misc packages** — see Core Principles above.
- Doc comments on all exported types follow the `// TypeName does...` convention.

## Reference Shelf

- *Clean Code* — Robert C. Martin
- *Clean Architecture* — Robert C. Martin
- *The Pragmatic Programmer* — Hunt & Thomas
- *Design Patterns (GoF)* — Gamma et al.
- *Effective Java* — Joshua Bloch (language-agnostic principles translate well)
