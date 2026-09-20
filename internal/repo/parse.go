package repo

import (
	"bytes"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// DateLayout is the ISO 8601 date form every date field uses.
const DateLayout = "2006-01-02"

// StartOfDay is an instant's calendar date at midnight UTC, which is how a date
// written in front matter is held once parsed.
//
// It lives beside DateLayout because every comparison between a written date
// and "now" needs it. Comparing a parsed date against a wall-clock instant
// makes the answer depend on the hour the command ran and the zone it ran in:
// east of UTC it read today as the future, and west of it read tomorrow as the
// past. That was a defect twice, in two packages, before this was shared.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Problem is something wrong with a document, found while reading it. Lint
// turns problems into findings; repo does not decide severity or rule identity.
type Problem struct {
	// Line is 1-based within the document, or 0 when it is not known.
	Line    int
	Message string
}

// Schema lists the keys a type's front matter may carry. A key outside this
// list is an unknown field.
func Schema(t Type) []string {
	switch t {
	case TypeRFC, TypeADR:
		return append(RequiredKeys(t), "backfilled")
	default:
		return RequiredKeys(t)
	}
}

// RequiredKeys lists the keys that must be present, whatever their value.
//
// The schema may only ever gain optional keys. A new required key would put
// every frozen document in every existing repository in violation of L01, and
// L11 forbids the edit that would add it.
func RequiredKeys(t Type) []string {
	switch t {
	case TypeSpec:
		return []string{"title", "includes"}
	case TypeRef:
		return []string{"id", "title", "verified"}
	default:
		return []string{"id", "title", "status", "created", "decided", "depends", "updates", "obsoletes"}
	}
}

// StatusIn reports the status recorded in a document's front matter, without
// reading the rest of it. L11 uses it to ask what a document looked like on the
// branch.
func StatusIn(source []byte) Status {
	block, _, _, ok := splitFrontMatter(source)
	if !ok {
		return ""
	}
	fm, _ := parseFrontMatter(TypeRFC, block)
	return fm.Status
}

// isDelimiter reports whether a line is a front matter delimiter. Several
// Windows editors write a UTF-8 BOM before it, and a CRLF file leaves a
// carriage return after it; neither changes what the line means. The bytes are
// not trimmed from the document itself, because every writer splices by offset
// into the file as the author wrote it, and a frozen document that silently
// gained or lost a BOM would read as edited.
func isDelimiter(line []byte) bool {
	line = bytes.TrimPrefix(line, []byte("\ufeff"))
	return string(bytes.TrimRight(line, "\r")) == "---"
}

// splitFrontMatter divides a document into its YAML block and its body. The
// block excludes the delimiters. ok is false when the document does not open
// with a front matter block.
func splitFrontMatter(source []byte) (block, body []byte, bodyLine int, ok bool) {
	lines := bytes.Split(source, []byte("\n"))
	if !isDelimiter(lines[0]) {
		return nil, source, 1, false
	}
	for i := 1; i < len(lines); i++ {
		if isDelimiter(lines[i]) {
			block = bytes.Join(lines[1:i], []byte("\n"))
			body = bytes.Join(lines[i+1:], []byte("\n"))
			// Body begins on the line after the closing delimiter, 1-based.
			return block, body, i + 2, true
		}
	}
	return nil, source, 1, false
}

// parseFrontMatter decodes the YAML block for a document of the given type.
// Every field is decoded separately so that one bad value does not hide the
// rest, and so each problem carries the line it was found on.
func parseFrontMatter(t Type, block []byte) (FrontMatter, []Problem) {
	fm := FrontMatter{present: map[string]int{}}
	var problems []Problem

	var root yaml.Node
	if err := yaml.Unmarshal(block, &root); err != nil {
		return fm, []Problem{{Line: 2, Message: fmt.Sprintf("front matter is not valid YAML: %v", err)}}
	}
	if len(root.Content) == 0 {
		return fm, problems
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return fm, []Problem{{Line: 2, Message: "front matter is not a mapping"}}
	}

	// Front matter lines are offset by the opening delimiter.
	const offset = 1
	nodes := map[string]*yaml.Node{}
	allowed := map[string]bool{}
	for _, k := range Schema(t) {
		allowed[k] = true
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if fm.present[key.Value] != 0 {
			problems = append(problems, Problem{
				Line:    key.Line + offset,
				Message: fmt.Sprintf("duplicate key %q", key.Value),
			})
			continue
		}
		fm.present[key.Value] = key.Line + offset
		if !allowed[key.Value] {
			fm.unknown = append(fm.unknown, UnknownKey{Name: key.Value, Line: key.Line + offset})
			continue
		}
		nodes[key.Value] = value
	}

	str := func(key string, into *string) {
		n, ok := nodes[key]
		if !ok {
			return
		}
		if err := n.Decode(into); err != nil {
			problems = append(problems, Problem{
				Line:    n.Line + offset,
				Message: fmt.Sprintf("%s: %v", key, err),
			})
		}
	}
	list := func(key string, into *[]string) {
		n, ok := nodes[key]
		if !ok {
			return
		}
		if n.Tag == "!!null" {
			*into = []string{}
			return
		}
		if err := n.Decode(into); err != nil {
			problems = append(problems, Problem{
				Line:    n.Line + offset,
				Message: fmt.Sprintf("%s: %v", key, err),
			})
		}
	}
	date := func(key string, into *time.Time) {
		n, ok := nodes[key]
		if !ok {
			return
		}
		// Decoded rather than read off n.Value, as the two closures above do.
		// For an alias node Value is the anchor's name and Tag is empty, so
		// reading it directly reported `"d" is not an ISO 8601 date` for a
		// perfectly good `decided: *d`, naming something the author never
		// wrote, and left the field zero so a second rule complained too.
		var written string
		if err := n.Decode(&written); err != nil {
			problems = append(problems, Problem{
				Line:    n.Line + offset,
				Message: fmt.Sprintf("%s: %v", key, err),
			})
			return
		}
		if written == "" {
			return
		}
		parsed, err := time.Parse(DateLayout, written)
		if err != nil {
			problems = append(problems, Problem{
				Line:    n.Line + offset,
				Message: fmt.Sprintf("%s: %q is not an ISO 8601 date", key, written),
			})
			return
		}
		*into = parsed
	}

	str("id", &fm.ID)
	str("title", &fm.Title)
	var status string
	str("status", &status)
	fm.Status = Status(status)
	date("created", &fm.Created)
	date("decided", &fm.Decided)
	date("verified", &fm.Verified)
	date("backfilled", &fm.Backfilled)
	list("depends", &fm.Depends)
	list("updates", &fm.Updates)
	list("obsoletes", &fm.Obsoletes)
	list("includes", &fm.Includes)

	return fm, problems
}
