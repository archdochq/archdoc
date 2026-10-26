// Package lint checks a spec repository against the rules in PROCESS.md. Every
// rule is a function returning findings; nothing here prints or exits.
package lint

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/repo"
)

// Severity is how much a finding matters. A rule never reports an error against
// a frozen document, because L11 forbids the edit that would clear it.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Finding is one rule's complaint about one place.
type Finding struct {
	// Path is relative to root.
	Path string
	// Line is 1-based, or 0 when the finding has no single line.
	Line     int
	Severity Severity
	Message  string
	Rule     string
}

// Context is everything a rule needs.
//
// Its fields are unexported and NewContext is the only way to fill them. That
// is deliberate: the branch snapshot is what Frozen reads, and a Context built
// as a literal without it reported every document as editable, which made the
// same document an error from one command and a warning from another. A rule
// stating "use the constructor" in a comment was not enough, so the compiler
// states it instead.
type Context struct {
	repo *repo.Repo
	// git is nil outside a repository, which L11 reports rather than assumes.
	git git.Repository
	// now is today, injected so that date rules are testable.
	now time.Time
	// branch is each document as it stands on the configured branch, keyed by
	// path. A path that is absent is new on the branch.
	//
	// It is a pointer to a snapshot built on first use, not a map built up
	// front. Building it costs a git subprocess per document, and `archdoc
	// link` asks for it only when a terminal document turns out to contain a
	// wiki link, which on most repositories is never. A pointer, because a
	// Context is copied by value into every rule and Frozen has a value
	// receiver: a plain lazily filled map would be rebuilt by each of them.
	branch *snapshot
	// base is the record on the configured branch, set only when lint is
	// asked to judge a pull request. The merge rules run only when it is.
	base *changes.Base
}

// snapshot defers reading the branch until something asks.
type snapshot struct {
	once  sync.Once
	load  func() map[string]BranchFile
	files map[string]BranchFile
}

func (s *snapshot) get() map[string]BranchFile {
	if s == nil {
		return nil
	}
	s.once.Do(func() { s.files = s.load() })
	return s.files
}

// Repo is the repository being checked.
func (c Context) Repo() *repo.Repo { return c.repo }

// Git is the repository's git, or nil when there is not one.
func (c Context) Git() git.Repository { return c.git }

// Now is the date the rules compare against.
func (c Context) Now() time.Time { return c.now }

// OnBranch is a document as it stands on the configured branch. ok is false
// when it is new there, or when there is no branch to compare against.
func (c Context) OnBranch(path string) (BranchFile, bool) {
	on, ok := c.branch.get()[path]
	return on, ok
}

// BranchFile is a document as it stands on the branch. Err records a failure to
// read it, which must not be mistaken for the document being new: one means the
// repository is unreadable, the other that the document is editable.
type BranchFile struct {
	Content []byte
	Status  repo.Status
	Err     error
}

// Frozen reports whether a document has already reached the branch in a
// terminal status, and so can no longer be edited. A rule never reports an
// error against a frozen document, because L11 forbids the fix.
func (c Context) Frozen(d *repo.Document) bool {
	on, ok := c.branch.get()[d.Path]
	return ok && on.Err == nil && on.Status.Terminal()
}

// Rule is one check, identified by the code it reports under, with what the
// reference publishes about it and the triage guide's table is generated from.
type Rule struct {
	Code  string
	Check func(Context) []Finding
	// Title names what the rule checks in a few words.
	Title string
	// Summary says what a finding means, and Remedy how one is cleared, both
	// as inline Markdown.
	Summary string
	Remedy  string
	// Since is the tag of the first release carrying the rule, and Changes the
	// later releases that changed what it reports, in release order.
	Since   string
	Changes []Change
}

// Change is a release that changed what a rule reports.
type Change struct {
	Release string
	Summary string
}

// Merge reports whether a rule judges a pull request against its base, which
// Run applies only to a context carrying one. Their codes start with M.
func (r Rule) Merge() bool { return strings.HasPrefix(r.Code, "M") }

// NewContext builds a Context with the branch snapshot populated. Any caller
// whose rules depend on whether a document is frozen must use this rather than
// building a Context literal: Branch is what Frozen reads, and a Context
// without it reports every document as editable.
func NewContext(r *repo.Repo, g git.Repository, now time.Time) Context {
	ctx := Context{repo: r, git: g, now: now}
	if g != nil {
		// The closure captures ctx before branch is set, which is safe because
		// branchFiles reads only the repository and the git handle.
		ctx.branch = &snapshot{load: func() map[string]BranchFile { return branchFiles(ctx) }}
	}
	return ctx
}

// branchFiles reads each document as it stands on the branch. A document absent
// from the branch is new and is left out; one that cannot be read is recorded
// with its error, so that L11 can report it rather than concluding the document
// is editable.
func branchFiles(ctx Context) map[string]BranchFile {
	branch := ctx.Repo().Config().Branch
	if exists, err := ctx.Git().BranchExists(branch); err != nil || !exists {
		return nil
	}
	prefix := gitPrefix(ctx)

	// One listing, rather than asking git whether each document exists. A
	// document absent from the branch is the common case in an active
	// repository, and each question costs a subprocess.
	onBranch := map[string]bool{}
	if listed, err := ctx.Git().ListFiles(branch); err == nil {
		for _, file := range listed {
			onBranch[file] = true
		}
	}

	// One read for all of them, for the same reason. Reading a document at a
	// time cost a subprocess per document, and on a committed repository of any
	// size that was the whole of lint's running time.
	wanted := make([]string, 0, len(ctx.Repo().Documents()))
	byFullPath := make(map[string]string, len(wanted))
	for _, d := range ctx.Repo().Documents() {
		full := path.Join(prefix, d.Path)
		if len(onBranch) > 0 && !onBranch[full] {
			continue // new on the branch
		}
		wanted = append(wanted, full)
		byFullPath[full] = d.Path
	}

	contents, err := ctx.Git().FilesAt(branch, wanted)
	files := make(map[string]BranchFile, len(wanted))
	if err != nil {
		// Recorded against every document rather than swallowed: a failure to
		// read the branch must not be mistaken for the documents being new,
		// which is the opposite conclusion.
		for _, full := range wanted {
			files[byFullPath[full]] = BranchFile{Err: err}
		}
		return files
	}
	for _, full := range wanted {
		if committed, ok := contents[full]; ok {
			// StatusIn reads a status key out of any blob, and a spec page or a
			// ref has no lifecycle, so one carrying the key by mistake was treated
			// as frozen and every later edit to it became an L11 error on a
			// document type that cannot be frozen at all.
			var status repo.Status
			if d := ctx.Repo().ByPath(byFullPath[full]); d != nil && d.Type.HasLifecycle() {
				status = repo.StatusIn(committed)
			}
			files[byFullPath[full]] = BranchFile{Content: committed, Status: status}
		}
	}
	return files
}

// Rules is every rule, in code order.
var Rules = []Rule{
	{
		Code:    "L01",
		Check:   L01,
		Title:   "Front matter matches the schema",
		Summary: "Front matter is malformed, has an unknown key, a required key is missing or empty, or `as_of` is empty",
		Remedy:  "Fix the front matter",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L02",
		Check:   L02,
		Title:   "The identifier matches the filename",
		Summary: "`id` does not match the filename, or the filename is not `<NNNN>-<slug>.md`",
		Remedy:  "Rename the file, or correct `id`",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L03",
		Check:   L03,
		Title:   "Identifiers are unique",
		Summary: "Two documents claim the same identifier",
		Remedy:  "Renumber the wrong one with `archdoc renumber <path>`, then correct each reference it lists that meant that one",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L04",
		Check:   L04,
		Title:   "References resolve",
		Summary: "A referenced identifier does not resolve, or a group's member names nothing or is listed twice",
		Remedy:  "Create the document, or correct the reference or the group's list",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L05",
		Check:   L05,
		Title:   "Updates and obsoletes name accepted documents",
		Summary: "`updates`/`obsoletes` targets something not accepted, of the wrong type, or itself",
		Remedy:  "Accept the target, or correct the list",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L06",
		Check:   L06,
		Title:   "Includes name accepted documents",
		Summary: "`includes` targets something not accepted",
		Remedy:  "Accept the target, or remove it from the page",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L07",
		Check:   L07,
		Title:   "Dates are consistent",
		Summary: "`decided` set without a terminal status, or before `created`, backfill dates disagree, or a ref's `as_of` is after `verified`",
		Remedy:  "Correct the dates",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L08",
		Check:   L08,
		Title:   "Required sections are present and in order",
		Summary: "A required section is missing, misnamed, out of order, or the closing section is not last",
		Remedy:  "Add or reorder. Titles must match exactly",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L09",
		Check:   L09,
		Title:   "An accepted document has no open questions",
		Summary: "An accepted document still has content under `Open questions`",
		Remedy:  "Move each question to `Resolved questions` with its answer, and remove anything else",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L10",
		Check:   L10,
		Title:   "The heading matches the front matter",
		Summary: "The H1 does not match the front matter",
		Remedy:  "Correct whichever is wrong",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L11",
		Check:   L11,
		Title:   "Frozen documents are unchanged",
		Summary: "A frozen document changed, or was deleted",
		Remedy:  "Revert it. To change what it says, write a new document that updates or obsoletes it",
		Since:   "v0.1.0-alpha.1",
		Changes: []Change{
			{Release: "v0.6.0", Summary: "Judges what changed or was deleted against the merge base of HEAD and the branch, not the branch's tip"},
		},
	},
	{
		Code:    "L12",
		Check:   L12,
		Title:   "Spec pages include nothing obsolete",
		Summary: "A spec page includes something obsoleted (warning)",
		Remedy:  "Update the page and its `includes`",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L13",
		Check:   L13,
		Title:   "Refs are verified recently",
		Summary: "A ref's `verified` date is older than `ref_stale_days`, and it carries no `as_of` (warning)",
		Remedy:  "Re-check the content, then update `verified`. A record of an exchange carries `as_of` instead",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L14",
		Check:   L14,
		Title:   "Required sections are not empty",
		Summary: "A required section is empty",
		Remedy:  "Write it",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L15",
		Check:   L15,
		Title:   "A term is defined in one paragraph",
		Summary: "A term's definition under `term/` is not exactly one paragraph",
		Remedy:  "Rewrite the definition as one paragraph",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L16",
		Check:   L16,
		Title:   "Wiki links are resolved",
		Summary: "An unresolved `[[...]]` link",
		Remedy:  "Run `archdoc link`",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L17",
		Check:   L17,
		Title:   "Relative links resolve",
		Summary: "A relative link points at a missing file or heading",
		Remedy:  "Fix the path or the anchor",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L18",
		Check:   L18,
		Title:   "Dependencies are as settled as the document",
		Summary: "`depends` names something less settled than the referring document (warning)",
		Remedy:  "Usually accept the dependency first",
		Since:   "v0.1.0-alpha.1",
	},
	{
		Code:    "L19",
		Check:   L19,
		Title:   "No glossary page is left from before terms",
		Summary: "`spec/glossary.md` is left from before terms became files, and is no longer read (warning)",
		Remedy:  "Move its terms to `term/` with `archdoc term add`. Guide: `spec-pages.md`",
		Since:   "v0.5.0",
	},
	{
		Code:    "L20",
		Check:   L20,
		Title:   "Question sections are shaped as questions",
		Summary: "A question section is not shaped as questions: something before the first question; a question with no anchor, or sharing one with another heading; a heading beneath a question; a blockquote holding a heading, fence or comment; an open question ending with a blockquote; a resolved question with no answer, an empty one, or another blockquote just before it; `Resolved questions` empty, misplaced or repeated",
		Remedy:  "Rewrite it in that shape: reword a clashing question, join two blockquotes with a line holding only `>`. Guide: `authoring.md`, Questions",
		Since:   "v0.6.0",
	},
	{
		Code:    "L21",
		Check:   L21,
		Title:   "The Changelog has the shape the tool reads",
		Summary: "A Changelog entry is undated, has a malformed or impossible date, has a date and no text, or is out of order or outside the document's dates; a line in the section is outside the list form the tool reads; the heading is indented, misspelt or repeated, or the heading after the section is indented",
		Remedy:  "Date the entry, or rewrite the line as an entry or within one, and write the heading `## Changelog` at the start of its line. Guide: `authoring.md`, The Changelog",
		Since:   "v0.6.0",
	},
	{
		Code:    "M01",
		Check:   M01,
		Title:   "No draft lands",
		Summary: "An RFC or ADR arrives on the configured branch as a draft",
		Remedy:  "Propose it with `archdoc propose` before it merges",
		Since:   "v0.6.0",
	},
	{
		Code:    "M02",
		Check:   M02,
		Title:   "An arriving document takes a free number",
		Summary: "A document arrives carrying an identifier a document on the configured branch already holds",
		Remedy:  "Rebase, then renumber the arriving one by path: `archdoc renumber <path>`",
		Since:   "v0.6.0",
	},
	{
		Code:    "M03",
		Check:   M03,
		Title:   "Status changes follow the lifecycle from the branch",
		Summary: "A change of status the lifecycle does not permit from the status on the configured branch, under that branch's `strict`",
		Remedy:  "Make one step at a time from the branch: where `strict` is on, merge the proposal first and give the verdict afterwards",
		Since:   "v0.6.0",
	},
	{
		Code:    "M04",
		Check:   M04,
		Title:   "A proposed document is withdrawn, not deleted",
		Summary: "A document proposed on the configured branch is deleted or renumbered",
		Remedy:  "Restore it, and withdraw it with `archdoc withdraw` if it is no longer wanted",
		Since:   "v0.6.0",
	},
}

// Run applies every rule and returns the findings, ordered by path, then line,
// then rule, so that output is stable between runs.
func Run(ctx Context) []Finding {
	// The registry is the single source of truth for a rule's code, so the rule
	// functions do not repeat it on every finding they build.
	var findings []Finding
	for _, rule := range Rules {
		if rule.Merge() && ctx.base == nil {
			continue
		}
		for _, f := range rule.Check(ctx) {
			f.Rule = rule.Code
			findings = append(findings, f)
		}
	}
	slices.SortStableFunc(findings, func(a, b Finding) int {
		return cmp.Or(
			strings.Compare(a.Path, b.Path),
			cmp.Compare(a.Line, b.Line),
			strings.Compare(a.Rule, b.Rule),
		)
	})
	return findings
}
