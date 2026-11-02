// Package changes compares the record in the working tree with the record on
// the configured branch: what `archdoc changes` lists, and what the rules lint
// runs on a pull request judge. Nothing here prints.
package changes

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/repo"
	"golang.org/x/text/unicode/norm"
)

// ErrNoBase reports that there is no configured branch to compare against,
// which is a usage error rather than a finding: there is nothing to judge.
var ErrNoBase = errors.New("no branch to compare against")

// NoSharedHistory says HEAD and branch share no commit, formatted with the
// branch. actions/checkout fetches a depth of one by default, which holds HEAD
// alone and is the usual way to arrive here.
func NoSharedHistory(branch string) string {
	return fmt.Sprintf("HEAD shares no history with %s; a shallow clone needs its full history (fetch-depth: 0)", branch)
}

// Base is the record on the configured branch as HEAD last took it in: the
// commit GitHub merged a pull request against, or the point an unrebased branch
// was cut from, so that what the branch merged since is not read as removed.
type Base struct {
	Branch string
	// Commit is the merge base of HEAD and Branch, which is what is read.
	Commit string
	// Root is where the documents live on the base, as a path from the
	// repository's top level, which a pull request moving the root changes.
	Root string
	// Documents is every document on the branch, by path relative to root.
	Documents map[string]*repo.Document
	// Strict is the branch's own setting, so that a pull request cannot relax
	// the rule it is judged by, or the working tree's where the branch holds no
	// archdoc.json, as before the first pull request that adds one, or one that
	// cannot be read.
	Strict bool
}

// ReadBase reads every document at the merge base of HEAD and the configured
// branch, and its strict, in one batch.
func ReadBase(r *repo.Repo, g git.Repository) (*Base, error) {
	branch := r.Config().Branch
	if g == nil {
		return nil, fmt.Errorf("%w: not inside a git repository, so there is no %s", ErrNoBase, branch)
	}
	exists, err := g.BranchExists(branch)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w: branch %q does not exist", ErrNoBase, branch)
	}
	commit, err := g.MergeBase(branch)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoBase, NoSharedHistory(branch))
	}
	located, err := Locate(r, g, commit)
	if err != nil {
		return nil, err
	}
	files, err := g.ListFiles(commit)
	if err != nil {
		return nil, err
	}

	var wanted []string
	docPaths := map[string]string{}
	for _, file := range files {
		if docPath, ok := Within(file, located.Root); ok {
			if _, isDoc := repo.TypeOf(docPath); isDoc {
				wanted = append(wanted, file)
				docPaths[file] = docPath
			}
		}
	}
	contents, err := g.FilesAt(commit, wanted)
	if err != nil {
		return nil, err
	}

	base := &Base{Branch: branch, Commit: commit, Root: located.Root, Documents: make(map[string]*repo.Document, len(docPaths)), Strict: located.Strict}
	for file, docPath := range docPaths {
		if d, ok := repo.Parse(docPath, contents[file]); ok {
			base.Documents[docPath] = d
		}
	}
	return base, nil
}

// Located is the record as a revision holds it: the settings its archdoc.json
// gives, and its root as a path from the repository's top level.
type Located struct {
	config.Settings
	Root string
	// Unreadable is why the revision's archdoc.json could not be read, where
	// it could not, and the working tree's settings stand in for it. It is not
	// an error: a pull request mending the branch's file could never pass.
	Unreadable error
}

// Locate reads the settings rev holds: from its archdoc.json at the working
// tree's location, or failing that from the one the working tree's moved from,
// so that a pull request moving the file is still judged by the base's strict
// and followed to the base's root. Where rev holds neither, as before the pull
// request that adds ArchDoc, the working tree's own settings stand.
func Locate(r *repo.Repo, g git.Repository, rev string) (Located, error) {
	working := Located{
		Settings: config.Settings{Root: r.Config().Root, Strict: r.Config().Strict},
		// g is opened in the directory holding archdoc.json, as every command
		// opens it, and root is relative to that.
		Root: git.PathOf(g, r.Config().Root),
	}
	files, err := g.ListFiles(rev)
	if err != nil {
		return working, err
	}
	here := git.PathOf(g, config.Filename)
	settings := here
	if !slices.Contains(files, settings) {
		if settings, err = movedFrom(g, files); err != nil || settings == "" {
			return working, err
		}
	}
	contents, err := g.FilesAt(rev, []string{settings})
	if err != nil {
		return working, err
	}
	read, err := config.SettingsIn(contents[settings])
	if err != nil {
		working.Unreadable = fmt.Errorf("%s: %w", settings, err)
		return working, nil
	}
	root := path.Join(path.Dir(settings), read.Root)
	if root == "." {
		root = ""
	}
	// A root rev lists nothing under is asked of git as the working tree
	// spells it: written in another case, in another Unicode normalisation or
	// through a symbolic link, it reaches a directory git lists under another
	// name. Failing that, as for a moved file's root, which is relative to
	// where the file was, it is matched against rev's listing, case-folded and
	// composed. One rev lists files under is taken as written, so that a link
	// left where the root was cannot send rev's documents elsewhere.
	if !slices.ContainsFunc(files, func(f string) bool { _, ok := Within(f, root); return ok }) {
		if settings == here {
			root = git.PathOf(g, read.Root)
		}
		root = listedAs(files, root)
	}
	// A file gone from the working tree moved only if the record came with it:
	// one whose documents are none of them under the working tree's root, such
	// as an example deleted beside the record, was not the record's.
	if settings != here {
		held, kept := false, false
		for _, f := range files {
			if docPath, ok := Within(f, root); ok {
				if _, isDoc := repo.TypeOf(docPath); isDoc {
					held = true
					kept = kept || r.ByPath(docPath) != nil
				}
			}
		}
		if held && !kept {
			return working, nil
		}
	}
	return Located{Settings: read, Root: root}, nil
}

// listedAs is root as files spell it, where a directory in files matches it
// case-folded and composed, and root as written where none does.
// ponytail: the first match in git's order is taken; a listing holding the
// same directory under two such spellings, which only a case-sensitive
// checkout can produce, reads the first.
func listedAs(files []string, root string) string {
	want := norm.NFC.String(root)
	for _, f := range files {
		for dir := path.Dir(f); dir != "."; dir = path.Dir(dir) {
			if strings.EqualFold(norm.NFC.String(dir), want) {
				return dir
			}
		}
	}
	return root
}

// movedFrom is the archdoc.json among files that the working tree's moved
// from: the only one the working tree no longer holds. One still there belongs
// to something else, such as an example beside the record, and of several gone
// which one moved cannot be told, so it is empty for both.
func movedFrom(g git.Repository, files []string) (string, error) {
	top, err := g.RepoRoot()
	if err != nil {
		return "", err
	}
	from := ""
	for _, file := range files {
		if path.Base(file) != config.Filename {
			continue
		}
		if _, err := os.Lstat(filepath.Join(top, filepath.FromSlash(file))); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if from != "" {
			return "", nil
		}
		from = file
	}
	return from, nil
}

// Within is file as a path inside root, both from the top level.
func Within(file, root string) (string, bool) {
	if root == "" {
		return file, true
	}
	return strings.CutPrefix(file, root+"/")
}

// Differing names which of docPaths, paths within a root, differ between rev,
// whose documents live under revRoot, and the working tree, whose live under
// root, as git judges it. A document absent from rev differs.
func Differing(g git.Repository, rev, revRoot, root string, docPaths []string) (map[string]bool, error) {
	if len(docPaths) == 0 {
		return map[string]bool{}, nil
	}
	full := make(map[string]string, len(docPaths))
	var differs map[string]bool
	var err error
	if revRoot == root {
		paths := make([]string, 0, len(docPaths))
		for _, p := range docPaths {
			full[path.Join(root, p)] = p
			paths = append(paths, path.Join(root, p))
		}
		differs, err = g.Changed(rev, paths)
	} else {
		moved := make(map[string]string, len(docPaths))
		for _, p := range docPaths {
			full[path.Join(root, p)] = p
			moved[path.Join(root, p)] = path.Join(revRoot, p)
		}
		differs, err = g.Differs(rev, moved)
	}
	if err != nil {
		return nil, err
	}
	edited := make(map[string]bool, len(differs))
	for f, d := range differs {
		if d {
			edited[full[f]] = true
		}
	}
	return edited, nil
}

// The changes a document can have.
const (
	Arrives = "arrives"
	Status  = "status"
	Edited  = "edited"
	Removed = "removed"
)

// Change is one document the working tree changes against the base.
type Change struct {
	Type   repo.Type   `json:"type"`
	ID     string      `json:"id,omitempty"`
	Path   string      `json:"path"`
	Title  string      `json:"title,omitempty"`
	Change string      `json:"change"`
	From   repo.Status `json:"from,omitempty"`
	To     repo.Status `json:"to,omitempty"`
	// Terminal is set on an edit to an RFC or ADR only: whether the document
	// was terminal on the base, which L11 reports.
	Terminal *bool `json:"terminal,omitempty"`

	number int
}

// order is the order types are listed in: the decisions first, then what
// describes and supports them.
var order = []repo.Type{repo.TypeRFC, repo.TypeADR, repo.TypeSpec, repo.TypeRef, repo.TypeTerm, repo.TypeGroup}

// Of lists what the working tree changes against base. A rename is a removal
// and an arrival, as L11 treats it.
func Of(r *repo.Repo, g git.Repository, base *Base) ([]Change, error) {
	root := git.PathOf(g, r.Config().Root)
	var changes []Change
	both := map[string]*repo.Document{}
	var docPaths []string
	present := map[string]bool{}
	for _, d := range r.Documents() {
		present[d.Path] = true
		was, ok := base.Documents[d.Path]
		switch {
		case !ok:
			c := change(d, Arrives)
			if d.Type.HasLifecycle() {
				c.To = d.FrontMatter.Status
			}
			changes = append(changes, c)
		case d.Type.HasLifecycle() && d.FrontMatter.Status != was.FrontMatter.Status &&
			d.FrontMatter.Status != "" && was.FrontMatter.Status != "":
			// A status missing on either side is a front matter fault, which
			// L01 reports; the file reads as edited rather than as a move.
			c := change(d, Status)
			c.From, c.To = was.FrontMatter.Status, d.FrontMatter.Status
			changes = append(changes, c)
		default:
			both[d.Path] = d
			docPaths = append(docPaths, d.Path)
		}
	}

	// Compared across roots, so a pull request moving the root lists what it
	// edited and not every document as arriving.
	edited, err := Differing(g, base.Commit, base.Root, root, docPaths)
	if err != nil {
		return nil, err
	}
	for _, docPath := range docPaths {
		if !edited[docPath] {
			continue
		}
		d := both[docPath]
		c := change(d, Edited)
		if d.Type.HasLifecycle() {
			terminal := base.Documents[d.Path].FrontMatter.Status.Terminal()
			c.Terminal = &terminal
		}
		changes = append(changes, c)
	}

	for docPath, was := range base.Documents {
		if present[docPath] {
			continue
		}
		c := change(was, Removed)
		if was.Type.HasLifecycle() {
			c.From = was.FrontMatter.Status
		}
		changes = append(changes, c)
	}

	slices.SortFunc(changes, func(a, b Change) int {
		return cmp.Or(
			cmp.Compare(slices.Index(order, a.Type), slices.Index(order, b.Type)),
			cmp.Compare(a.number, b.number),
			strings.Compare(a.Path, b.Path),
		)
	})
	return changes, nil
}

func change(d *repo.Document, what string) Change {
	return Change{Type: d.Type, ID: d.ID, Path: d.Path, Title: d.FrontMatter.Title, Change: what, number: d.Number}
}
