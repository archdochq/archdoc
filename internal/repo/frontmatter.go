package repo

import "time"

// Status is the lifecycle status of an RFC or ADR. Spec pages and refs have
// none.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusProposed  Status = "proposed"
	StatusAccepted  Status = "accepted"
	StatusRejected  Status = "rejected"
	StatusWithdrawn Status = "withdrawn"
)

// Statuses is every valid status.
var Statuses = []Status{StatusDraft, StatusProposed, StatusAccepted, StatusRejected, StatusWithdrawn}

// Terminal reports whether a status is final. A document in a terminal status
// is frozen once it reaches the branch.
func (s Status) Terminal() bool {
	return s == StatusAccepted || s == StatusRejected || s == StatusWithdrawn
}

// FrontMatter holds every field any document type may carry. Fields that do
// not apply to a type stay zero; lint decides which are permitted where.
type FrontMatter struct {
	ID        string
	Title     string
	Status    Status
	Created   time.Time
	Decided   time.Time
	Depends   []string
	Updates   []string
	Obsoletes []string
	Includes  []string
	Verified  time.Time
	// Formerly lists a term's previous names, oldest first. Generation gives
	// each one an anchor, so a link written before a rename still resolves.
	Formerly []string
	// NamedBy is the document that introduced a term. It records where the
	// term came from and is not an inclusion: a term is not implementation.
	NamedBy string
	// Backfilled is the date the document was written to record a decision
	// taken earlier. Optional, and the only optional key: created and decided
	// carry the historical dates, this one says when they were written down.
	Backfilled time.Time

	// present maps each key that was written to its 1-based line, so that a key
	// present with an empty value is distinguishable from one that is absent,
	// and a finding about either can point at it. `decided:` on a draft is the
	// case that requires the distinction.
	present map[string]int
	// unknown lists keys written in the block that belong to no schema.
	unknown []UnknownKey
}

// Has reports whether a key was written in the block, whatever its value.
func (f FrontMatter) Has(key string) bool { return f.present[key] != 0 }

// LineOf is the line a key was written on, or 0 if it was not.
func (f FrontMatter) LineOf(key string) int { return f.present[key] }

// UnknownKey is a front matter key belonging to no schema, and where it was
// written.
type UnknownKey struct {
	Name string
	Line int
}

// Unknown lists keys written in the block that belong to no schema.
func (f FrontMatter) Unknown() []UnknownKey { return f.unknown }
