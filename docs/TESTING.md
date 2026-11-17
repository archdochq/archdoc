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

Fixtures live under `testdata/`: `repo` is a valid repository, `lint` holds a case per rule,
`commonmark` holds the reference cases, and `crlf`, `links`, `empty`, `nofrontmatter` and `golden`
each pin one situation.

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
  contains. `testdata/lint` holds a case per rule, and a test asserts that every registered L rule fires
  on it or on `testdata/repo`, so a rule cannot be added without a fixture case. The merge rules, M01 to
  M04, need a base in git, which a fixture directory is not, and `internal/lint/merge_test.go` builds one
  for each.
- **The golden index is read before it is trusted.** Generating the expectation from the implementation
  proves nothing on its own.
- **`internal/repotest` builds throwaway repositories.** Six test packages had near-identical helpers. It
  imports `testing`, as `httptest` and `iotest` do, and sits under `internal/` so it never reaches a
  binary. `internal/lint`'s git tests keep their own builder, because those need a real checkout rather
  than a directory.

## The reference cases

`internal/markdown`'s outline is compared with cmark-gfm's own parse, the reading GitHub gives, rather
than with readings written by hand, which hold only the cases someone thought of.

- **The cases.** `testdata/commonmark` holds the 672 examples of the GFM specification 0.29 and the 30
  and 26 of cmark-gfm's extension and regression tests, at the tag `REFERENCE` names, attributed in
  `ATTRIBUTION.md`, and ArchDoc's own under `archdoc/`: each misreading found, and each difference
  between goldmark and the reference found, as a case. Each case's `.md` is the body and its `.xml` the
  reference's parse, as `cmark-gfm -t xml --sourcepos` writes it with the extensions `REFERENCE` names.
  A body is read as written, line endings included, so `.gitattributes` keeps git from changing them.
- **Recording a parse.** `go test ./internal/markdown -update-commonmark` records every case's parse
  again. It fails where `cmark-gfm` is not on the path or reports a version other than the one
  `REFERENCE` names. Nothing else needs cmark-gfm: the comparison reads the recorded parses.
- **What is compared.** The tree of blocks and pieces, their types, the attributes the parse carries,
  and their text, with adjacent texts joined, as the two divide text at different places. The outline's
  link reference definitions are passed over, as the parse keeps none. The reference writes footnote
  references and definitions as `<unknown>` tags that are not well-formed XML, moves each referenced
  definition to the end in the order first referenced and drops the rest, so the comparison reads the
  empty tag inside a block as a reference and any other as a definition, and moves the outline's
  definitions the same way. A task is compared as a list item beginning with its box.
- **Known differences.** `KNOWN` lists each case on which goldmark reads otherwise than the reference,
  the path of the first node that differs, and why. The test fails where a listed case differs at
  another node, and where it no longer differs, so an entry goes when a release of goldmark reads the
  case as the reference does.
- **Positions are checked by rule, not against the reference.** cmark-gfm's positions are not exact:
  it places a code span at its content, an address it finds a column early, and pieces after a link
  reference definition on the wrong line. `TestEveryCaseIsPlacedByTheRules` asserts of every case that
  each block and piece starts at the first byte of its construct, lies within its parent, follows its
  siblings without overlapping them, that a text's bytes are its text where it holds no escape or
  entity, and that a destination's bytes, resolved, are its destination.
- **The fuzz test needs cmark-gfm.** `FuzzOutlineMatchesTheReference` compares the outline with the
  reference's parse of generated input where the reference is on the path, and asserts the position
  rules either way. Variants of the known differences in tables and addresses are common in generated
  input, so a run stops at one quickly; a difference that is new becomes a case.
- **Timing tests of reading.** Each shape cmark-gfm's pathological tests hold, the shapes added beside
  them, and each shape the readers' own timing tests hold, is read under a limit. A shape goldmark reads
  in more than linear time is skipped, naming it, until it does not.
