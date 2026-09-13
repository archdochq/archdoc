// Package git exposes the few git operations archdoc needs, behind an
// interface so that callers can be tested without a repository.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// ErrNotARepository reports that a directory is not inside a git repository.
// L11 skips itself rather than failing when it sees this.
var ErrNotARepository = errors.New("not inside a git repository")

// Repository is every git operation archdoc performs, and nothing else until
// something needs more.
type Repository interface {
	// FileAt returns the contents of path as committed on branch. ok is false
	// when the path does not exist there.
	FileAt(branch, path string) (content []byte, ok bool, err error)
	// FilesAt returns the contents of many paths as committed on branch, in one
	// operation. A path absent from the branch is absent from the result rather
	// than an error. Reading a document at a time cost a subprocess per
	// document, which was the whole of lint's running time on a repository of
	// any size.
	FilesAt(branch string, paths []string) (map[string][]byte, error)
	// Changed names which of paths differ between branch and the working tree,
	// as git itself judges it.
	//
	// git is asked rather than the bytes compared, because git applies checkout
	// filters between the blob it stores and the file it writes to disk. Under
	// end-of-line normalisation, which is the default on Windows, every frozen
	// document differed byte for byte while git reported the tree clean, so the
	// rule this tool exists to enforce failed on every document there.
	Changed(branch string, paths []string) (map[string]bool, error)
	// HasCommits reports whether the repository holds any commit at all. A
	// freshly initialised one holds none, so no document can be frozen yet and
	// the configured branch being absent is expected rather than a fault.
	HasCommits() (bool, error)
	// BranchExists reports whether branch resolves to a commit. L11 needs this
	// to tell a repository with no such branch from one where every document is
	// new, which would otherwise pass silently.
	BranchExists(branch string) (bool, error)
	// ListFiles names every path committed on branch, sorted. L11 needs it to
	// notice a frozen document deleted from the working tree, which discovery
	// cannot see.
	ListFiles(branch string) ([]string, error)
	// CurrentBranch is the checked-out branch.
	CurrentBranch() (string, error)
	// RepoRoot is the absolute path of the repository's top level.
	RepoRoot() (string, error)
}

// shell runs the git binary. It is the only implementation; the interface
// exists so that callers can be given a stub.
type shell struct {
	dir string
	// resolved caches the revision each branch name was found under, so the
	// lookup costs one subprocess per branch rather than one per question.
	resolved map[string]string
	// root is the repository's top level, asked for once. Open already runs
	// rev-parse --show-toplevel to decide whether this is a repository at all,
	// so the answer is free and was being discarded and then asked for again
	// three times per lint run.
	root string
	// present caches which revisions exist, and listed the files on each
	// branch. One lint run asked the same two questions three and two times
	// respectively, each costing a subprocess and none of it changing under a
	// single run.
	present map[string]bool
	listed  map[string][]string
}

// Open returns a Repository rooted at dir. It returns ErrNotARepository only
// when git itself reports that the directory is outside a repository. Any other
// failure, git missing from PATH above all, is returned as it is: a caller that
// treated those alike would skip every frozen-document check and still succeed.
func Open(dir string) (Repository, error) {
	s := &shell{dir: dir, resolved: map[string]string{}, present: map[string]bool{}, listed: map[string][]string{}}
	switch out, code, err := s.run("rev-parse", "--show-toplevel"); {
	case err == nil:
		s.root = strings.TrimSpace(string(out))
		return s, nil
	case code > 0:
		// git ran and said no.
		return nil, fmt.Errorf("%w: %s: %w", ErrNotARepository, dir, err)
	default:
		// git could not be run at all.
		return nil, err
	}
}

// run executes git and returns its standard output. A command that exits
// non-zero returns its status as code, so callers can tell "no such object",
// which git reports as 1, from a failure to run git at all.
func (s *shell) run(args ...string) (out []byte, code int, err error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.dir

	// Output captures stderr into ExitError.Stderr, because cmd.Stderr is nil.
	out, err = cmd.Output()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out, 0, nil
	case errors.As(err, &exit):
		return out, exit.ExitCode(), fmt.Errorf(
			"git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
	default:
		return nil, -1, fmt.Errorf("running git %s: %w", strings.Join(args, " "), err)
	}
}

// exists reports whether a revision resolves. --verify --quiet makes the exit
// status the only signal, so an absent object is never confused with a broken
// repository.
func (s *shell) exists(rev string) (bool, error) {
	if cached, ok := s.present[rev]; ok {
		return cached, nil
	}
	found, err := s.probe(rev)
	if err == nil {
		s.present[rev] = found
	}
	return found, err
}

func (s *shell) probe(rev string) (bool, error) {
	_, code, err := s.run("rev-parse", "--verify", "--quiet", rev)
	switch {
	case err == nil:
		return true, nil
	case code == 1:
		return false, nil
	default:
		return false, err
	}
}

// rev is the revision a branch name should be read from. A checkout often holds
// the comparison branch only as a remote-tracking ref: actions/checkout leaves a
// pull request on a detached head with no local branch at all, and rev-parse
// does not resolve a bare name to origin/<name>. Without this the rule the tool
// exists for reports that the branch is missing on every pull request.
func (s *shell) rev(branch string) string {
	if cached, ok := s.resolved[branch]; ok {
		return cached
	}
	resolved := branch
	if local, err := s.exists(branch + "^{commit}"); err == nil && !local {
		if remote, err := s.exists("origin/" + branch + "^{commit}"); err == nil && remote {
			resolved = "origin/" + branch
		}
	}
	s.resolved[branch] = resolved
	return resolved
}

func (s *shell) FileAt(branch, path string) ([]byte, bool, error) {
	rev := s.rev(branch) + ":" + path

	// One subprocess in the common case. cat-file reports a missing path itself,
	// so the rev-parse probe that used to precede every read is needed only to
	// tell "absent" from "unreadable", and only when the read has already
	// failed. The caller that reads a document per document was paying for both
	// on every one of them. cat-file blob also refuses a directory, where show
	// would have printed a tree listing as though it were content.
	content, _, err := s.run("cat-file", "blob", rev)
	if err == nil {
		return content, true, nil
	}
	switch ok, probeErr := s.exists(rev); {
	case probeErr != nil:
		return nil, false, probeErr
	case !ok:
		return nil, false, nil
	}
	return nil, false, err
}

// FilesAt reads every requested blob in a single `git cat-file --batch`.
//
// The batch protocol answers each request line in order, with either
// "<sha> <type> <size>" followed by that many bytes and a newline, or
// "<request> missing". Pairing responses with requests by position is what lets
// a path containing a space be read back safely, since only the missing form
// echoes the request and it is recognised by its suffix.
func (s *shell) FilesAt(branch string, paths []string) (map[string][]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	rev := s.rev(branch)

	var in bytes.Buffer
	for _, p := range paths {
		in.WriteString(rev + ":" + p)
		in.WriteByte(0)
	}

	// -z, so requests are separated by NUL rather than by newline. git permits
	// a newline in a path, and with newline-separated requests such a path
	// became two requests and two responses while this loop read one response
	// per path: every later path was then paired with the previous path's
	// content. L11 reported an untouched document as changed and missed a real
	// tamper, and an archdoc root whose directory name contained a newline
	// disabled L11 entirely, in silence, at exit 0. ListFiles already used -z
	// for the same reason.
	cmd := exec.Command("git", "cat-file", "--batch", "-z")
	cmd.Dir = s.dir
	cmd.Stdin = &in
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	files := make(map[string][]byte, len(paths))
	rest := out.Bytes()
	for _, p := range paths {
		nl := bytes.IndexByte(rest, '\n')
		if nl < 0 {
			return nil, fmt.Errorf("git cat-file --batch: response ended after %d of %d paths", len(files), len(paths))
		}
		header := string(rest[:nl])
		rest = rest[nl+1:]
		if strings.HasSuffix(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size > len(rest) {
			return nil, fmt.Errorf("git cat-file --batch: unusable size in %q", header)
		}
		if fields[1] == "blob" {
			files[p] = bytes.Clone(rest[:size])
		}
		rest = rest[size:]
		// The batch separates entries with a newline of its own.
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		}
	}
	// Belt and braces: anything left means the responses and the requests did
	// not line up, and every value read above is then suspect.
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("git cat-file --batch: %d bytes left after %d paths", len(rest), len(paths))
	}
	return files, nil
}

func (s *shell) HasCommits() (bool, error) { return s.exists("HEAD") }

func (s *shell) Changed(branch string, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	// :(top) makes each pathspec relative to the repository root, which is what
	// the caller has. git otherwise resolves a pathspec against the process's
	// working directory, and this runs in the directory holding archdoc.json:
	// in the layout `archdoc init` scaffolds inside a subdirectory, and the one
	// its own generated workflow runs from, every pathspec matched nothing and
	// so every frozen document passed. The freeze check was silently off.
	// -c diff.relative=false: with it set, which a user may have in ~/.gitconfig,
	// git prints paths relative to the working directory. This runs in the
	// directory holding archdoc.json and matches the output against
	// repository-root-relative paths, so every frozen document read as unchanged
	// and the whole check went off, silently, at exit 0. The :(top) prefix below
	// settles the input side of the same assumption.
	args := []string{"-c", "diff.relative=false", "diff", "--name-only", "-z", s.rev(branch), "--"}
	for _, p := range paths {
		args = append(args, ":(top)"+p)
	}
	out, _, err := s.run(args...)
	if err != nil {
		return nil, err
	}
	changed := make(map[string]bool, len(paths))
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			changed[name] = true
		}
	}
	return changed, nil
}

func (s *shell) ListFiles(branch string) ([]string, error) {
	if cached, ok := s.listed[branch]; ok {
		return cached, nil
	}
	files, err := s.listFiles(branch)
	if err == nil {
		s.listed[branch] = files
	}
	return files, err
}

func (s *shell) listFiles(branch string) ([]string, error) {
	out, _, err := s.run("ls-tree", "-r", "--full-name", "--name-only", "-z", s.rev(branch))
	if err != nil {
		return nil, err
	}
	// --full-name prints paths from the repository root rather than from the
	// working directory, which is the base every caller translates against.
	// -z separates with NUL, so paths containing newlines or quotes survive
	// intact and need no unquoting.
	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	slices.Sort(files)
	return files, nil
}

func (s *shell) BranchExists(branch string) (bool, error) {
	return s.exists(s.rev(branch) + "^{commit}")
}

func (s *shell) CurrentBranch() (string, error) {
	// symbolic-ref, not rev-parse: rev-parse resolves HEAD to a commit, so on a
	// repository with no commits yet it fails outright. That is the state
	// `git init` leaves, and `archdoc init` runs in it, so the branch name went
	// unanswered and the default "main" was written into the configuration of a
	// repository whose branch was something else. The first commit then made
	// L11 fail on a repository the user had done nothing wrong in.
	//
	// symbolic-ref fails on a detached head, which has no branch, and the
	// caller treats that as "no answer" exactly as it did before.
	out, _, err := s.run("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *shell) RepoRoot() (string, error) {
	if s.root != "" {
		return s.root, nil
	}
	out, _, err := s.run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	s.root = strings.TrimSpace(string(out))
	return s.root, nil
}
