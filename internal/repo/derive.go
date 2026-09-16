package repo

import "slices"

// Reverse relationships and the facts derived from them. Forward relationships
// live in front matter; these are computed and never written to a document,
// because a frozen document cannot be edited to record what happened to it
// later.

// ByID returns the document with this identifier, or nil. The identifier is the
// one derived from the type and filename, not whatever the front matter claims.
func (r *Repo) ByID(id string) *Document { return r.byID[id] }

// ByPage returns the spec page with this name, or nil.
func (r *Repo) ByPage(page string) *Document { return r.byPage[page] }

// ByPath returns the document at this path relative to root, or nil.
func (r *Repo) ByPath(path string) *Document { return r.byPath[path] }

// derive fills in every reverse relationship and the facts that follow from
// them. It runs once, after every document has been parsed. A forward
// reference to a document that does not exist is ignored here; L04 reports it.
func (r *Repo) derive() {
	r.index()

	for _, d := range r.documents {
		r.contribute(d)
	}
	for _, d := range r.documents {
		slices.Sort(d.UpdatedBy)
		slices.Sort(d.ObsoletedBy)
		slices.Sort(d.DependedOnBy)
		slices.Sort(d.IncludedIn)
	}
	r.deriveFacts()
}

// index builds the lookups every other pass reads.
func (r *Repo) index() {
	r.byID = map[string]*Document{}
	r.byPage = map[string]*Document{}
	r.byPath = map[string]*Document{}

	for _, d := range r.documents {
		r.byPath[d.Path] = d
		if d.ID != "" {
			// A duplicate identifier keeps the first document; L03 reports it.
			if _, seen := r.byID[d.ID]; !seen {
				r.byID[d.ID] = d
			}
		}
		if d.Page != "" {
			r.byPage[d.Page] = d
		}
	}
}

// contribute records one document in the reverse lists of everything it names.
func (r *Repo) contribute(d *Document) {
	// A numbered document whose filename carries no number has no identifier,
	// and naming it in a reverse list would leave an empty entry in the index.
	// L02 already reports the filename; this keeps the damage there. A spec
	// page has no identifier by design and contributes through includes, which
	// is why that list is handled separately below.
	if d.ID != "" {
		for _, id := range d.FrontMatter.Updates {
			if target := r.byID[id]; target != nil {
				target.UpdatedBy = append(target.UpdatedBy, d.ID)
			}
		}
		for _, id := range d.FrontMatter.Obsoletes {
			if target := r.byID[id]; target != nil {
				target.ObsoletedBy = append(target.ObsoletedBy, d.ID)
			}
		}
		for _, id := range d.FrontMatter.Depends {
			if target := r.byID[id]; target != nil {
				target.DependedOnBy = append(target.DependedOnBy, d.ID)
			}
		}
	}

	if d.Type != TypeSpec {
		return
	}
	for _, id := range d.FrontMatter.Includes {
		if target := r.byID[id]; target != nil {
			target.IncludedIn = append(target.IncludedIn, d.Page)
		}
	}
}

// deriveFacts computes what follows from the reverse relationships. Effective
// obsolescence comes first, because staleness is defined in terms of it.
func (r *Repo) deriveFacts() {
	for _, d := range r.documents {
		for _, id := range d.ObsoletedBy {
			if by := r.byID[id]; by != nil && by.FrontMatter.Status == StatusAccepted {
				d.EffectivelyObsolete = true
				break
			}
		}
		// The glossary is excluded: it defines words, it does not describe
		// behaviour. The shipped glossary template asks for the RFC that named
		// a term to go in includes, so counting that as implementation made the
		// tool report as built whatever it had just instructed someone to cite.
		d.Implemented = d.FrontMatter.Status == StatusAccepted && len(ImplementedIn(d.IncludedIn)) > 0
	}

	for _, d := range r.documents {
		if d.Type != TypeSpec {
			continue
		}
		for _, id := range d.FrontMatter.Includes {
			if included := r.byID[id]; included != nil && included.EffectivelyObsolete {
				d.Stale = true
				break
			}
		}
	}
}

// ImplementedIn is the spec pages whose inclusion of a document means the
// document has been built, which is every page but the glossary.
//
// The full IncludedIn list is kept as written, because "which spec pages
// reference this" is a separate and factual question.
func ImplementedIn(includedIn []string) []string {
	out := make([]string, 0, len(includedIn))
	for _, page := range includedIn {
		if page != GlossaryPage {
			out = append(out, page)
		}
	}
	return out
}
