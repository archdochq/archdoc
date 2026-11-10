package lint

import (
	"maps"
	"path/filepath"
	"slices"
)

// acknowledge leaves out each warning archdoc.json accepts, and returns what is
// left and L22's findings on the acknowledgements matching none. An
// acknowledgement matches on path and rule, so it accepts every warning that
// rule reports on the document.
//
// A finding is acknowledged only as a warning on a frozen document. An error
// is always a fault someone can clear, and a warning on a document still open
// to editing is cleared by editing it.
func acknowledge(ctx Context, findings []Finding) (kept, stale []Finding) {
	r := ctx.Repo()
	cfg := r.Config()
	frozen := func(path string) bool {
		d := r.ByPath(path)
		return d != nil && ctx.Frozen(d)
	}

	matched := map[string]map[string]bool{}
	for _, f := range findings {
		if f.Severity == Warning && slices.Contains(cfg.Acknowledged[f.Path], f.Rule) && frozen(f.Path) {
			if matched[f.Path] == nil {
				matched[f.Path] = map[string]bool{}
			}
			matched[f.Path][f.Rule] = true
			continue
		}
		kept = append(kept, f)
	}

	// Both are absolute, so Rel cannot fail.
	at, _ := filepath.Rel(cfg.RootDir(), cfg.Path)
	at = filepath.ToSlash(at)
	rep := report{}
	for _, path := range slices.Sorted(maps.Keys(cfg.Acknowledged)) {
		switch {
		case r.ByPath(path) == nil:
			rep.addf(Error, at, 0, "acknowledged %s is not a document: remove it from acknowledged", path)
			continue
		case !frozen(path):
			rep.addf(Error, at, 0, "acknowledged %s is not frozen: remove it from acknowledged", path)
			continue
		}
		for _, code := range cfg.Acknowledged[path] {
			switch {
			case !slices.ContainsFunc(Rules, func(r Rule) bool { return r.Code == code && !r.Merge() }):
				rep.addf(Error, at, 0, "acknowledged code %s on %s is not an L rule: remove it from acknowledged", code, path)
			case !matched[path][code]:
				rep.addf(Error, at, 0, "acknowledged rule %s reports no warning on %s: remove it from acknowledged", code, path)
			}
		}
	}
	for _, f := range rep.findings {
		f.Rule = "L22"
		stale = append(stale, f)
	}
	return kept, stale
}
