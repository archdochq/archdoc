package repo

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	var names []string
	for _, k := range Keys(t) {
		names = append(names, k.Name)
	}
	return names
}

// Key is one front matter key of a type.
type Key struct {
	Name string
	Type ValueType
	// Required is whether the key must be present, whatever its value, and
	// MayBeEmpty whether it may be present with no value.
	Required   bool
	MayBeEmpty bool
	// Values lists what an enum may hold.
	Values []string
}

// ValueType is the type of a front matter key's value.
type ValueType string

const (
	ValueString ValueType = "string"
	ValueDate   ValueType = "date"
	ValueList   ValueType = "list"
	ValueEnum   ValueType = "enum"
)

// Keys lists a type's front matter keys, required keys first.
//
// A type's schema may only ever gain optional keys. A new required key would
// put every frozen document in every existing repository in violation of L01,
// and L11 forbids the edit that would add it. A new type's keys may be
// required, as no existing document is of that type.
func Keys(t Type) []Key {
	id := Key{Name: "id", Type: ValueString, Required: true}
	title := Key{Name: "title", Type: ValueString, Required: true}
	list := func(name string) Key {
		return Key{Name: name, Type: ValueList, Required: true, MayBeEmpty: true}
	}
	switch t {
	case TypeSpec:
		return []Key{title, list("includes")}
	case TypeRef:
		return []Key{id, title,
			{Name: "verified", Type: ValueDate, Required: true},
			{Name: "as_of", Type: ValueDate},
		}
	case TypeTerm:
		return []Key{title, list("formerly"), {Name: "named_by", Type: ValueString, Required: true, MayBeEmpty: true}}
	case TypeGroup:
		return []Key{title, list("documents")}
	default:
		statuses := make([]string, len(Statuses))
		for i, s := range Statuses {
			statuses[i] = string(s)
		}
		return []Key{id, title,
			{Name: "status", Type: ValueEnum, Required: true, Values: statuses},
			{Name: "created", Type: ValueDate, Required: true},
			{Name: "decided", Type: ValueDate, Required: true, MayBeEmpty: true},
			list("depends"), list("updates"), list("obsoletes"),
			{Name: "backfilled", Type: ValueDate, MayBeEmpty: true},
		}
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

	// Front matter lines are offset by the opening delimiter.
	const offset = 1
	var root yaml.Node
	if err := yaml.Unmarshal(block, &root); err != nil {
		fm.unread = true
		return fm, []Problem{{Line: faultLine(block, err) + offset, Message: "front matter is not valid YAML: " + yamlProblem(err)}}
	}
	if len(root.Content) == 0 {
		return fm, problems
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		fm.unread = true
		return fm, []Problem{{Line: mapping.Line + offset, Message: "front matter is not a mapping"}}
	}
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
				Message: decodeError(key, err),
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
				Message: decodeError(key, err),
			})
			return
		}
		// A null item decodes to "", which names nothing and would otherwise
		// vanish from every rule that reads the list.
		for _, item := range n.Content {
			if item.Tag == "!!null" {
				problems = append(problems, Problem{
					Line:    item.Line + offset,
					Message: fmt.Sprintf("%s holds an empty item", key),
				})
			}
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
				Message: decodeError(key, err),
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
	date("as_of", &fm.AsOf)
	list("depends", &fm.Depends)
	list("updates", &fm.Updates)
	list("obsoletes", &fm.Obsoletes)
	list("includes", &fm.Includes)
	list("formerly", &fm.Formerly)
	str("named_by", &fm.NamedBy)
	list("documents", &fm.Documents)

	return fm, problems
}

// faultLine is the line of the block on which it stops being valid YAML: the
// first from which the block cut after that line or any later one fails as the
// whole block does. Read from the end, because a construct over several lines
// cut part way through can fail with the same message as the fault, on a line
// before it that is valid. yaml's own line number cannot be used, because it
// counts a scanner error's line from 1 and a parser error's from 0, names the
// line a construct opened on for some errors, and names none on the first
// line.
func faultLine(block []byte, err error) int {
	lines := bytes.Split(block, []byte("\n"))
	want := yamlProblem(err)
	k := len(lines)
	for ; k > 1; k-- {
		var n yaml.Node
		if e := yaml.Unmarshal(bytes.Join(lines[:k-1], []byte("\n")), &n); e == nil || yamlProblem(e) != want {
			break
		}
	}
	return k
}

// yamlProblem is a YAML error without its prefix and line number.
func yamlProblem(err error) string {
	return yamlPrefixPattern.ReplaceAllString(err.Error(), "")
}

var yamlPrefixPattern = regexp.MustCompile(`^yaml: (line \d+: )?`)

// decodeError is a key's decoding error on one line. yaml reports a type error
// as a list, one per line with its own line number, counted from the value
// rather than the file; the finding already carries the file's line.
func decodeError(key string, err error) string {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Sprintf("%s: %s", key, strings.ReplaceAll(err.Error(), "\n", " "))
	}
	reasons := make([]string, len(typeErr.Errors))
	for i, e := range typeErr.Errors {
		reasons[i] = errorLinePattern.ReplaceAllString(e, "")
	}
	return fmt.Sprintf("%s: %s", key, strings.Join(reasons, "; "))
}

// errorLinePattern is the line number yaml puts before each type error.
var errorLinePattern = regexp.MustCompile(`^line \d+: `)
