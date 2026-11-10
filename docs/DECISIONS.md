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
- **A YAML syntax error is placed by re-parsing, not by yaml's line number.** yaml.v3 counts a scanner
  error's line from 1 and a parser error's from 0, gives the line a construct opened on for some errors,
  and none for the first line, so translating its number put a fault on line 4 at line 3. The finding
  goes on the first line from which the block, cut after that line or any later one, fails with the
  same message as the whole block. It is found reading from the end: a construct over several lines
  cut part way through can fail with the same message on an earlier line that is valid, and reading
  from the start put the fault there.
- **A value that did not parse is reported once.** Where there is no block, or it is not a mapping,
  no key was read, so none is reported missing; where one value failed to decode, L02, L07, L10 and
  L13 do not compare against the zero value left in its place. L02 and L10 also skip a `title` or
  `id` that is missing or empty, which L01 already reports. yaml's type errors are flattened onto the finding's line.
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
- **The defaults stay four constants, which `config.Keys` names rather than supplies.** `Load` and
  `init` apply the constants, and the table the reference prints refers to the same ones. Having `Load`
  apply defaults from the table meant matching each entry to a struct field by its JSON name through
  reflection, in place of four assignments. A test loads a file naming only `name` and checks every key
  against the table, which catches either side changing alone.
- **Each type's keys are one table, `repo.Keys`.** The parser's schema, L01 and the reference read it.
  L01 kept its own list of the keys that must carry a value, which the table would have repeated, so
  it now reads whether every key is empty and leaves which keys may be to the table. A test writes
  every key of every type empty and checks L01 reports exactly the keys the table says must carry a
  value, so a key the table marks with no way for L01 to see it empty fails.
- **Membership is held by the group, not by its members.** A frozen document can never gain a key, so
  a key on each document would leave every accepted document ungrouped for good, and could not follow a
  division that changes after the documents inside it froze. A group has no status and is never
  frozen, so regrouping is always an edit to the group. Each document's groups are derived, as every
  other reverse relationship is.
- **A group's member that is not an identifier resolves to a spec page and nothing else.** `Member`
  checks the identifier's shape before looking in `byID`, and looks only among spec pages otherwise. A
  term and a group are named by slug as a spec page is, and would otherwise answer for a page of the
  same name, which is the `ByPage` defect that let a term stand in for a spec page in the index.

## Markdown

The tool needs headings, comments, fenced blocks and links, and within two kinds of section,
blockquotes and list items. A full parser would be a liability when rewriting files in place, because it
would have to round-trip everything it parsed.

- **One walk per document.** The scanner that decides what is a comment, a fence or a code span is not
  only a shared *function* but a shared *traversal*: its masked output is cached on the document, and
  section text is a slice of it. Running the same function again on a fragment starts from a clean
  state and disagrees with the walk that decided the heading was a heading, which let a section
  containing only a comment read as full.
- **A comment is nothing, wherever it is.** `Section.Empty`, the glossary's paragraph split, heading
  parsing and link scanning all read the same masked text, so a comment on one line behaves like a
  comment spanning several. Comments are blanked in place rather than dropped, which keeps every byte
  offset valid and ~~lets prose sharing a line with a comment survive~~ (narrowed by #73) lets prose
  sharing a line with a comment inside a paragraph survive. The last line of a comment opening a block is the block's, as
  GitHub reads it: text after its `-->`, read as prose, had links checked and rewritten that nothing
  renders. The line is literal, as an HTML block's are, with that text kept in the masked line because
  it is still content, so a section holding it is not empty. Where text follows a lone carriage
  return on that line, which CommonMark reads as a line ending, only what is before it is the
  block's, and that is blanked instead, so the rest is read. ~~An unterminated comment runs to
  the end of the body, as a browser reads one.~~ Superseded by the next entry: only a comment opening
  a block does.
- **A comment is read where CommonMark reads one.** A `<!--` opening a block, ~~at most three spaces in,~~
  at most three columns in, a tab running to the next multiple of four, runs until a line holds `-->`, ~~or to the end~~ or until the blockquote or list item holding it
  ends, or to the end: run on past its container, it hid everything after a quoted `<!-- note`.
  Anywhere else it is raw HTML only where a `-->` follows before the paragraph ends, and text
  otherwise, as is one after a backslash. ~~The paragraph ends at a blank line.~~ It ends where
  CommonMark ends one, at a line blank inside its blockquote or one starting a block that interrupts
  it, such as a heading; ending only at a blank line hid a heading's links behind a `-->` on it. A
  paragraph holding a comment continues past it, and a code span wrapped across its lines holds any
  `<!--` in it. The rule that any unterminated `<!--` ran to the end hid every heading and link after
  a sentence mentioning the marker, and a code span wrapped onto a second line did the same, while
  GitHub showed them.
- **A heading and a table cell hold their own comments.** A `<!--` in a heading is a comment only where
  `-->` closes it on the heading's line: a heading is one line, and read as running on, a stray marker
  hid lines GitHub renders. A row is parted into cells before anything else, as GFM
  does, and each cell's comments and code spans are read in it, so a backtick in an earlier cell opens
  no span over a marker in a later one. A heading or a comment block takes a line a delimiter row
  follows, as neither is a table's header. `<!-->` and `<!--->` are complete comments, the `-->`
  sharing the `<!--`'s dashes, in the masker and in the patterns that read a comment whole for anchors.
  `<!` opens an HTML block only before an uppercase letter, as cmark-gfm has it, so `<!x>` is text in
  its paragraph.
- **A tab runs to the next multiple of four columns.** Counted as four wherever it fell, a tab after
  `- ` or `> ` read the item's text as an indented code block and hid its links.
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
- **Blockquotes and list items are read only where a section gives them a meaning.** A blockquote is an
  answer in a question section and a list item is an entry in a `Changelog`; anywhere else each stays
  text, so nothing about a document that carries neither section changes. ~~Their markers and
  indentation are not read anywhere else.~~ Superseded by the next entry, for the blocks they hold.
- **Code blocks and HTML blocks are literal, inside blockquotes and list items as well.** A fence
  indented under a list item, or behind `>`, was read as prose, so a `<!--` it held hid the rest of the
  document; an indented code block was prose, so `renumber` rewrote a wiki link in one and `link --suggest`
  linked text in one; and an HTML block's lines were headings and links GitHub does not show. The walk
  now follows containers by their markers and indentation, with lazy continuation, and treats indented
  code, the seven HTML block types other than a comment, and fences as literal: content, never prose.
  A backtick fence's info string cannot hold a backtick, so ```` ```go``` ```` opening a line is a code
  span. This is still not a parser: no inline structure is built, and setext headings are not read.
- **A blockquote in a question section may not hold a heading, a fence or a comment.** The walk finds
  each of those before it knows a line is quoted, ~~and does not see a fence inside a blockquote at all, so
  a comment marker inside one would hide the rest of the document~~ and an answer is read from its lines
  as written, which hold none of them as the rendered quote does. They are reported rather than read
  some other way.
- **Changelog entries are read in a subset of list syntax, and anything outside it is reported.** The
  subset is the shapes where this reading of a list item and CommonMark's agree: `- ` at column 0, then
  lines indented by two spaces, blank lines followed by indented ones, and unindented lazy lines directly
  after the entry's text that cannot begin another block. Hand-writing CommonMark's list rules was tried
  first and read several shapes differently from how they render.
- **Changelog lines are read as written, not masked.** Masking blanks a line holding only a comment, and
  a blank line continues a list item where a comment at column 0 ends it, so the masked text gave an
  indented line after such a comment to the entry above it. Section boundaries still come from the masked
  walk, and a heading that is one only once a comment on its line is masked is reported.
- **A pattern anchored at the end of a line accepts a carriage return before it.** A body is split on
  `\n`, so a line of a CRLF file keeps its `\r`, and a pattern ending in `[ \t]*$` read a file checked
  out by Git for Windows differently from its LF copy: link definitions went unread, so L17 never
  checked them, and a heading kept its closing hashes. The `\r` is accepted rather than stripped,
  because it is the line's last byte and accepting it leaves every offset where it was.
- **Masking never moves a byte.** Every masker writes spaces in place, so a match found in the masked
  text is at the same offset in the text as written, and an edit spliced by offset lands in the right
  column. This is the invariant the whole rewriting design rests on, and it is fuzzed rather than
  sampled.
- **Heading anchors follow GitHub, including the `-1`, `-2` suffix for a repeat.** Without the suffix
  L17 reports an error on links that work, because a page with two `## Configuration` headings has a
  real `#configuration-1` and that is what a reader copying from their browser will write. Anchors are
  assigned by walking a document's headings in order against a seen-set, never computed from a heading
  in isolation, and are slugged from the heading's *rendered* text so link syntax is stripped first.
  ~~Stripping inline links was all the rendering done.~~ Rendering now resolves escapes and entities,
  keeps a code span's content, drops tags, emphasis underscores and images, and reads reference links
  and nested brackets, and the slug keeps the marks, letter numbers and connector punctuation
  github-slugger keeps. Each was a heading whose link worked on GitHub and failed L17, or the reverse,
  and `link` wrote the wrong anchor into a glossary link. An underscore run is emphasis only where it
  pairs, the runs of `*`, `_` and `~` paired as cmark-gfm pairs them: dropping every run that flanked a
  word gave `## The _id field` the anchor `the-id-field`, where GitHub's is `the-_id-field`. A bare
  address GitHub links is text as written, its end read as cmark-gfm's autolink extension reads it,
  since an underscore in `https://a.com/_x_` is not emphasis there. A
  reference link is a link only where the document defines its label, and text otherwise. A link in a
  heading closes where one in prose does, its destination and title read alike and a bracket in a code
  span in its text not one of its own: closing the parentheses at their balancing `)` linked
  `[a](x y.md)`, which GitHub renders as text, so L17 rejected GitHub's anchor. A heading is
  rendered as written, so a comment in it leaves the space beside it. Of the numbers, the slug keeps
  decimal digits and letter numbers and drops the rest, such as `²` and `½`, as github-slugger does.
- **A link names an anchor a document holds explicitly as well as its headings'.** L17 read `<a id>`
  only in files that are not documents, so a document's own explicit anchor was a target nowhere.
  Explicit anchors are read outside comments and code, and in HTML blocks, in single or double quotes.
  A comment within an HTML block is unmasked, so it is removed before an anchor is read from the block,
  and an anchor's entities are resolved, as a browser reads both.
- **Link destinations are read in every form CommonMark allows and written in one.** `Document.Links`
  understands a bare destination, the `<...>` form and a link reference definition, and percent-decodes
  before comparing against a path on disk. `repo.LinkDestination` percent-encodes on the way out. The
  two are inverses, and encoding rather than escaping avoids the interaction where a path's own
  backslash pairs with an added one and lets the destination close early. What follows the destination
  is a title in `"..."`, `'...'` or `(...)` or nothing, and an empty destination is one, as cmark-gfm
  reads them: anything up to a `)` was taken as a title, so `[a](x y.md)`, which GitHub renders as
  text, was a link to `x`.
- **Link text is read across a paragraph by a scanner, not a pattern.** A pattern matched one line at a
  time with `[^\]]*` for the text, so link text wrapped across lines, a linked image and brackets in the
  text were not links, and L17, `export`, `term remove` and `renumber` all passed over them. The scanner
  keeps the brackets left open across a paragraph's lines, as CommonMark's bracket stack does, and reads
  the masked view, so a bracket in code or a comment is not one. A link sits on the line holding its
  destination, which is where a writer edits it. A reference link ends the link around it as an inline
  one does, so the document's definitions are read before its links; without that, `[RFC [1]](x.md)`
  beside a definition of `[1]` was read as a link to `x.md`, which GitHub does not render. Each block
  that ends a paragraph ends the text, since text carried into the next block made a link of a `](`
  whose `[` was in a heading, a table row or another list item. ~~Whether a line opens a list item is
  the block walk's to say, as it depends on the containers around the line: a pattern read `2) ` as
  an item anywhere, and so left readable a link inside a code span crossing it.~~ Superseded, as a
  pattern still decided headings and table rows: whether a line continues the paragraph before it is
  the block walk's to say, for every block, as each depends on the containers around the line. A
  pattern read `2) ` as an item anywhere, which left readable a link inside a code span crossing it,
  and took a heading or a `|` line at any indentation, so link text wrapped onto `    # x`, or onto a
  `|` line starting no table, was not read. A line starts a table only as the header above a
  delimiter row of as many cells, in a paragraph with no line above a delimiter row of another number,
  as in GFM, and each row and cell is read on its own, so link text, a code span or a comment does
  not run from one cell into the next. A line in a paragraph is blank only as written, so one holding
  nothing but a comment indented four columns, or nothing but part of a comment opened earlier in the
  paragraph, continues it, for L15 as for links, and a delimiter row on a line such a comment holds
  still marks the paragraph, as GFM reads blocks before comments.
  The scanner and the code span masker read the walk's answer rather than a rule of their own.
  Section boundaries do not: a heading is read from the masked line there, so `#<!-- c --> x` opens a
  section while the scanner reads it as text, and L21 reports the difference at a `Changelog` section.
  L15 reads such a line as written, as the scanner does, so it is text in a definition.
- **Reading links is linear in the length of a line.** The first scanner ran the destination pattern
  from every `]` to the end of the line when no `)` followed, and made every open bracket inactive one
  at a time when a link closed, so a 60 KB line took ten seconds. The pattern is not tried after a
  line's last `)`, inactivity is one bound on the stack rather than a flag on each bracket, and a
  label's length is measured only as far as the longest label can run.
- **A paragraph's end is found once for the paragraph.** Each comment opened inside a paragraph walked
  to the paragraph's end to learn whether it closed, and each code span left open before such a comment
  walked there again, so a paragraph of lines each opening a comment was quadratic in its length.
  `scanBody` finds the end once and every line before it reuses it; a line at or past it is in another
  paragraph~~, and a line leaving no paragraph open discards it, since a comment in a heading asks for
  the end of a paragraph that is not there~~. A comment in a heading no longer asks: it closes on its
  line or is text. The first line from each line on holding a `-->` is found once for the body, so asking
  whether a comment closes does not reread the lines of its paragraph.
- **A line's markers share one reading of the line.** Each `<!--` on a line asked whether the rest of
  the line closed it, where the next code span began and which spans before it were left open, each
  read from the line's start or to its end, so a line of 40,000 markers took twenty seconds, and three
  minutes in a heading, whose anchor ran a pattern to the end of the text for each marker to find no
  close. The line's last `-->` and its backtick runs, each with the end of the run closing it, are
  found once; the next marker and the next span are positions kept until passed. The first span left
  open on the line is found for the first marker to ask, after `closes` has answered, and not sooner:
  `ends` reads the walk's state when it is first asked and the answer is kept for later lines, so
  asking it on a line that never asked before would move where a paragraph ends.
- **A line is a link reference definition only if that is all it is.** The pattern is anchored at both
  ends, requires a non-empty label, allows a title and does not count a line that interrupts a
  paragraph. Without the end anchor a footnote, a numbered citation or any sentence beginning with a
  bracketed word was read as a link and reported as a broken one, and a citation list is exactly what a
  `Sources` section looks like. A heading and a code or HTML block end a paragraph as a blank line
  does, so a definition directly under `## Sources` is read; it was not, so L17 never checked it and
  `renumber` never rewrote it.
- **L17 judges a path against the git repository, its case exactly.** A link from a specification
  beside the code it describes to that code leaves `root` and stays in the repository, and was
  reported as leaving it. A path differing only in case was found on a case-insensitive filesystem and
  is missing on GitHub and in a Linux checkout, so each component is matched against its directory's
  listing. A destination with a URI scheme or a leading `/` is not relative and is not checked.

## Sections

- **One required-section list, read by everything that asks the question.** `Document.RequiredSections()`
  adds `Sources` when a document is backfilled and `Rejection rationale` when it is rejected, and L08
  and L14 read it, and the pre-freeze gate through them. Written against separate lists, they disagreed:
  a backfilled document whose `Sources` held only the template comment satisfied both, because one checked presence
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

- **Freezing is gated at the transition, not after it.** `accept` and `reject` refuse; `withdraw` gates
  nothing, because a document abandoned before a verdict is empty by nature.
- **The gate refuses every error lint reports about the document.** What it lets through, L11 then
  forbids anyone repairing, so a mismatched H1 or a missing `Changelog` became a permanently red
  repository with no in-tool remedy. The rules themselves are run rather than restated, so a rule added
  later is gated with no second change, and every error is listed in one refusal. The document is judged
  in a copy of the repository holding it as the transition writes it, because the transition changes what
  lint says: accepting a proposed document that carries a `decided` date clears L07, and L21 compares the
  Changelog's dates with the `decided` date about to be written, where the document's own is empty while
  it is proposed. A rejection is judged with its rationale taken as written, since the section `reject`
  appends holds only a prompt, which L14 reports as empty. L03 is left out, because renumbering the other
  document corrects it once this one is frozen. A fault in a relationship is refused although editing the
  other document would clear it, so that the transition's own commit does not leave lint failing.
- **The gate began as its own list of checks.** It checked the H1, the required sections' presence, order
  and emptiness, unresolved `[[...]]`, and what L20 and L21 report, reusing the accessors the rules use,
  and stopped at the first fault. It gained a check that the closing section is last once `Changelog` and
  `Open questions` stopped being required: while they were, the order check kept them before `Sources`,
  and L08 reports a heading after `Sources` with an error that does not soften. It never looked at the
  front matter, so an unknown key or a `depends` naming no document was frozen, and a document with
  several faults was refused once for each. Running the rules replaced it.
- **A refusal carries lint's findings, not a rendering of them.** `lifecycle` returns them in its error
  and `cmd/archdoc` prints them as `lint` does, so a frontend without a terminal receives findings it can
  present, and a refusal reads the same as the run of `lint` that would follow it.
- **Open questions are refused before lint is asked.** `accept` refuses a non-empty `Open questions`
  section naming `Resolved questions` as where an answered question goes, which L09's finding does not,
  and stops there, so a document with questions still open is refused for those alone.
- **A lint error must name something the user is permitted to change.** A finding that no permitted edit
  to the named document could clear is a warning: ~~L14, L16, L17, L20 and L21, whose subject is the
  document's own body. It does not extend to a fault in a relationship between documents, which is
  cleared by editing the other document, nor to L11 itself.~~ Superseded by ADR-0023, under which a
  finding no permitted edit could clear is a warning whatever it concerns: L01, L02, L07, L08, L09, L10,
  L14, L16, L17, L20 and L21 in a frozen document; L04 and L05 where only an edit to a frozen document
  could clear the fault, as with a ref in `depends`, a reference that is not an identifier any
  document can hold, a type mismatch, or a frozen target that is not accepted; and L03 when every
  document holding the identifier is frozen. A relationship fault another permitted edit can clear stays
  an error: a missing target named by a valid identifier can be written, a target not yet frozen can
  still be edited, and an identifier held by an editable document can be renumbered. L11 stays an error.
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
- **`--created` has no default under `--backfill`.** It once defaulted to today, which is never the date
  of a decision taken before the repository existed, and the document is frozen with whatever date it
  carries. At a terminal the status and dates are prompted for; without one, a missing `--created` is a
  usage error. The prompts join the form for type and title, which the interaction rule makes one form,
  so decided cannot be pre-filled with a created date typed into the same form: it is left empty to mean
  that date.
- **What the command line gives `new` is checked before the form.** The type, the title and the backfill
  flags were once checked after it, so a wrong flag was refused only once the form had been filled in and
  its answers lost. Each prompted answer is judged by the same function as it is typed, against the
  values already given.
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
  front matter no longer reads correctly. `term add` and `term rename` parse the term file they built
  and refuse unless its title and H1 read back as given, and `add` unless the
  definition reads as one paragraph. Values are quoted through the YAML encoder, and the read-back is
  what holds that to be enough: written bare, `Foo: bar` did not parse, `x # y` read as `x`, `#tag`
  as nothing, and a former name holding a comma became two, so its anchor vanished from GLOSSARY.md.
- **A term's definition is read as blocks, to the end of the file.** Splitting on blank lines read a
  heading, list or fence written straight after the definition as part of it, and GLOSSARY.md printed
  the heading as a second term. The blocks come from the document's own walk, and ~~a paragraph ends
  where the link scanner's blockStartPattern ends one, at a setext underline and a table row as well,
  so the two agree on where a paragraph stops.~~ Superseded, as the two did not agree: the link
  scanner ~~takes~~ took a heading or a table row at any indentation, since it ~~does~~ did not know
  the indentation of the list item holding a line, so `    | x` after a definition's line ended the
  paragraph for links and continued it for L15, which is what CommonMark does. A paragraph ends at a
  setext underline ~~and a table row as well,~~ indented at most three spaces as a block start at the
  top level is, and at a table's header row, by the block walk's own test. The table row is struck because a
  `|` line with no delimiter row beneath it is text in GFM, and L15 reported it. A lone carriage
  return ends a line there, as it does in CommonMark and in what is published:
  read as part of its line, `Def.` and `More.` with a line feed and a carriage return between them
  passed L15 as one paragraph and were published as two. A comment marker inside a code block is
  code, and taking it as an open comment miscounted every block after the fence. Prose above the H1
  is reported rather than dropped, since nothing publishes it. What is published is every line from
  the H1 to the end of the file as written, ~~with its comments cut out,~~ comments included, rather
  than the paragraphs alone: L15 reports a block that is not a paragraph, but export never fails on it, and publishing
  only the paragraphs dropped the block from GLOSSARY.md, `term show` and the export unseen.
  Printing the masked text showed the blanks a comment left. ~~A comment on one line that is all that
  keeps it from being a heading, a thematic break or an underline stays, since cutting it published a
  block L15 had read as text. The decision is made a line at a time, so a comment spanning lines is
  still cut.~~ Cutting a comment changes what surrounds it: `Def.` above `*** <!-- c -->` passed L15
  and published a thematic break, and a comment spanning lines could do the same, so comments are
  published with the definition and `term show` and the export carry their text. A comment block
  left open at the end of the file is the exception: `GLOSSARY.md` would read every term after it as
  part of the comment.
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
  `archdoc.json` behind that `init` then refused to overwrite. `INDEX.md` and `GLOSSARY.md` are
  generated once the rest exists and are planned with it all the same: the glossary was not, so `init`
  replaced an existing one, and refused a symbolic link there only after writing everything else.
- **`link` leaves a document entirely untouched if any of its wiki links resolves to nothing.** A
  partial rewrite would leave the file in a state neither the author nor the tool intended. Every other
  document is still written, because the failure belongs to one file.
- **`renumber` follows links by resolving them.** It found them by searching each body line for the
  renamed file's name, so it rewrote a link to a same-named file in another directory, which an RFC and
  the ADR extracted from it commonly are, along with a mention in prose or a line of code, and a frozen
  document holding any of these refused the rename. It now resolves the links lint reads, from the
  masked view, against the directory of the document holding each, and rewrites a destination in place
  by its columns, so the directory and fragment as written survive. Wiki links are read from the same
  view. The moved document's own references are followed too, since a link to itself would otherwise
  name a file that is gone. A link the parser does not read, such as one whose text wraps across lines
  or holds a `]`, is not followed, where the text search caught it among everything else; renumber
  follows what lint checks. A destination whose file name as written does not decode to the old one,
  because an encoded `/` or `#` moves it, is refused rather than rewritten into another destination.
- **`renumber` leaves an identifier another document still claims.** When two branches each take the same
  next number, renumbering one of the two rewrote every reference to the identifier, including those
  meant for the document keeping it, and a frozen document naming it refused the rename although its
  reference might have meant the document keeping the number. An identifier in a front matter list or a
  wiki link cannot say which document it means, so neither is rewritten or blocks; each is listed for
  the author to repoint, and marked where the document is frozen. A link to the file names one
  document, so it is still rewritten and a frozen one still blocks.
- **`renumber` undoes a rename that fails part way.** It wrote the other documents first, in map order,
  then the new file, then removed the old one, and a failure at any step left referring documents naming
  a number no file carried. It now refuses a target path that already exists, writes the new file first,
  writes the edits in path order, and on a failure writes each edited document's original back and
  removes the new file. Checking every write in advance was not enough on its own: whether a file can be
  written is known only by writing it. The new file is created with `O_EXCL`, and is removed only once
  created, so the undo never removes a file it did not make. A document is recorded for the undo once its
  write succeeds, and a restore or removal that fails is reported with the original error.
- **A file is replaced by renaming a complete copy over it.** Every writer that replaces a file writes the
  bytes to a temporary file in the same directory and renames it over the original, keeping the
  original's permission bits, and removes the temporary file on any failure. `os.WriteFile` truncated the
  original first, so a write failing part way, at a size limit or on a full disk, left a document cut
  short, and the undo that wrote it back failed the same way. The original is opened for writing, without
  truncating it, before anything is written, so a read-only file is still refused although the rename
  needs only the directory to be writable. A writable file in a read-only directory is refused too, where
  `os.WriteFile` wrote it, because the temporary file cannot be made there; writing in place for that case
  alone would bring back the truncation for exactly the files the rename was meant to protect. Every
  failure names the file asked for rather than the temporary file, which the user never asked for. The
  rename makes a new file, so a hard link to the original keeps the old contents, and the owner and group
  become the writer's rather than the original's; only the permission bits are carried over.
- **Interactive suggestion application re-reads before writing.** The snapshot is taken before the first
  prompt and the write happens after the last answer, so the whole session is a window in which an edit
  made elsewhere would be silently discarded. A file that has moved on is left alone and said so.
- **`archdoc init` followed by `archdoc lint` and `archdoc index --check` exits 0.** A scaffolded
  repository must pass the workflow `init` itself writes, which is why `INDEX.md` is generated rather
  than templated ~~and why the glossary template may carry a preamble and no entries~~. Superseded by
  [RFC-0007](../spec/rfc/0007-the-glossary-is-a-type-of-its-own.md): there is no glossary template, and
  `GLOSSARY.md` is generated from `term/` as `INDEX.md` is from the documents.

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
- **`init` and `update` print every path they write**, because `init` can write outside the current
  directory and that is surprising without a record of it. Each is printed from the working directory,
  or absolute where it lies outside it. They were printed from the directory each file is resolved
  against, the configuration's, `root` or the git root, so run anywhere else a printed path named
  nothing.
- **`archdoc.json`'s keys are checked as written.** `encoding/json` matches a key to a field without
  regard to case and reads `null` as leaving the field alone, so `{"NAME": "x"}` loaded as `name` and
  `{"strict": null}` as the default, while the layout page said unknown keys are an error.
- **`branch` must be a name git accepts as a branch~~, and not `@`~~.** An empty one failed rendering the
  workflow, and one holding `": "`, `" #"` or a line break rendered an invalid or truncated one; git
  refuses every such name. ~~`@` is a valid branch, but the name reaches `rev-parse` bare, where `@` is
  `HEAD`. The names git gives its own refs, such as `FETCH_HEAD` and `ORIG_HEAD`, are valid branches
  that `rev-parse` also resolves to something else, and are not refused.~~ Resolved by naming the
  branch to git in full: `@`, `FETCH_HEAD` and every other valid branch name are read as the
  branch. The rules are applied lexically rather than by asking git, which keeps `internal/config`
  free of a repository and works where `init` runs outside one.
- **`README.md`, `PROCESS.md` and `INDEX.md` are not documents** under the discovery rule, so no rule
  examines them, L17 included.

## Commands and interaction

- **Prompting substitutes for values the command was not given, and for nothing else.** A prompt is
  offered for required arguments and for every optional flag whose value reaches a file: `archdoc.json`,
  `LICENSE`, or a document's front matter. Flags that select a mode of operation write nothing, and
  prompting for them would ask the user to re-specify the command already typed. The form is built field
  by field from what the command line settled, so a fully specified invocation from a terminal writes
  immediately rather than rendering a form and waiting.
- **`as_of` is asked for only in a form shown anyway.** It reaches a ref's front matter, but most refs
  are not records of an exchange, and `new ref "Title"` from a terminal writing immediately is worth more
  than a form for a field almost always left empty. Where the type or title is asked for, it is a second
  group, hidden until the type is a ref. Whether to add that group is decided before the type select is
  built: binding a huh select writes its first option into the bound variable at once, so a check made
  afterwards read every prompted type as an RFC and the field never appeared.
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
- **`link --suggest` asks its questions on stderr.** Under `--json` stdout is owed one JSON array, and
  with stdout redirected the person answering is reading the terminal, which stderr still reaches. The
  questions were on stdout, which put them in the redirected file ahead of the array.
- **`--json` without `--check` is a usage error on `index` and `glossary`.** `--json` shapes the
  findings of `--check`, and there are none without it. The flag was ignored, so a script asking for
  JSON was given the name of the file written.
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
- **A wiki link in a heading is refused, not expanded.** Expanding it changes the heading's text, which
  is what L10, L08 and L09 compare, so `link` reports it as an error naming the edit, `new` refuses a
  title holding one, `term add` and `term rename` refuse a name holding one, and L16 gives the same
  remedy, except in a frozen document, where L11 forbids the edit and L16 says it is left as written.
  `link` had stored every document holding a `[[...]]` as changed and reported it resolved, when the
  heading was left as written.
- **The first occurrence counts the links already written.** The first occurrence was chosen among
  bare mentions only, so each `--apply` linked the next mention of a target already linked, and
  `--suggest` settled only after as many runs as there were mentions. A reference link counts where
  it is used, since its definition renders nothing, and a link to a term's own file counts as one to
  the term.
- **The masker is deliberately generous, and withholds a line it cannot read.** It blanks wiki links,
  inline and reference links, link reference definitions, any bracketed span, bare URLs, autolinks and
  raw HTML; and it withholds the whole line when a raw HTML tag is present or the brackets are
  unbalanced after masking, because both mean a construct whose extent a line-at-a-time masker cannot
  see. Over-masking costs one missed suggestion; getting it wrong splices a link inside a link, which
  CommonMark forbids, and demotes the author's link to literal text with nothing able to see it
  afterwards.
- **A term reaching a link label is escaped**, as a title is, so a term carrying a bracket or a pipe
  cannot put live markup into a document its author never touched.
- ~~**A glossary entry ends where the walk that found it says.** Deriving the extent from the next entry~~
  ~~instead runs straight through an ordinary H1 section sitting among them, and removing a term then~~
  ~~deleted it.~~
- ~~**The glossary may carry a preamble and may have no entries.** Anything before the first H2 is~~
  ~~preamble and is ignored; entries are inserted after it. Arbitrary preamble is permitted rather than~~
  ~~comments alone, so the glossary is not the one spec page where a lead sentence is illegal.~~
- ~~**`Formerly` lines accumulate**, one per line after the definition paragraph, so repeated renames leave~~
  ~~the full chain. They are recognised line by line rather than as a blank-line-separated block, and~~
  ~~paragraphs split on a pattern that matches CRLF.~~
- ~~**`rename` is remove then re-insert**, which re-sorts the entry for free and keeps one insertion~~
  ~~routine. The entry's own lines are carried across verbatim; rebuilding it from its parsed paragraphs~~
  ~~deleted comments and reflowed fenced blocks.~~
- ~~**`term rename` does not rewrite links elsewhere, and `link` cannot repair them.** `link` expands~~
  ~~`[[...]]` and never edits an existing markdown link, and a recorded former name is not a term, so~~
  ~~`[[<old name>]]` does not resolve either. L17 reports them and they are corrected by hand.~~
- The five entries above described the glossary as one page of H2 entries, and are superseded by
  [RFC-0007](../spec/rfc/0007-the-glossary-is-a-type-of-its-own.md), under which each term is a file
  under `term/` and `GLOSSARY.md` is generated from them. What replaces them:
- **A term's former names are front matter.** `formerly` lists them oldest first, so repeated renames
  leave the chain, and `GLOSSARY.md` gives each an explicit anchor before the term's heading. A rename
  leaves out a former name whose anchor is the new name's, as a change of case or a return to an earlier
  name gives, since the heading answers it.
- **`term rename` edits the term file field by field and moves it.** It sets `title`, appends the old
  name to `formerly`, retitles the H1, and writes the file at the new name's path before removing the
  old one, or in place where the slug is unchanged, as in a change of case; the rest of the file is
  carried across as written.
- **A link to a former name's anchor keeps resolving; nothing else is rewritten.** A link to
  `GLOSSARY.md#<old name>` resolves through the anchor generation keeps for it. A link to the term's old
  file under `term/` does not, nor does `[[<old name>]]`, since a former name is not a term; L17 and L16
  report them.
- **With no `term/`, `glossary --check` accepts an absent `GLOSSARY.md`.** A repository with no `term/`
  has no glossary, and the check demanded the empty page `archdoc glossary` writes, so removing `term/`
  and the page failed the workflow `init` scaffolds. A page that is there, including a symbolic link
  that resolves to nothing, is still held to what `archdoc glossary` writes.

## Linting

- **Every rule is a function returning findings; nothing prints or exits.** The registry is the single
  source of a rule's code, so the rule functions do not repeat it on every finding they build.
- **A rule's title, summary, remedy and releases are on its registry entry.** The triage guide's table
  was prose beside the registry, and is now the registry's text, compared by a test and rewritten by
  `-update-triage` rather than generated at build time, so `archdoc agents`, `update` and the release's
  skills keep copying a static file. ~~Changes made to a rule before the registry recorded them are not
  reconstructed~~ Superseded by
  [RFC-0018](../spec/rfc/0018-the-registry-records-every-change-since-v0-5-1.md). Changes in a
  release tagged before the registry, v0.5.1 and earlier, are not reconstructed: each would be read
  off the code at each tag, and a wrong one would be published. Changes made for v0.6.0 before the
  registry existed are recorded, since that release was not yet tagged and its changes are read off
  one diff against v0.5.1.
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
  clean. The diff runs from the repository root with plain paths. Run from the directory holding
  `archdoc.json`, pathspecs and `diff.relative` both resolved against that directory; a `:(top)` prefix
  and `-c diff.relative=false` covered the two sides until `GIT_LITERAL_PATHSPECS` read the prefix as
  part of the name, and from the root neither is needed. git is given `GIT_LITERAL_PATHSPECS=1`, and
  every inherited pathspec setting is removed, since git refuses the literal one beside the glob or
  icase one: read as a pathspec, a root beginning with `:` was magic, matched nothing, and no frozen
  document read as changed.
- **L11 reads which documents are frozen from the branch's tip, and what changed from the merge base.**
  Compared with the tip, a branch cut before a document was accepted there held it as it was, or not
  at all, and L11 reported it changed or deleted although merging leaves it as the branch has it; a
  pull request's check re-run after the branch moved failed the same way. Frozen is still the tip's
  answer, so a document accepted there since the cut and edited on the branch is reported, because
  merging would change it. A deletion counts only where the merge base held the document. Where HEAD
  and the branch share no commit, as in a clone of depth one that fetched the branch, the rule fails
  naming `fetch-depth: 0` rather than falling back to the tip, which would report every document
  frozen there since.
- **L11 reports a frozen document only where it differs from the tip as well.** ~~What changed is the
  merge base's answer alone.~~ A branch stacked on one whose acceptance was squash-merged, or one that
  cherry-picked it, holds exactly what the tip does, so merging changes nothing, but its merge base
  still held the document before it froze: the push run failed while the pull request's passed.
- **Each side is read under its own root, from its own `archdoc.json`.** The working tree's `root` was
  applied to the branch, so a pull request moving the root matched nothing there and passed every
  edit and deletion. Documents are matched by their path within root, which follows a move and still
  reports an edit or a deletion made with it; a file that moved is compared by the object id git
  computes for it through the same filters a commit applies. ~~A branch's `archdoc.json` is the one at
  the working tree's location, or the branch's only one, so moving the file is still judged by the
  branch's `strict`; with several, which one moved cannot be told.~~ (An `examples/archdoc.json` beside
  the record was taken for a moved file, and its documents reported deleted.) ~~A branch's
  `archdoc.json` is the one at the working tree's location, or the one the working tree's moved from:
  the branch's only one the working tree no longer holds, so moving the file is still judged by the
  branch's `strict`, and one still there belongs to something else.~~ (Deleting an
  `examples/archdoc.json` while adopting ArchDoc at the top level was taken for a move, and the
  example's documents reported deleted.) A branch's `archdoc.json` is the one at the working tree's
  location, or the one the working tree's moved from: the branch's only one the working tree no longer
  holds, where the branch holds no document under the root it names, or one of them is still under the
  working tree's root. Moving the file is still judged by the branch's `strict`, one still there
  belongs to something else, and one whose documents are none of them under the working tree's root
  was not the record's. With several gone, which one moved cannot be told. Absence is the test rather
  than git's rename detection, which sees only files git tracks and so never a working tree's
  `archdoc.json` not yet added. The branch is listed whole, not the
  working directory's subtree, so a file or root above the directory archdoc runs in is found.
  ~~Where the root the branch names is the working tree's, git is asked how it spells it, as a case,
  normalisation or symbolic link difference matched nothing.~~ (A pull request correcting the root's
  spelling matched nothing on the branch, and one leaving a link where the root was sent the branch's
  documents to the link's target.) ~~Where the branch's file is at the working tree's location and the
  branch lists nothing under the root it names, git is asked how the working tree spells that root, as
  a case, normalisation or symbolic link difference matched nothing; a root the branch lists files
  under is taken as written.~~ (A moved file's root, relative to where the file was, was taken as
  written, so one in another case than git lists it matched nothing and the pull request's edit to a
  frozen document passed.) Where the branch lists nothing under the root its file names, git is asked
  how the working tree spells that root if the file is at the working tree's location, as a case,
  normalisation or symbolic link difference matched nothing. Failing that, and for a moved file, the
  root is the directory in the branch's listing that matches it case-folded and in Unicode
  normalisation form C, the first in git's order where several do; the listing is the branch's, so the
  match holds whether or not the filesystem tells the spellings apart. A root the branch lists files
  under is taken as written. A branch's `archdoc.json` that cannot be read leaves the working tree's
  settings standing, with one L11 warning: an error holds until the branch is mended, so the pull
  request mending it could never pass. `branch` is the working tree's: the base is found through it, so no
  other copy can be trusted to name it, and a pull request could edit the workflow instead.
- **The branch is named to git in full.** A bare name resolves to `$GIT_DIR/<name>`, then a tag, before
  `refs/heads/<name>`, so a tag named like the branch, or a branch named `FETCH_HEAD`, had L11, the
  merge rules and `changes` judge against the wrong commit. The branch is read as
  `refs/heads/<branch>`, or `refs/remotes/origin/<branch>` where there is no local one, and a commit
  such as the merge base is passed as given.
- **A missing branch is two different situations.** With no commits at all nothing can be frozen, so the
  rule is skipped with a warning: `archdoc init` followed by `archdoc lint` is the first thing anyone
  does. With commits present a missing branch means the configured name is wrong, and passing quietly
  would disable the freeze check altogether.
- **Only a type with a lifecycle can be frozen.** A status key can be read out of any committed blob, so
  a spec page or a ref carrying one by mistake was treated as frozen and every later edit to it became
  an error on a type that has no lifecycle at all. The same holds for a document deleted from the
  working tree, which L11 reads from the branch: it was checked for a terminal status without asking
  its type, so a group committed with a stray `status: accepted` was reported as frozen once removed.
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
- **A merge rule is a registry entry whose code starts with M, skipped unless the context carries a
  base.** The base is read up front by the command that asked for it, so a missing branch is a usage
  error before any rule runs, and the transition commands, which run lint before writing, never pay
  for reading the branch. Marking the rules by their code rather than a field leaves the registry's
  shape alone.
- **The base is read whole, in one batch, separately from the branch snapshot.** The snapshot covers
  only paths in the working tree, and the merge rules need the documents that are gone from it too:
  a rename is found by identifier at a path the working tree no longer has, and M04 is about a
  document deleted.
- **The base is the merge base of HEAD and the branch, not the branch's tip.** The tip moves after a
  pull request's merge commit is made, so a re-run of an old check read everything merged since as
  deleted here, and M04 failed it. A branch not rebased since it was cut hit the same, locally and on
  any host that checks out the pull request's head rather than a merge. The merge base is the branch
  as the merge was made in the first case and the fork point in the second; `HEAD^1` was GitHub's
  case alone, and wrong where the branch's own tip merges the branch in.
- **The transition graph is a method on the status.** M03 judges by the same table as the transition
  commands, and `lifecycle`, where it was, imports `lint`.
- **M03 reads `strict` from the base**, so a pull request turning it off is still judged with it on.
  Where the base has no `archdoc.json`, as before the pull request that adds ArchDoc, the working
  tree's is the only setting there is. ~~The base's is read at the working tree's location.~~ It is
  found as L11 finds it, so moving the file does not escape it.
- **Acknowledged warnings are left out by `Run`, and L22 is computed there.** L22 judges what the
  other rules report before any is left out, so `Run` applies every other rule, then separates the
  acknowledged warnings and the stale acknowledgements in one pass. L22's registry entry has no
  `Check`: `Run` is the only caller of one, and a check of its own would have to run every other
  rule again, from an initialiser of `Rules` that refers to `Rules`, which Go refuses.
- **With no branch, every acknowledgement is stale.** Outside a git repository or before the branch
  exists nothing is frozen, so nothing is acknowledged and L22 reports each entry as not frozen.
  Every other rule reads "no branch" as "nothing frozen" through the same `Frozen`. A fault that is a
  warning only because its document is frozen is an error there anyway, so the exit status is the
  same with or without the entry. A rule that warns on any document, as L18 does, exits 0 without the
  entry and 2 with it. Treating it as unknown would add a state that holds for L22 alone.
- **`archdoc init` does not write `acknowledged`.** The field is left out of the file where empty. A
  release before the one reading the key refuses it as unknown, and a repository acknowledging
  nothing must not fail under such a CI pin.
- **The export carries `acknowledged`**, empty where `archdoc.json` has none, since `config` is every
  setting the file holds and its schema refuses a property it does not declare. Adding a field does
  not change `schema`.

## The index

- **`INDEX.md` is generated, not templated**, so a scaffolded repository passes the `index --check` its
  own workflow runs.
- **All four sections are always emitted** with their header rows, so the file does not change shape as
  the repository grows and `index --check` is stable immediately after `init`.
- **The `Groups` section is emitted only where there are groups.** Always emitting it would change the
  index of every repository on upgrade, so `index --check` would fail in every one until it was
  regenerated, for a section most repositories never use.
- **Every cell is escaped, and every destination is encoded.** One renderer percent-encodes each
  character that would end a destination, a link or the table cell around it, and one escape function
  handles labels. A filename and a title are both free text reaching a file headed "Do not edit".
- **`Stale` renders `yes` or empty**, following the "empty cells are empty" convention. A column of `no`
  is noise.
- **A status carries its qualifiers in parentheses**, comma-separated in a fixed order, generalising the
  single obsolete case rather than special-casing a second one.
- **A unified diff is implemented here** rather than adding a dependency, because `index --check` is
  specified to print one and an index is a few hundred lines.

## The export

- **`documents` is always an array**, empty where there are none, as in a freshly initialised
  repository. A nil slice encoded as `null`, which the schema refuses.
- **A status outside the lifecycle is omitted, as an invalid date already was, and so is a
  relationship entry that is not an identifier.** The schema's enum is the lifecycle's statuses and its
  pattern for `depends`, `updates`, `obsoletes` and `includes` is an identifier's, and values such as
  `approved` or `nonsense` were published as written, outside them. Lint reports each; the export
  publishes only what its schema allows.
- **`glossary` is an empty array where `term/` holds no terms, and absent only where there is no
  `term/`.** `has_glossary` says which, and `--out` writes `glossary.json` whenever it is true. The
  field is `omitzero` rather than `omitempty`, which drops an empty slice along with a nil one: a
  `term/` holding no terms exported `has_glossary: true` and no `glossary`.

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
- **A rule's `since` is checked against the tag being released, and only there.** No other run knows
  which release it is building. The release workflow passes the tag as `ARCHDOC_RELEASE_TAG`, and a
  second test checks it does, since a missing variable skips the check without failing. Versions are
  compared by semantic version precedence in the test rather than through a module.
- **The docs bundle and `install.sh` are written by a goreleaser `before` hook**, so they are checked
  before anything is published and listed in the `checksums.txt` goreleaser writes. Written after
  publishing, they would mean replacing the file `archdochq/setup` downloads.
- **Every entry in the bundle is dated the Unix epoch**, so one commit and tag always give the same
  bundle.
- **A guide's links are read with `internal/repo`**, as a page whose body is the whole file, so the
  generator reads links exactly as lint does.

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
  The sentinel is chosen from git's message, not its exit status: git exits 128 as well for a repository
  it refuses to read, one owned by another user as a checkout mounted into a container is, or under a
  broken `~/.gitconfig`, and L11 skipped itself at exit 0 there. git runs with `LC_ALL=C` so the message
  is English, and with `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR` and `GIT_INDEX_FILE` removed: git
  exports `GIT_DIR` alone to hooks and `rebase -x` in a linked worktree and then takes the working
  directory as the top level, so no document matched the branch. It reads many blobs in one
  `cat-file --batch` and asks one `diff` which paths changed, because a subprocess per document was the
  whole of lint's running time on a repository of any size. Requests are NUL-separated, because git
  permits a newline in a path and a line-oriented protocol pairs them wrongly when one contains one.
  `-z` terminates requests only, and a missing path is answered by echoing it, so that answer is matched
  against the whole request. `-Z`, which terminates both, needs git 2.43.
- **`cmd/archdoc` walks the command tree into the reference; `internal/reference` builds the rest.**
  `cobra/doc` brings `go-md2man` and writes Markdown, man pages, ReST and YAML rather than this shape.
  The walk does not filter `--help`: cobra adds it only to the command running, which is the hidden
  `reference`, and a test fails if it ever appears.
- **Markdown is handled with the standard library.** ~~The dependency list is cobra, yaml.v3 and huh, with~~
  ~~goreleaser at build time. Nothing else.~~ The list left out `x/term`, which
  [ADR-0005](../spec/adr/0005-the-dependency-list-is-closed.md) names, and the code has since imported
  pflag and x/text, which [ADR-0025](../spec/adr/0025-the-dependency-list-adds-pflag-and-x-text.md)
  proposes adding, which makes the list cobra, pflag, yaml.v3, huh, `x/term` and x/text, with
  goreleaser at build time. Nothing else.
