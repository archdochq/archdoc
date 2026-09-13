package repo

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetField replaces the value of one front matter key, on the line it was
// written, and leaves everything else exactly as it was: the order of the other
// keys, their spacing, and any comments. Re-serialising the block through a
// YAML encoder would be shorter and would quietly discard all three.
func SetField(source []byte, key string, line int, value string) ([]byte, error) {
	if line <= 0 {
		return nil, fmt.Errorf("front matter has no %q key to rewrite", key)
	}
	lines := bytes.Split(source, []byte("\n"))
	if line > len(lines) {
		return nil, fmt.Errorf("front matter key %q is recorded on line %d, past the end of the file", key, line)
	}
	closing, ok := closingDelimiter(lines)
	if !ok {
		return nil, fmt.Errorf("no front matter block to rewrite")
	}
	if line >= closing+1 {
		return nil, fmt.Errorf("line %d is outside the front matter block, which ends at line %d", line, closing+1)
	}

	original := string(lines[line-1])
	// Keep any carriage return so a CRLF file stays CRLF.
	ending := ""
	if strings.HasSuffix(original, "\r") {
		original, ending = strings.TrimSuffix(original, "\r"), "\r"
	}

	indent, rest, found := cutIndent(original)
	if !found || !strings.HasPrefix(rest, key+":") {
		return nil, fmt.Errorf("line %d is not the %q key: %q", line, key, original)
	}
	comment := trailingComment(strings.TrimPrefix(rest, key+":"))

	rebuilt := indent + key + ":"
	if value != "" {
		rebuilt += " " + value
	}
	rebuilt += comment + ending

	// A value continues onto the lines beneath the key. Replacing only the key's
	// line would leave the rest orphaned, which does not parse.
	end := valueEnd(lines, line-1, len(indent))

	// Those continuation lines are about to be replaced wholesale, so a comment
	// among them would be deleted. The promise is that a rewrite preserves the
	// order of the other keys, their spacing and any comments, and silently
	// dropping one is a worse outcome than refusing the edit. A comment after
	// the value is outside the extent and survives either way.
	for _, line := range lines[line:end] {
		if strings.HasPrefix(strings.TrimSpace(text(line)), "#") {
			return nil, fmt.Errorf(
				"rewriting %q would delete a comment written inside its value; edit the front matter by hand", key)
		}
	}
	rewritten := append(lines[:line-1:line-1], []byte(rebuilt))
	out := bytes.Join(append(rewritten, lines[end:]...), []byte("\n"))

	// The last line of defence. However the old value was written, the result
	// must still be readable: front matter that no longer decodes takes the
	// document's every derived relationship with it, and SetField could not
	// repair it afterwards because it could no longer find the key.
	if !frontMatterParses(out) {
		return nil, fmt.Errorf(
			"rewriting %q would leave front matter that does not parse; the value spans lines in a shape this cannot rewrite safely", key)
	}
	return out, nil
}

// frontMatterParses reports whether a document's YAML block still reads. It
// asks only whether the structure is intact, not whether the fields are valid:
// a document with a bad date is lint's business, not a reason to refuse an
// unrelated edit.
func frontMatterParses(source []byte) bool {
	block, _, _, ok := splitFrontMatter(source)
	if !ok {
		return false
	}
	var node yaml.Node
	return yaml.Unmarshal(block, &node) == nil
}

// closingDelimiter is the index of the front matter block's closing ---.
//
// isDelimiter is shared with the reading side, so the two cannot come to
// disagree about what opens a block. They did once: the reader trimmed a byte
// order mark and this did not, so every transition on a document saved by a
// Windows editor failed on a block the user could plainly see.
func closingDelimiter(lines [][]byte) (int, bool) {
	if len(lines) == 0 || !isDelimiter(lines[0]) {
		return 0, false
	}
	for i := 1; i < len(lines); i++ {
		if isDelimiter(lines[i]) {
			return i, true
		}
	}
	return 0, false
}

// valueEnd is the index one past the last line of a key's value.
//
// YAML lets a value continue beneath its key in several shapes, and the only
// one indentation alone recognises is the indented block. A sequence may sit at
// the key's own column, and items may be separated by blank lines or comments.
// Deriving the extent from indentation alone orphans the rest, which is how
// this produced unparseable front matter before.
//
// Only lines that certainly belong to the value move the end, so trailing blank
// lines and comments after it stay where they are.
func valueEnd(lines [][]byte, keyIndex, keyIndent int) int {
	// A flow collection ends when its brackets balance, wherever that falls.
	if depth := bracketDepth(text(lines[keyIndex])); depth > 0 {
		for end := keyIndex + 1; end < len(lines); end++ {
			if depth += bracketDepth(text(lines[end])); depth <= 0 {
				return end + 1
			}
		}
		return len(lines)
	}

	last := keyIndex + 1
	for i := keyIndex + 1; i < len(lines); i++ {
		line := text(lines[i])
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue // a blank line may sit inside a block value
		case strings.HasPrefix(trimmed, "#"):
			continue // so may a comment, and one after the value is not ours
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		isItem := trimmed == "-" || strings.HasPrefix(trimmed, "- ")
		if indent > keyIndent || (indent == keyIndent && isItem) {
			last = i + 1
			continue
		}
		break
	}
	return last
}

// bracketDepth is how far a line opens or closes flow collections, ignoring
// brackets inside a quoted scalar.
func bracketDepth(line string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '#':
			return depth // the rest is a comment
		case c == '[', c == '{':
			depth++
		case c == ']', c == '}':
			depth--
		}
	}
	return depth
}

// LineEnding is the ending a file uses, so that an edit does not leave it with
// a mixture. The first line decides it. Three packages were deciding this
// separately, and one of them forgot.
func LineEnding(source []byte) string {
	if first, _, found := bytes.Cut(source, []byte("\n")); found && bytes.HasSuffix(first, []byte("\r")) {
		return "\r\n"
	}
	return "\n"
}

// text is a line without its carriage return.
func text(line []byte) string { return string(bytes.TrimRight(line, "\r")) }

// cutIndent splits leading whitespace from the rest of a line.
func cutIndent(line string) (indent, rest string, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)], trimmed, trimmed != ""
}

// trailingComment returns the comment at the end of a value, with the
// whitespace that preceded it, or the empty string. A `#` inside quotes is part
// of the value, not the start of a comment.
func trailingComment(value string) string {
	var quote byte
	started := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case !started && (c == '\'' || c == '"'):
			// Only a quote at the start of the value opens a quoted scalar.
			// An apostrophe inside a word is part of the text.
			quote = c
		case c == '#' && i > 0 && (value[i-1] == ' ' || value[i-1] == '\t'):
			// The comment keeps the spacing that separated it from the value.
			start := i
			for start > 0 && (value[start-1] == ' ' || value[start-1] == '\t') {
				start--
			}
			return value[start:]
		}
		if c != ' ' && c != '\t' {
			started = true
		}
	}
	return ""
}
