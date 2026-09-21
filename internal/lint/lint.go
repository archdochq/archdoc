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

// Rule is one check, identified by the code it reports under.
type Rule struct {
	Code  string
	Check func(Context) []Finding
}

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
	{"L01", L01},
	{"L02", L02},
	{"L03", L03},
	{"L04", L04},
	{"L05", L05},
	{"L06", L06},
	{"L07", L07},
	{"L08", L08},
	{"L09", L09},
	{"L10", L10},
	{"L11", L11},
	{"L12", L12},
	{"L13", L13},
	{"L14", L14},
	{"L15", L15},
	{"L16", L16},
	{"L17", L17},
	{"L18", L18},
}

// Run applies every rule and returns the findings, ordered by path, then line,
// then rule, so that output is stable between runs.
func Run(ctx Context) []Finding {
	// The registry is the single source of truth for a rule's code, so the rule
	// functions do not repeat it on every finding they build.
	var findings []Finding
	for _, rule := range Rules {
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
