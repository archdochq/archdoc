package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
)

func draft(id, title string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: draft\n" +
		"created: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

func accepted(id, title string, depends string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: accepted\n" +
		"created: 2026-01-01\ndecided: 2026-01-02\ndepends: [" + depends + "]\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

// draftDepending is a draft that rests on another document, which is what the
// rename has to follow. It was called accepted2 and builds nothing of the sort.
func draftDepending(id, title, depends string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: draft\n" +
		"created: 2026-01-01\ndecided:\ndepends: [" + depends + "]\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

// nothingFrozen is the predicate for a repository with no history, where every
// document is still editable.
func nothingFrozen(*repo.Document) bool { return false }

// frozenIDs builds a predicate freezing exactly the named documents, standing in
// for what lint learns from the branch.
func frozenIDs(ids ...string) func(*repo.Document) bool {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return func(d *repo.Document) bool { return set[d.ID] }
}

// TestRenumberRefusesWhenAFrozenDocumentReferencesIt is the guard that matters
// most. A frozen document cannot be edited, so its reference to the old
// identifier could never be corrected, and renaming anyway would leave it
// pointing at a file that no longer exists with no permitted repair.
func TestRenumberRefusesWhenAFrozenDocumentReferencesIt(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md":  draft("RFC-0001", "Base"),
		"rfc/0002-later.md": accepted("RFC-0002", "Later", "RFC-0001"),
	})
	before, err := os.ReadFile(r.File("rfc/0002-later.md"))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = repo.Renumber(r, "RFC-0001", "RFC-0009", frozenIDs("RFC-0002"))
	if err == nil {
		t.Fatal("renumbered a document that a frozen document references")
	}
	if !strings.Contains(err.Error(), "RFC-0002") {
		t.Errorf("the error does not name the frozen document that blocks it: %v", err)
	}

	// Nothing may have been written: a refusal is all or nothing.
	if _, err := os.Stat(r.File("rfc/0001-base.md")); err != nil {
		t.Errorf("the original file was removed despite the refusal: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0009-base.md")); err == nil {
		t.Error("a renamed file was left behind despite the refusal")
	}
	after, err := os.ReadFile(r.File("rfc/0002-later.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the frozen document was modified despite the refusal")
	}
}

// TestRenumberRefusesAFrozenDocument covers the other half: the document being
// renumbered is itself frozen, so the rename is the edit L11 forbids.
func TestRenumberRefusesAFrozenDocument(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md": accepted("RFC-0001", "Base", ""),
	})
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", frozenIDs("RFC-0001")); err == nil {
		t.Fatal("renumbered a frozen document")
	}
	if _, err := os.Stat(r.File("rfc/0001-base.md")); err != nil {
		t.Errorf("the original file was removed: %v", err)
	}
}

// TestRenumberRewritesEveryPlaceTheIdentityAppears pins the three places a
// number lives in its own document. Doing two of the three trades an L03 error
// for an L10 one, which is the mistake the command exists to prevent.
func TestRenumberRewritesEveryPlaceTheIdentityAppears(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-retry-policy.md": draft("RFC-0002", "Retry policy"),
	})
	changed, _, err := repo.Renumber(r, "RFC-0002", "RFC-0009", nothingFrozen)
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}

	if _, err := os.Stat(r.File("rfc/0002-retry-policy.md")); err == nil {
		t.Error("the old file is still there")
	}
	moved := r.File("rfc/0009-retry-policy.md")
	body, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("the renamed file is missing: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "id: RFC-0009") {
		t.Errorf("the front matter id was not rewritten:\n%s", text)
	}
	if !strings.Contains(text, "# RFC-0009: Retry policy") {
		t.Errorf("the heading was not rewritten:\n%s", text)
	}
	if strings.Contains(text, "RFC-0002") {
		t.Errorf("the old identifier survives somewhere:\n%s", text)
	}
	if len(changed) == 0 {
		t.Error("Renumber reported changing nothing")
	}
}

// TestRenumberTakesTheNextFreeNumber covers the form the contribution flow
// uses, where the contributor does not care which number they end up with.
func TestRenumberTakesTheNextFreeNumber(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
		"rfc/0002-mine.md":  draft("RFC-0002", "Mine"),
		"rfc/0007-other.md": draft("RFC-0007", "Other"),
	})
	if _, _, err := repo.Renumber(r, "RFC-0002", "", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0008-mine.md")); err != nil {
		t.Errorf("expected the next free number, 0008: %v", err)
	}
}

// TestRenumberUpdatesReferencesInEditableDocuments stops the rename from
// breaking L04 everywhere the old identifier was named.
func TestRenumberUpdatesReferencesInEditableDocuments(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md":  draft("RFC-0001", "Base"),
		"rfc/0002-later.md": draftDepending("RFC-0002", "Later", "RFC-0001"),
		"spec/thing.md":     "---\ntitle: Thing\nincludes: [RFC-0001]\n---\n\n# Thing\n\nSee [[RFC-0001]].\n",
		"group/team.md":     "---\ntitle: Team\ndocuments:\n  - thing\n  - RFC-0001\n---\n\n# Team\n",
	})
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	for _, check := range []struct{ path, want string }{
		{"rfc/0002-later.md", "depends: [RFC-0009]"},
		{"spec/thing.md", "includes: [RFC-0009]"},
		{"spec/thing.md", "See [[RFC-0009]]."},
		{"group/team.md", "  - RFC-0009\n"},
	} {
		body, err := os.ReadFile(filepath.Join(r.Config().RootDir(), filepath.FromSlash(check.path)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), check.want) {
			t.Errorf("%s was not updated, wanted %q:\n%s", check.path, check.want, body)
		}
	}
}

// TestRenumberRefusesANumberAlreadyTaken keeps the command from creating the
// very collision it exists to resolve.
func TestRenumberRefusesANumberAlreadyTaken(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
		"rfc/0002-mine.md":  draft("RFC-0002", "Mine"),
	})
	if _, _, err := repo.Renumber(r, "RFC-0002", "RFC-0001", nothingFrozen); err == nil {
		t.Fatal("renumbered onto a number that is already taken")
	}
}

// TestRenumberRefusesToChangeType keeps an RFC from becoming an ADR, which
// would move it to a directory with different required sections.
func TestRenumberRefusesToChangeType(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
	})
	if _, _, err := repo.Renumber(r, "RFC-0001", "ADR-0001", nothingFrozen); err == nil {
		t.Fatal("renumbered an RFC into an ADR")
	}
}

// TestRenumberRefusesAnAmbiguousIdentifier covers the case the command exists
// for. When two pull requests both claimed a number, two documents carry the
// identifier, and ByID answers with whichever path sorts first. Acting on that
// would rename the other contributor's document.
func TestRenumberRefusesAnAmbiguousIdentifier(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-alice.md": draft("RFC-0002", "Alice"),
		"rfc/0002-bob.md":   draft("RFC-0002", "Bob"),
	})
	_, _, err := repo.Renumber(r, "RFC-0002", "", nothingFrozen)
	if err == nil {
		t.Fatal("renumbered an ambiguous identifier instead of refusing")
	}
	// Both have to be named, or the user cannot tell which one to pass.
	for _, want := range []string{"rfc/0002-alice.md", "rfc/0002-bob.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
	for _, path := range []string{"rfc/0002-alice.md", "rfc/0002-bob.md"} {
		if _, err := os.Stat(r.File(path)); err != nil {
			t.Errorf("%s was touched despite the refusal: %v", path, err)
		}
	}
}

// TestRenumberAcceptsAPathToDisambiguate is the way out of the above: name the
// file, which is unambiguous even when the identifier is not.
func TestRenumberAcceptsAPathToDisambiguate(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-alice.md": draft("RFC-0002", "Alice"),
		"rfc/0002-bob.md":   draft("RFC-0002", "Bob"),
	})
	if _, _, err := repo.Renumber(r, "rfc/0002-bob.md", "", nothingFrozen); err != nil {
		t.Fatalf("Renumber by path: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0003-bob.md")); err != nil {
		t.Errorf("the named document was not renumbered: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0002-alice.md")); err != nil {
		t.Errorf("the other contributor's document was disturbed: %v", err)
	}
}

// sharedNumber is the collision two branches meet when each takes the same next
// number: RFC-0001 committed and referenced, and a second RFC-0001 added.
func sharedNumber() map[string]string {
	return map[string]string{
		"rfc/0001-storage.md":  accepted("RFC-0001", "Storage", ""),
		"rfc/0001-newthing.md": draft("RFC-0001", "New thing"),
		"rfc/0002-cache.md": accepted("RFC-0002", "Cache", "RFC-0001") +
			"\nSee [[RFC-0001]], [the storage design](0001-storage.md) and [the new thing](0001-newthing.md).\n",
	}
}

// TestRenumberLeavesAnIdentifierAnotherDocumentStillClaims: an identifier two
// documents share cannot say which it means, so the rename cannot follow it.
func TestRenumberLeavesAnIdentifierAnotherDocumentStillClaims(t *testing.T) {
	r := repotest.New(t, sharedNumber())
	_, unchanged, err := repo.Renumber(r, "rfc/0001-newthing.md", "", nothingFrozen)
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	body, err := os.ReadFile(r.File("rfc/0002-cache.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"depends: [RFC-0001]", "[[RFC-0001]]", "(0001-storage.md)", "(0003-newthing.md)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("rfc/0002-cache.md lacks %q:\n%s", want, body)
		}
	}
	// Reported, so the author can repoint any that meant the moved document,
	// quoting each line as it now reads.
	want := []string{
		"rfc/0002-cache.md:7: depends: [RFC-0001]",
		"rfc/0002-cache.md:18: See [[RFC-0001]], [the storage design](0001-storage.md) and [the new thing](0003-newthing.md).",
	}
	if strings.Join(unchanged, "\n") != strings.Join(want, "\n") {
		t.Errorf("unchanged = %q, want %q", unchanged, want)
	}
}

// TestRenumberReportsWhatItLeftInLineOrder: the front matter keys are visited
// in a fixed order, which is not the order they appear in. The depends list
// continues beneath its key, so the line reported is the indented item.
func TestRenumberReportsWhatItLeftInLineOrder(t *testing.T) {
	files := sharedNumber()
	files["rfc/0002-cache.md"] = strings.Replace(accepted("RFC-0002", "Cache", "RFC-0001"),
		"status: accepted\n", "status: accepted\nupdates: [RFC-0001]\n", 1)
	files["rfc/0002-cache.md"] = strings.NewReplacer("updates: []\n", "", "depends: [RFC-0001]", "depends:\n  - RFC-0001").
		Replace(files["rfc/0002-cache.md"])
	r := repotest.New(t, files)
	_, unchanged, err := repo.Renumber(r, "rfc/0001-newthing.md", "", nothingFrozen)
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	want := []string{"rfc/0002-cache.md:5: updates: [RFC-0001]", "rfc/0002-cache.md:9: - RFC-0001"}
	if strings.Join(unchanged, "\n") != strings.Join(want, "\n") {
		t.Errorf("unchanged = %q, want %q", unchanged, want)
	}
}

// TestRenumberIsNotBlockedByAFrozenReferenceToTheDocumentStaying: the frozen
// document refers to the identifier the rename leaves alone.
func TestRenumberIsNotBlockedByAFrozenReferenceToTheDocumentStaying(t *testing.T) {
	files := sharedNumber()
	files["rfc/0002-cache.md"] = accepted("RFC-0002", "Cache", "RFC-0001")
	r := repotest.New(t, files)
	_, unchanged, err := repo.Renumber(r, "rfc/0001-newthing.md", "", frozenIDs("RFC-0002"))
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0003-newthing.md")); err != nil {
		t.Errorf("the document was not renumbered: %v", err)
	}
	// Marked, because a frozen document cannot be repointed.
	if want := "rfc/0002-cache.md:7: depends: [RFC-0001] (frozen)"; len(unchanged) != 1 || unchanged[0] != want {
		t.Errorf("unchanged = %q, want [%q]", unchanged, want)
	}
}

// TestRenumberIsBlockedByAFrozenLinkToTheMovedFile: a link to the file does say
// which document it means, so a frozen one still blocks.
func TestRenumberIsBlockedByAFrozenLinkToTheMovedFile(t *testing.T) {
	r := repotest.New(t, sharedNumber())
	_, _, err := repo.Renumber(r, "rfc/0001-newthing.md", "", frozenIDs("RFC-0002"))
	// Named by path: the identifier names both documents.
	if err == nil || !strings.Contains(err.Error(), "rfc/0001-newthing.md is referenced by frozen RFC-0002 (rfc/0002-cache.md)") {
		t.Fatalf("err = %v; want the rename refused for the frozen link to the moved file", err)
	}
}

// slugTwins is an RFC and the ADR extracted from it, which commonly share a
// slug, and a document mentioning and linking to both.
func slugTwins(other string) map[string]string {
	return map[string]string{
		"rfc/0001-storage.md": draft("RFC-0001", "Storage"),
		"adr/0001-storage.md": draft("ADR-0001", "Storage"),
		"adr/0002-other.md":   draft("ADR-0002", "Other") + other,
	}
}

// TestRenumberRewritesOnlyLinksThatReachTheMovedFile: a link is followed by
// resolving it, not by finding the file name in the line.
func TestRenumberRewritesOnlyLinksThatReachTheMovedFile(t *testing.T) {
	other := "\nSee [the ADR](0001-storage.md), [the RFC](../rfc/0001-storage.md#proposal) and [d].\n\n" +
		"[A mirror](https://example.com/../../../rfc/0001-storage.md).\n\n" +
		"[Mail](mailto:x/../../rfc/0001-storage.md).\n\n" +
		"[Encoded](../rfc/0001%2Dstorage.md) [plain](../rfc/0001-storage.md).\n\n" +
		"[d]: <../rfc/0001-storage.md>\n\n" +
		"The draft was first saved as 0001-storage.md, and `0001-storage.md` is its name.\n\n" +
		"```\ncp notes.txt ../rfc/0001-storage.md [[RFC-0001]]\n```\n\n" +
		"<!-- [[RFC-0001]] -->\n"
	r := repotest.New(t, slugTwins(other))
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0005", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	body, err := os.ReadFile(r.File("adr/0002-other.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := draft("ADR-0002", "Other") +
		"\nSee [the ADR](0001-storage.md), [the RFC](../rfc/0005-storage.md#proposal) and [d].\n\n" +
		"[A mirror](https://example.com/../../../rfc/0001-storage.md).\n\n" +
		"[Mail](mailto:x/../../rfc/0001-storage.md).\n\n" +
		"[Encoded](../rfc/0005-storage.md) [plain](../rfc/0005-storage.md).\n\n" +
		"[d]: <../rfc/0005-storage.md>\n\n" +
		"The draft was first saved as 0001-storage.md, and `0001-storage.md` is its name.\n\n" +
		"```\ncp notes.txt ../rfc/0001-storage.md [[RFC-0001]]\n```\n\n" +
		"<!-- [[RFC-0001]] -->\n"
	if string(body) != want {
		t.Errorf("adr/0002-other.md =\n%s\nwant\n%s", body, want)
	}
}

// TestRenumberIsNotBlockedByAFrozenMentionOfTheFileName: only what would be
// rewritten blocks, and a mention is not.
func TestRenumberIsNotBlockedByAFrozenMentionOfTheFileName(t *testing.T) {
	r := repotest.New(t, slugTwins("\n[The ADR](0001-storage.md) was first saved as ../rfc/0001-storage.md.\n\n"+
		"```\n[[RFC-0001]] [x](../rfc/0001-storage.md)\n```\n"))
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0005", frozenIDs("ADR-0002")); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
}

// TestRenumberFollowsTheMovedDocumentsReferencesToItself: it moves with them,
// so a link to itself would otherwise be left pointing at a file that is gone.
func TestRenumberFollowsTheMovedDocumentsReferencesToItself(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-storage.md": draft("RFC-0001", "Storage") + "\nAs [[RFC-0001]] says in [its abstract](0001-storage.md#abstract).\n",
	})
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0005", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	body, err := os.ReadFile(r.File("rfc/0005-storage.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "As [[RFC-0005]] says in [its abstract](0005-storage.md#abstract).\n"; !strings.HasSuffix(string(body), want) {
		t.Errorf("rfc/0005-storage.md =\n%s\nwant it to end %q", body, want)
	}
}

// TestRenumberReportsTheMovedDocumentsOwnSharedIdentifier: under a shared
// identifier its own wiki link cannot say which document it means either.
func TestRenumberReportsTheMovedDocumentsOwnSharedIdentifier(t *testing.T) {
	files := sharedNumber()
	files["rfc/0001-newthing.md"] = draft("RFC-0001", "New thing") + "\nUnlike [[RFC-0001]], or [[RFC-0001]].\n"
	delete(files, "rfc/0002-cache.md")
	r := repotest.New(t, files)
	_, unchanged, err := repo.Renumber(r, "rfc/0001-newthing.md", "", nothingFrozen)
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	// Once for the line, however many it holds.
	if want := "rfc/0002-newthing.md:18: Unlike [[RFC-0001]], or [[RFC-0001]]."; len(unchanged) != 1 || unchanged[0] != want {
		t.Errorf("unchanged = %q, want [%q]", unchanged, want)
	}
}

// TestRenumberRewritesLinksInACRLFFile keeps the carriage return out of the
// destination it rewrites, and in the line.
func TestRenumberRewritesLinksInACRLFFile(t *testing.T) {
	other := "\r\nSee [the RFC](../rfc/0001-storage.md) and [[RFC-0001]].\r\n\r\n[d]: ../rfc/0001-storage.md\r\n"
	r := repotest.New(t, slugTwins(other))
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0005", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	body, err := os.ReadFile(r.File("adr/0002-other.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "\r\nSee [the RFC](../rfc/0005-storage.md) and [[RFC-0005]].\r\n\r\n[d]: ../rfc/0005-storage.md\r\n"
	if !strings.HasSuffix(string(body), want) {
		t.Errorf("adr/0002-other.md ends %q, want %q", string(body)[len(body)-len(want):], want)
	}
}

// TestRenumberRefusesALinkItCannotRewriteAsWritten: the link resolves to the
// file once decoded, but the file name is not where the text as written puts
// it, so replacing it there would write a different destination.
func TestRenumberRefusesALinkItCannotRewriteAsWritten(t *testing.T) {
	for _, link := range []string{"..%2Frfc%2F0001-storage.md", "../rfc/0001-storage.md%23x"} {
		r := repotest.New(t, slugTwins("\nSee [the RFC]("+link+").\n"))
		before, err := os.ReadFile(r.File("adr/0002-other.md"))
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = repo.Renumber(r, "RFC-0001", "RFC-0005", nothingFrozen)
		if err == nil || !strings.Contains(err.Error(), "adr/0002-other.md:18") {
			t.Errorf("%s: err = %v; want the rename refused, naming the link", link, err)
		}
		if after, _ := os.ReadFile(r.File("adr/0002-other.md")); string(after) != string(before) {
			t.Errorf("%s: the document was rewritten despite the refusal", link)
		}
		if _, err := os.Stat(r.File("rfc/0001-storage.md")); err != nil {
			t.Errorf("%s: the document was moved despite the refusal: %v", link, err)
		}
	}
}

// TestRenumberWritesNothingWhenAWriteFails holds the promise the refusal makes:
// the referring documents were written first and left naming a number no file
// carried when a later write failed.
func TestRenumberWritesNothingWhenAWriteFails(t *testing.T) {
	files := map[string]string{
		"rfc/0001-base.md":  draft("RFC-0001", "Base"),
		"rfc/0002-later.md": draftDepending("RFC-0002", "Later", "RFC-0001"),
		"spec/zz.md":        "---\ntitle: Zz\nincludes: [RFC-0001]\n---\n\n# Zz\n\nSee [[RFC-0001]].\n",
	}
	for name, block := range map[string]func(t *testing.T, root string){
		"the target is a directory": func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, "rfc", "0009-base.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"a referrer is read-only": func(t *testing.T, root string) {
			if os.Getuid() == 0 {
				t.Skip("root writes through a read-only mode")
			}
			if err := os.Chmod(filepath.Join(root, "spec", "zz.md"), 0o444); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := repotest.New(t, files)
			root := r.Config().RootDir()
			block(t, root)
			if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", nothingFrozen); err == nil {
				t.Fatal("Renumber succeeded despite the blocked write")
			}
			for p, want := range files {
				got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
				if err != nil || string(got) != want {
					t.Errorf("%s was changed or removed (%v):\n%s", p, err, got)
				}
			}
			info, err := os.Stat(filepath.Join(root, "rfc", "0009-base.md"))
			if blocked := name == "the target is a directory"; blocked != (err == nil && info.IsDir()) || err == nil && !info.IsDir() {
				t.Errorf("rfc/0009-base.md was left behind or the directory in its place removed (%v)", err)
			}
		})
	}
}

func TestRenumberFollowsATermsNamedBy(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md": draft("RFC-0001", "Base"),
		"term/widget.md":   "---\ntitle: Widget\nformerly: []\nnamed_by: RFC-0001\n---\n\n# Widget\n\nA definition.\n",
	})
	if _, _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(r.Config().RootDir(), "term", "widget.md"))
	if err != nil || !strings.Contains(string(body), "named_by: RFC-0009\n") {
		t.Errorf("named_by was not followed (%v):\n%s", err, body)
	}
}
