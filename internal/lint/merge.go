package lint

import (
	"fmt"
	"maps"
	"slices"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/repo"
)

// The merge rules judge a pull request, as checked out merged into the
// configured branch, against that branch. Each change is judged as one step
// from the base, so they read the base and never the history between.

// WithBase returns the context with the record on the configured branch, which
// makes Run apply the merge rules.
func (c Context) WithBase(b *changes.Base) Context {
	c.base = b
	return c
}

// arrival tells, for each document in the working tree, what it was on the
// base.
type arrival struct {
	base *changes.Base
	// renamed holds the base's numbered documents at a path the working tree
	// no longer has, by identifier: a working-tree document carrying one of
	// those identifiers at a new path was renamed, not added.
	renamed map[string]*repo.Document
}

func arrivals(ctx Context) arrival {
	inTree := map[string]bool{}
	for _, d := range ctx.Repo().Documents() {
		inTree[d.Path] = true
	}
	a := arrival{base: ctx.base, renamed: map[string]*repo.Document{}}
	for _, p := range basePaths(ctx) {
		d := ctx.base.Documents[p]
		if _, taken := a.renamed[d.ID]; d.ID != "" && !inTree[p] && !taken {
			a.renamed[d.ID] = d
		}
	}
	return a
}

// basePaths is every path on the base, sorted. Where the base itself carries
// one identifier twice, the first path is the one judged, so that findings
// never come from map order.
func basePaths(ctx Context) []string {
	return slices.Sorted(maps.Keys(ctx.base.Documents))
}

// was is the document as it stood on the base, and false for one arriving.
func (a arrival) was(d *repo.Document) (*repo.Document, bool) {
	if on, ok := a.base.Documents[d.Path]; ok {
		return on, true
	}
	if d.ID != "" {
		if on, ok := a.renamed[d.ID]; ok {
			return on, true
		}
	}
	return nil, false
}

// M01 reports an RFC or ADR arriving as a draft. A pull request is where a
// document is offered for comment, so what it lands has been proposed.
func M01(ctx Context) []Finding {
	var rep report
	a := arrivals(ctx)
	for _, d := range ctx.Repo().Documents() {
		if !d.Type.HasLifecycle() || d.FrontMatter.Status != repo.StatusDraft {
			continue
		}
		if _, ok := a.was(d); ok {
			continue
		}
		message := "arrives on " + ctx.base.Branch + " as a draft; propose it before it merges"
		if d.ID != "" {
			message += ": archdoc propose " + d.ID
		}
		rep.add(Error, d.Path, d.FrontMatter.LineOf("status"), message)
	}
	return rep.findings
}

// M02 names which of two documents sharing an identifier to renumber: the one
// arriving, since the other already holds the number on the base. L03 reports
// both without saying which.
func M02(ctx Context) []Finding {
	inTree := map[string]bool{}
	for _, d := range ctx.Repo().Documents() {
		inTree[d.Path] = true
	}
	holders := map[string]*repo.Document{}
	for _, p := range basePaths(ctx) {
		d := ctx.base.Documents[p]
		if _, taken := holders[d.ID]; d.ID != "" && inTree[p] && !taken {
			holders[d.ID] = d
		}
	}

	var rep report
	a := arrivals(ctx)
	for _, d := range ctx.Repo().Documents() {
		if d.ID == "" {
			continue
		}
		if _, ok := a.was(d); ok {
			continue
		}
		if holder, ok := holders[d.ID]; ok {
			rep.errorf(d, d.FrontMatter.LineOf("id"),
				"%s is already %s's on %s; renumber this document instead: archdoc renumber %s",
				d.ID, holder.Path, ctx.base.Branch, d.Path)
		}
	}
	return rep.findings
}

// M03 reports a change of status the lifecycle does not permit from the status
// on the base, under the base's strict.
func M03(ctx Context) []Finding {
	var rep report
	a := arrivals(ctx)
	for _, d := range ctx.Repo().Documents() {
		to := d.FrontMatter.Status
		if !d.Type.HasLifecycle() || !slices.Contains(repo.Statuses, to) {
			continue // L01 reports a status that is not one
		}
		from := repo.StatusDraft
		on, onBase := a.was(d)
		if onBase {
			from = on.FrontMatter.Status
			if from.Terminal() || !slices.Contains(repo.Statuses, from) {
				continue // L11 reports any change to a frozen document
			}
		} else if to == repo.StatusDraft {
			continue // M01 reports a draft arriving
		} else if !d.FrontMatter.Backfilled.IsZero() {
			continue // a backfilled document is written in any status but draft
		}
		if from == to {
			continue
		}
		permitted := from.Permitted(ctx.base.Strict)
		if slices.Contains(permitted, to) {
			continue
		}
		message := fmt.Sprintf("moves from %s on %s to %s, which is not a permitted transition", from, ctx.base.Branch, to)
		if !onBase {
			message = fmt.Sprintf("arrives on %s as %s, which is not a permitted transition from draft", ctx.base.Branch, to)
		}
		if from == repo.StatusDraft && ctx.base.Strict {
			message += "; with strict on, propose it here and give the verdict once the proposal has merged"
		}
		rep.add(Error, d.Path, d.FrontMatter.LineOf("status"), message)
	}
	return rep.findings
}

// M04 reports an RFC or ADR proposed on the base that the working tree no
// longer holds under its identifier: deleted, or renumbered. A proposed
// document no longer wanted is withdrawn, so that its number stays taken. A
// draft may be deleted, and a terminal document's deletion is L11's.
func M04(ctx Context) []Finding {
	ids := map[string]bool{}
	for _, d := range ctx.Repo().Documents() {
		if d.ID != "" {
			ids[d.ID] = true
		}
	}
	var rep report
	for _, d := range ctx.base.Documents {
		if !d.Type.HasLifecycle() || d.FrontMatter.Status != repo.StatusProposed || d.ID == "" || ids[d.ID] {
			continue
		}
		rep.addf(Error, d.Path, 0,
			"proposed on %s and deleted or renumbered here; withdraw it instead: archdoc withdraw %s",
			ctx.base.Branch, d.ID)
	}
	return rep.findings
}
