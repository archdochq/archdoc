# Decisions

Why ArchDoc works the way it does. `spec/spec/` describes what the tool does; this file records the
choices behind it and the reasoning, including the alternatives that were tried and rejected. Decisions
about the test suite are in `TESTING.md`. `internal/template/PROCESS.md` is authoritative where any of
them disagree.

Entries are grouped by subject. Many exist because the obvious implementation was wrong in a way that
only showed up on a real document, and the reasoning is kept so the shape is not undone later.

## Documents and front matter

- **Opening a repository never fails on a malformed document.** Faults are recorded per document as
  `Problem` values carrying a line and a message. A document that fails to parse would otherwise stop
  lint at the first bad file, when the whole point of `archdoc lint` is to report everything wrong at
  once. Open returns an error only for faults that make the repository unreadable.
- **`repo` records problems; `lint` decides severity and rule identity.** Nothing in `repo` knows what
  a finding is, which keeps the parsing usable by `index`, `link` and any later frontend that has no
  interest in rules.
- **A document's identifier is derived from its type and filename, never from the front matter.**
  PROCESS.md makes the number the identity, so the front matter `id` is data to be checked against the
  derived value. A filename that does not match `<NNNN>-<slug>.md` leaves the identifier empty, which
  is the signal L02 reports on.
- **Front matter is parsed once into a `yaml.Node` and then decoded into a struct.** The node's mapping
  keys give presence and unknown-field detection; a struct decode alone cannot distinguish `decided:`
  from an absent `decided`, which the schema requires.
- **What a key may be is separate from what must be present.** They were one list, which made it
  possible to add a key that every existing document lacked. `backfilled` is optional and every key
  added after it must be optional too: a new required key would put every frozen document in every
  existing repository in violation of L01, and L11 forbids the edit that would add it. The schema can
  only ever grow optional fields.
- **A required key that carries no value is a fault distinct from an absent key.** A document with an
  empty `status` was otherwise skipped in silence by every rule that branches on status. Front matter
  records the line each key was written on, so the finding points at it.
- **Dates are decoded through the node rather than read off its value.** For an alias node the value is
  the anchor's name, so `created: &d 2026-01-05` with `decided: *d` reported that `"d"` is not a date,
  naming something the author never wrote.
- **A byte order mark is trimmed before the opening delimiter is matched.** Otherwise it hides the
  front matter entirely and produces a cascade of missing-key errors on a document that looks correct.
- **Titles are encoded as YAML scalars, not interpolated.** Writing `title: {{ .Title }}` meant
  `archdoc new rfc "Database: PostgreSQL"` produced a document that failed L01 on creation; leading
  `#`, `-`, `[`, `{`, `&`, `*`, `!`, `|`, `>` and `%` fail the same way. A `yaml` template function
  marshals through yaml.v3, so the library that parses the file decides how it is written. The H1 keeps
  the raw title and is read back by splitting on the first `": "`, so a title may contain colons.
- **`new` refuses a title that is empty or slugs to nothing**, which would otherwise produce `0001-.md`,
  and refuses one carrying a line break, which produces an H1 that fails L10. It also refuses to write
  a heading that does not read back as the title it was given.
- **Numbering stops at 9999.** The filename grammar requires exactly four digits, so a fifth would make
  every later command unable to read the document just written. Reaching it needs no 9999 documents: one
  hand-written `rfc/9999-old.md` is legal and the next `new` would have crossed the boundary.
- **Configuration decodes on top of a struct already holding the defaults**, so an absent key keeps its
  default and `strict` defaults to `true` rather than to Go's zero value. `Path` is tagged `json:"-"`,
  so a file naming it is an unknown-field error rather than a way to set it. The decoder must also reach
  end of file: `Decode` reads one value and leaves the rest of the stream, so a second object or any
  trailing text was silently ignored.

## Markdown

The tool needs headings, comments, fenced blocks and links, and nothing else. A full parser would be a
liability when rewriting files in place, because it would have to round-trip everything it parsed.

- **One walk per document.** The scanner that decides what is a comment, a fence or a code span is not
  only a shared *function* but a shared *traversal*: its masked output is cached on the document, and
  section text is a slice of it. Running the same function again on a fragment starts from a clean
  state and disagrees with the walk that decided the heading was a heading, which let a section
  containing only a comment read as full.
- **A comment is nothing, wherever it is.** `Section.Empty`, the glossary's paragraph split, heading
  parsing and link scanning all read the same masked text, so a comment on one line behaves like a
  comment spanning several. Comments are blanked in place rather than dropped, which keeps every byte
  offset valid and lets prose sharing a line with a comment survive. An unterminated comment runs to
  the end of the body, as a browser reads one.
- **Code wins over comments; whichever of a comment and a fence opens first wins over the other.** A
  `<!--` inside a code span or a fenced block is literal text, which is what lets a document describe
  the placeholders the templates ship with. A fence marker inside a comment is likewise literal. Both
  are blocks, and "whichever opens first" is the only rule needed to order them.
- **A code span closes on a backtick run of exactly the length that opened it.** Go's `regexp` is RE2
  and has no backreferences, so "the same length" cannot be written as a pattern: it is scanned for.
  The pattern this replaced accepted any closing run, so a double-backtick span was cut short at its
  first inner backtick and everything after it mis-parsed.
- **A fence closes on a run at least as long as the one that opened it, carrying no info string.** An
  info string is permitted on the opening fence only, so a line reading ```` ``` inside ```` within a
  ```` ```text ```` block is content. One pattern serving both ends ended the block early and let the
  real closing fence open a second one.
- **Masking never moves a byte.** Every masker writes spaces in place, so a match found in the masked
  text is at the same offset in the text as written, and an edit spliced by offset lands in the right
  column. This is the invariant the whole rewriting design rests on, and it is fuzzed rather than
  sampled.
- **Heading anchors follow GitHub, including the `-1`, `-2` suffix for a repeat.** Without the suffix
  L17 reports an error on links that work, because a page with two `## Configuration` headings has a
  real `#configuration-1` and that is what a reader copying from their browser will write. Anchors are
  assigned by walking a document's headings in order against a seen-set, never computed from a heading
  in isolation, and are slugged from the heading's *rendered* text so link syntax is stripped first.
- **Link destinations are read in every form CommonMark allows and written in one.** `Document.Links`
  understands a bare destination, the `<...>` form and a link reference definition, and percent-decodes
  before comparing against a path on disk. `repo.LinkDestination` percent-encodes on the way out. The
  two are inverses, and encoding rather than escaping avoids the interaction where a path's own
  backslash pairs with an added one and lets the destination close early.
- **A line is a link reference definition only if that is all it is.** The pattern is anchored at both
  ends, requires a non-empty label, allows a title and does not count a line that interrupts a
  paragraph. Without the end anchor a footnote, a numbered citation or any sentence beginning with a
  bracketed word was read as a link and reported as a broken one, and a citation list is exactly what a
  `Sources` section looks like.

## Sections

- **One required-section list, read by everything that asks the question.** `Document.RequiredSections()`
  adds `Sources` when a document is backfilled and `Rejection rationale` when it is rejected, and L08,
  L14 and the pre-freeze gate all read it. Written against separate lists, they disagreed: a backfilled
  document whose `Sources` held only the template comment satisfied both, because one checked presence
  and the other was not looking.
- **A document has a closing section only where a type states one.** `ClosingSection` is the last entry
  of the required list when that entry is a closing kind. Requiring `Sources` last *and* the rejection
  rationale after `Sources` was unsatisfiable, so a documented flag combination produced a document that
  could never lint clean and that L11 then froze. PROCESS.md settles the order: the rationale is final,
  and a backfilled document merely carries `Sources`.
- **The closing rule is checked only when the section is present.** Reporting that a missing section is
  also in the wrong place is two findings for one fault, the second contradicting the first.
- **Extra H2 sections are permitted** between or after the required ones. Forced anyway by the rejection
  rationale appending after `Changelog`, and by refs requiring `Sources` last.
- **A section includes its own subsections**, ending at the next heading of the same level or higher, so
  a `Proposal` structured with `###` headings is not reported empty.
- **A section is empty if it holds nothing but whitespace and comments.** That definition is the reason
  L14 checks emptiness rather than the absence of comments: a comment cannot be both the definition of
  emptiness and a violation in its own right.

## The lifecycle and freezing

- **Freezing is gated at the transition, not after it.** `accept` and `reject` refuse an empty required
  section and an unresolved `[[...]]`; `reject` runs both before appending the rationale. `withdraw`
  gates nothing, because a document abandoned before a verdict is empty by nature.
- **The gate refuses everything the linter would report forever.** It checks the H1, the required
  sections' presence and their relative order, reusing the same list and the same H1 accessor the rules
  use rather than restating them. What the gate lets through, L11 then forbids anyone repairing, so a
  mismatched H1 or a missing `Changelog` became a permanently red repository with no in-tool remedy.
- **A lint error must name something the user is permitted to change.** A finding that no permitted edit
  to the named document could clear is a warning: L14, L16 and L17, whose subject is the document's own
  body. It does not extend to a fault in a relationship between documents, which is cleared by editing
  the other document, nor to L11 itself.
- **A transition refuses to write a `decided` date before `created`.** Nothing rejects a `created` date
  in the future, so a draft carrying one lints clean until the transition writes the value that makes
  L07 fail permanently. It needs no hand-editing to reach: `new` on one machine and `accept` on another
  read the local clock, and a team spanning enough offsets differ by a day.
- **A transition refuses a status the graph does not know**, before consulting the graph. The graph
  answers "nothing permitted" for anything it does not recognise, and reading that as "the document is
  terminal" told the author to write a superseding document when the fault was a one-line front matter
  edit.
- **A transition refuses a document whose parse stops short.** The rejection rationale is appended to the
  end of the file, and when the body ends inside an unterminated comment or an unclosed fence that
  position is inside the comment, so the command reported writing a section that did not then exist.
- **Backfilling is a mode of creation, not a lifecycle path.** A backfilled document was never proposed,
  so routing it through `draft → proposed → accepted` would fabricate three events. The transition graph
  is untouched. `created` and `decided` carry the historical dates and `backfilled` the date of writing;
  dating `created` to the day the file was written would trip L07, and omitting `backfilled` would claim
  the document existed all along.
- **A backfilled document requires `Sources`**, as a ref does and for the same reason: a reconstruction
  with no evidence is someone's recollection. `Alternatives considered` and `Backwards compatibility`
  are pre-filled with `Not recorded.` rather than exempted, because an empty section is ambiguous
  between "there were none" and "nobody wrote them down", and an invented alternative is worse than
  either.

## Writers

Every writer parses a document, computes a line range and splices. The parser is defined so that one
malformed construct shortens its view of the file, so a writer that trusts the range without checking
the result can delete content it never read.

- **A writer verifies the structure it produced.** `SetField` re-parses its output and refuses if the
  front matter no longer reads correctly. The glossary writers re-read the rewritten page and refuse
  unless the terms on it are exactly those intended, and unless a newly added entry reads back as one
  paragraph. That single rule subsumes every "refuse this input" check that would otherwise have to be
  enumerated: a heading hidden in a definition, an unclosed fence, a `Formerly` line offered as prose.
- **A writer refuses a document whose parse stops short of the file.** When the body ends inside an
  unterminated comment or an unclosed fence, every line range derived from the parse runs past the
  entries the parser could not see, and removing one glossary term deleted every entry below it on a
  page that lint passed.
- **Front matter is rewritten field by field.** `SetField` replaces the value on the key's own line and
  leaves the rest of the block alone, so ordering, spacing and comments survive; re-serialising through
  a YAML encoder would be shorter and would silently discard all three. A value may continue beneath its
  key in several shapes, and all of them are recognised, because replacing only the key's line orphans
  the tail and produces front matter that no longer parses. A `#` inside quotes is not a comment, and a
  carriage return is preserved so a CRLF file stays CRLF. A comment written among a value's items is
  refused rather than deleted.
- **Nothing is written through a symbolic link, and nothing is written outside the repository.** One
  helper refuses a path that is a symlink, and one resolves every directory component before the write,
  because a root can be perfectly ordinary while a document directory inside it points elsewhere.
  `os.WriteFile` follows a link and truncates the target, and a symlink committed to a repository
  survives clone.
- **`init` plans every file before writing any of them**, tests for collisions with `Lstat` so a
  dangling link counts as something in the way, and creates with `O_EXCL` so the kernel performs the
  test. It validates the configuration it is about to write through the same code path that loads one,
  after prompting and before writing: checks reached only by reading the file back left a dead
  `archdoc.json` behind that `init` then refused to overwrite.
- **`link` leaves a document entirely untouched if any of its wiki links resolves to nothing.** A
  partial rewrite would leave the file in a state neither the author nor the tool intended. Every other
  document is still written, because the failure belongs to one file.
- **Interactive suggestion application re-reads before writing.** The snapshot is taken before the first
  prompt and the write happens after the last answer, so the whole session is a window in which an edit
  made elsewhere would be silently discarded. A file that has moved on is left alone and said so.
- **`archdoc init` followed by `archdoc lint` and `archdoc index --check` exits 0.** A scaffolded
  repository must pass the workflow `init` itself writes, which is why `INDEX.md` is generated rather
  than templated and why the glossary template may carry a preamble and no entries.

## Repository layout

- **Everything but `archdoc.json` and the workflow lives under `root`.** `README.md`, `PROCESS.md`,
  `INDEX.md` and `LICENSE` join the four document directories, so a spec repository can sit inside a
  code repository without colliding with that repository's own README or licence.
- **`root` must name the directory holding `archdoc.json` or one below it.** The check is applied twice:
  lexically, on the spelling, and again on the resolved path, because a symlink passes the first and
  not the second. Without it `{"root": "../victim"}` made `index` write into a directory nobody named.
- **The workflow is written at the git repository root**, never under `root`, because GitHub runs
  workflows only from there and writing it elsewhere fails silently: no error, and enforcement simply
  never happens. It carries a `working-directory` pointing back at the directory holding
  `archdoc.json`, since commands find their configuration by walking up. `init` writes
  `archdoc.json` at that same root, so the value is empty for anything it scaffolds and names a
  path only for repositories scaffolded before it did.
- **`init` prints every path it writes**, because it can write outside the current directory and that is
  surprising without a record of it.
- **`README.md`, `PROCESS.md` and `INDEX.md` are not documents** under the discovery rule, so no rule
  examines them, L17 included.

## Commands and interaction

- **Prompting substitutes for values the command was not given, and for nothing else.** A prompt is
  offered for required arguments and for every optional flag whose value reaches a file: `archdoc.json`,
  `LICENSE`, or a document's front matter. Flags that select a mode of operation write nothing, and
  prompting for them would ask the user to re-specify the command already typed. The form is built field
  by field from what the command line settled, so a fully specified invocation from a terminal writes
  immediately rather than rendering a form and waiting.
- **`--no-interaction` is a persistent flag on every command**, and is honoured whatever stdin is.
- **A terminal is detected with `term.IsTerminal`, not by asking whether stdin is a character device.**
  `/dev/null` is a character device, and it is how a shell, Makefile, cron job or CI runner says "no
  input", so the tool's default behaviour in automation was to prompt.
- **Exit codes travel as an error type.** A command that has already printed its findings returns a
  silent exit carrying the status; anything else is a usage or validation failure, printed once. Nothing
  under `internal/` calls `os.Exit`, which is what keeps those packages usable from a frontend.
- **Findings are printed relative to `root`**, so output does not depend on the working directory. JSON
  output is a bare array, progress notices go to stderr, and `--json` therefore puts the array and
  nothing else on stdout.
- **Commands take their writer from cobra** rather than carrying one through every signature, so a test
  reads what a command wrote instead of capturing the process's streams.

## Links and the glossary

- **A suggestion wraps the matched text; `[[X]]` expands it.** `RFC-0007` in prose becomes
  `[RFC-0007](path)`, not `[RFC-0007: <title>](path)`: a placeholder the author wrote expecting
  substitution is a different case from finished text, and expanding in prose rewrites their sentence.
- **Matching is deliberately conservative.** Identifiers match the canonical uppercase form; glossary
  terms match whole, case-insensitively, longest first, with no stemming and no plural handling. A
  missed suggestion costs a keystroke; a wrong match corrupts a sentence.
- **"Whole" is read from the adjacent runes, not from a word-boundary assertion.** Go's `\b` is an ASCII
  boundary, so for a term whose own edges are not word characters the assertion inverts: `C++` failed
  where it stood alone and matched inside `C++11`. Reading the runes on either side asks the question
  directly and handles `Café` and `Cafés` too.
- **Neither `link` nor `link --suggest` rewrites a heading.** A heading's raw text is a section's
  identity to L08, L09, L14 and the freeze gate, so wrapping one in a link deletes the section from
  every rule at once while leaving the anchor resolving.
- **The masker is deliberately generous, and withholds a line it cannot read.** It blanks wiki links,
  inline and reference links, link reference definitions, any bracketed span, bare URLs, autolinks and
  raw HTML; and it withholds the whole line when a raw HTML tag is present or the brackets are
  unbalanced after masking, because both mean a construct whose extent a line-at-a-time masker cannot
  see. Over-masking costs one missed suggestion; getting it wrong splices a link inside a link, which
  CommonMark forbids, and demotes the author's link to literal text with nothing able to see it
  afterwards.
- **A term reaching a link label is escaped**, as a title is, so a term carrying a bracket or a pipe
  cannot put live markup into a document its author never touched.
- **A glossary entry ends where the walk that found it says.** Deriving the extent from the next entry
  instead runs straight through an ordinary H1 section sitting among them, and removing a term then
  deleted it.
- **The glossary may carry a preamble and may have no entries.** Anything before the first H2 is
  preamble and is ignored; entries are inserted after it. Arbitrary preamble is permitted rather than
  comments alone, so the glossary is not the one spec page where a lead sentence is illegal.
- **`Formerly` lines accumulate**, one per line after the definition paragraph, so repeated renames leave
  the full chain. They are recognised line by line rather than as a blank-line-separated block, and
  paragraphs split on a pattern that matches CRLF.
- **`rename` is remove then re-insert**, which re-sorts the entry for free and keeps one insertion
  routine. The entry's own lines are carried across verbatim; rebuilding it from its parsed paragraphs
  deleted comments and reflowed fenced blocks.
- **`term rename` does not rewrite links elsewhere, and `link` cannot repair them.** `link` expands
  `[[...]]` and never edits an existing markdown link, and a recorded former name is not a term, so
  `[[<old name>]]` does not resolve either. L17 reports them and they are corrected by hand.

## Linting

- **Every rule is a function returning findings; nothing prints or exits.** The registry is the single
  source of a rule's code, so the rule functions do not repeat it on every finding they build.
- **Findings are accumulated through one helper**, which separates a message that is already built from
  one that is a format string. A parse error naming a value of `100%` would otherwise be formatted a
  second time and reach the user as `100%!(MISSING)`.
- **A rule's context cannot be built in a broken state.** Its fields are unexported and one constructor
  fills them, because a context built as a literal without the branch snapshot reported every document
  as editable and made the same document an error from one command and a warning from another.
- **The branch snapshot is built on first use.** `link` asks whether a document is frozen only when a
  terminal document turns out to contain a wiki link, which on most repositories is never, and building
  the snapshot eagerly cost a git subprocess per document for an answer nobody wanted.
- **Whether a frozen document has changed is decided by git, not by comparing bytes.** git applies
  checkout filters between the object it stores and the file on disk, so under end-of-line
  normalisation, the default on Windows, every frozen document differed from its blob while the tree was
  clean. Pathspecs are anchored to the repository root and `diff.relative` is forced off, because both
  sides of that assumption resolve paths relative to the working directory otherwise.
- **A missing branch is two different situations.** With no commits at all nothing can be frozen, so the
  rule is skipped with a warning: `archdoc init` followed by `archdoc lint` is the first thing anyone
  does. With commits present a missing branch means the configured name is wrong, and passing quietly
  would disable the freeze check altogether.
- **Only a type with a lifecycle can be frozen.** A status key can be read out of any committed blob, so
  a spec page or a ref carrying one by mistake was treated as frozen and every later edit to it became
  an error on a type that has no lifecycle at all.
- **An empty status is never rendered into a message.** A ref has none by design, and a document may
  carry the key with nothing after it; the rules that compare statuses stay quiet rather than producing
  "which is , while".
- **A rule never reports a false companion to its own accurate finding.** A key whose value could not be
  parsed is not also reported as empty, because the decoded value is zero only because parsing failed.
- **A link target that leaves the repository is refused rather than read.** Existence alone is an answer
  about a file the repository does not own.
- **Every reference rule is monotonic.** `includes`, `updates` and `obsoletes` target accepted documents,
  and accepted is terminal, so a reference that is valid stays valid and no document can be put in
  violation by another author's later action. That is what keeps frozen documents from spontaneously
  failing lint.
- **Findings never come from map order.** Findings sharing a path, a line and a rule are ties under the
  output sort, so any rule driven from a map differed between runs. A map is never a source of order,
  and a stable sort cannot rescue an order that never existed.

## The index

- **`INDEX.md` is generated, not templated**, so a scaffolded repository passes the `index --check` its
  own workflow runs.
- **All four sections are always emitted** with their header rows, so the file does not change shape as
  the repository grows and `index --check` is stable immediately after `init`.
- **Every cell is escaped, and every destination is encoded.** One renderer percent-encodes each
  character that would end a destination, a link or the table cell around it, and one escape function
  handles labels. A filename and a title are both free text reaching a file headed "Do not edit".
- **`Stale` renders `yes` or empty**, following the "empty cells are empty" convention. A column of `no`
  is noise.
- **A status carries its qualifiers in parentheses**, comma-separated in a fixed order, generalising the
  single obsolete case rather than special-casing a second one.
- **A unified diff is implemented here** rather than adding a dependency, because `index --check` is
  specified to print one and an index is a few hundred lines.

## Release

- **The version resolves from the goreleaser ldflag, then the module's build information, then `dev`.**
  The middle step matters: Go embeds the resolved tag for `go install ...@latest`, so those users get a
  correctly pinned workflow instead of being treated as unversioned.
- **Only a real release tag is pinnable.** Anything else, including `dev` and the pseudo-version
  `go install ...@main` reports, names an asset that can never be published, so the workflow is written
  as `latest` and the command says so. Testing for the literal string `dev` alone wrote a workflow whose
  first CI run could only fail.
- **The download is verified against the published checksums**, and downloads into a temporary
  directory rather than the checkout it is about to lint.
- **The asset names in `.goreleaser.yaml` and the generated workflow are a contract**, because every
  scaffolded repository is pinned to the release it was written for. Both sides are resolved from their
  own file and compared, so renaming one alone fails the suite and a coordinated rename of both does
  not.

## Licence

- **AGPL-3.0-or-later.** The requirement was that the tool be open source and that nobody be able to
  freely make money from it. Those are in conflict: the Open Source Definition forbids discriminating
  against commercial use by name, so a licence that prevents selling is not an open source licence.
  Copyleft resolves it by making commercial exploitation unattractive rather than forbidden. Anyone may
  use ArchDoc commercially; anyone who redistributes a modified version, or runs one as a network
  service, must publish their source under the same terms, so nobody can take it closed and sell it.
- **Affero rather than plain GPL, because it costs the ordinary user nothing here.** ArchDoc is a tool
  that is run, not a library that is linked, so copyleft imposes nothing on the documents it manages or
  the repository it runs in. Section 13 binds only someone offering a modified version over a network,
  which a locally run CLI never does. It closes the one route that plain GPL leaves open, a hosted
  spec-management service built on a private fork, at no cost to anyone using the tool as intended.
- **Source-available licences were rejected for this project.** BUSL, FSL and PolyForm would forbid
  commercial use outright, which is closer to the literal requirement, but ArchDoc is meant to live in
  company repositories and scaffold their CI. A licence that corporate policy blocks defeats the purpose
  of the tool.
- **Single copyright holder, deliberately.** Selling commercial exemptions alongside the AGPL remains
  possible only while one party owns all the code. Accepting a contribution without a licence agreement
  ends that permanently.
- **The scaffolded `LICENSE` is unrelated.** `archdoc init --license=mit` writes a licence for the spec
  repository being created, covering that project's documents. It has no bearing on ArchDoc's own.

## Package layout and dependencies

- **`cmd/archdoc` is the only package that imports cobra**, and every other package is usable without a
  terminal, so a later desktop or TUI frontend calls them directly.
- **`internal/git` is an interface with a shell-out implementation**, returning a sentinel error when the
  directory is not inside a repository so a caller can tell that from a file being absent on a branch.
  It reads many blobs in one `cat-file --batch` and asks one `diff` which paths changed, because a
  subprocess per document was the whole of lint's running time on a repository of any size. Requests and
  responses are NUL-separated: git permits a newline in a path, and a line-oriented protocol pairs them
  wrongly when one contains one.
- **Markdown is handled with the standard library.** The dependency list is cobra, yaml.v3 and huh, with
  goreleaser at build time. Nothing else.
