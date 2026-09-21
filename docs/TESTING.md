# Testing

How ArchDoc is tested, and why in this shape. `spec/spec/` describes what the tool does and
`DECISIONS.md` records the choices behind it; this file covers the ones about the test suite.

## Running them

```
go test ./...                                    # the whole suite
go test ./internal/repo/                         # one package
go test ./cmd/archdoc/ -run TestTheDocumentedWorkflowWorksEndToEnd
go test ./internal/repo/ -run FuzzProseNeverMovesAByte -fuzz FuzzProseNeverMovesAByte
gofmt -l . && go vet ./...
```

Fixtures live under `testdata/`: `repo` is a valid repository, `lint` holds a case per rule, and
`crlf`, `links`, `empty`, `nofrontmatter` and `golden` each pin one situation.

## Approach

- **The documented journey is one test.** `TestTheDocumentedWorkflowWorksEndToEnd` walks init, commit,
  lint, `index --check`, new, propose, accept, all three glossary writers, link resolution, index, and an
  edit to a frozen document that L11 must catch, in a real git repository. Every other test pins one rule
  or one past defect; this one pins the thing a user actually cares about.
- **The offset invariant is fuzzed.** For every prose line, the text as written and the masked text are
  the same length and differ only by spaces. Everything that rewrites a document by offset depends on it,
  and it has survived three rewrites of the code beneath it.
- **A guard is not finished until it has been deleted and the suite watched going red.** Tests written in
  the same sitting as the code they cover repeatedly passed for a reason other than the one intended: a
  substring matched a message from a different rule, a loop ran over an empty collection, a fixture's own
  content satisfied the assertion. Mutation is the only reliable way to tell those apart.
- **Each conjunct of a compound condition is mutated separately.** Killing a whole-condition mutation
  proves only that *something* in it is pinned; one half of an `||` was recorded as covered while nothing
  exercised it.
- **A test naming a document and a symptom together asserts that one finding carries both.** A helper
  that matches each substring independently across every finding is satisfied by two unrelated findings,
  which left several rules pinned by nothing.
- **Fixtures are split in two.** `testdata/repo` is a valid repository asserted to produce no errors,
  which is the guard against false positives, and it is expected to contain what a real repository
  contains. `testdata/lint` holds a case per rule, and a test asserts that every registered rule fires, so
  a rule cannot be added without a fixture case.
- **The golden index is read before it is trusted.** Generating the expectation from the implementation
  proves nothing on its own.
- **`internal/repotest` builds throwaway repositories.** Six test packages had near-identical helpers. It
  imports `testing`, as `httptest` and `iotest` do, and sits under `internal/` so it never reaches a
  binary. `internal/lint`'s git tests keep their own builder, because those need a real checkout rather
  than a directory.
