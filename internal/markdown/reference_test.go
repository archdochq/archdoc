package markdown

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var updateCommonmark = flag.Bool("update-commonmark", false, "record the reference's parse of every case again")

const casesDir = "../../testdata/commonmark"

// reference reads REFERENCE: the command, its version, and its extensions.
func reference(t testing.TB) (command, version string, extensions []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(casesDir, "REFERENCE"))
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(data))
	if len(f) < 2 {
		t.Fatalf("REFERENCE holds %q, want the command, its version and its extensions", data)
	}
	return f[0], f[1], f[2:]
}

// referenceOnPath returns the reference command where the version REFERENCE
// names is on the path, and why it cannot be used where it is not.
func referenceOnPath(t testing.TB) (string, []string, error) {
	command, version, extensions := reference(t)
	path, err := exec.LookPath(command)
	if err != nil {
		return "", nil, fmt.Errorf("%s is not on the path", command)
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", nil, fmt.Errorf("%s --version: %v", command, err)
	}
	got := strings.Fields(string(out))
	if len(got) < 2 || got[1] != version {
		return "", nil, fmt.Errorf("%s is version %q, REFERENCE names %s", command, strings.SplitN(string(out), "\n", 2)[0], version)
	}
	return path, extensions, nil
}

// recordParse runs the reference over body and returns its XML.
func recordParse(path string, extensions []string, body []byte) ([]byte, error) {
	args := []string{"-t", "xml", "--sourcepos"}
	for _, e := range extensions {
		args = append(args, "-e", e)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = bytes.NewReader(body)
	return cmd.Output()
}

type refCase struct {
	name string // e.g. spec/0001
	body []byte
}

func cases(t testing.TB) []refCase {
	t.Helper()
	var out []refCase
	for _, dir := range []string{"spec", "extensions", "regression", "archdoc"} {
		files, err := filepath.Glob(filepath.Join(casesDir, dir, "*.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			body, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, refCase{dir + "/" + strings.TrimSuffix(filepath.Base(f), ".md"), body})
		}
	}
	if len(out) == 0 {
		t.Fatal("no cases under " + casesDir)
	}
	return out
}

// known reads KNOWN: a case, the path of the first node that differs, and why.
func known(t testing.TB) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(casesDir, "KNOWN"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 3 {
			t.Fatalf("KNOWN line %q wants a case, a node and a reason", line)
		}
		if _, ok := out[f[0]]; ok {
			t.Fatalf("KNOWN lists %s twice", f[0])
		}
		out[f[0]] = f[1]
	}
	return out
}

func TestTheOutlineReadsEachCaseAsTheReferenceDoes(t *testing.T) {
	if *updateCommonmark {
		path, extensions, err := referenceOnPath(t)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cases(t) {
			out, err := recordParse(path, extensions, c.body)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if err := os.WriteFile(filepath.Join(casesDir, c.name+".xml"), out, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	expected := known(t)
	seen := map[string]bool{}
	for _, c := range cases(t) {
		data, err := os.ReadFile(filepath.Join(casesDir, c.name+".xml"))
		if err != nil {
			t.Errorf("%s: no recorded parse; run go test ./internal/markdown -update-commonmark", c.name)
			continue
		}
		at, diff := compareWithReference(c.body, data)
		want, isKnown := expected[c.name]
		seen[c.name] = true
		switch {
		case isKnown && diff == "":
			t.Errorf("%s: listed in KNOWN at %s, but the outline now reads it as the reference does; remove the entry", c.name, want)
		case isKnown && at != want:
			t.Errorf("%s: KNOWN names %s, but the outline first differs at %s: %s", c.name, want, at, diff)
		case !isKnown && diff != "":
			t.Errorf("%s: the outline first differs at %s: %s", c.name, at, diff)
		}
	}
	for name := range expected {
		if !seen[name] {
			t.Errorf("KNOWN lists %s, which is no case", name)
		}
	}
}

// compareWithReference compares the outline of body with the reference's
// parse, returning the path of the first node that differs and how.
func compareWithReference(body, parse []byte) (string, string) {
	ref, err := readParse(parse)
	if err != nil {
		return "document", "the recorded parse does not read: " + err.Error()
	}
	return compareNodes("document", comparableOutline(Parse(body)), ref)
}

// cnode is a node of either tree, as the two are compared.
type cnode struct {
	name     string
	attrs    map[string]string
	text     string
	children []*cnode
}

var (
	footnoteReference  = regexp.MustCompile(`<<unknown>([^>]*?)/>`)
	footnoteDefinition = regexp.MustCompile(`<<unknown>`)
	footnoteClose      = regexp.MustCompile(`</<unknown>>`)
)

// readParse reads the reference's XML. Its footnote references and
// definitions are written as <unknown> tags that are not well-formed XML:
// the empty one is a reference, the other a definition.
func readParse(data []byte) (*cnode, error) {
	data = footnoteReference.ReplaceAll(data, []byte("<footnote_reference$1/>"))
	data = footnoteDefinition.ReplaceAll(data, []byte("<footnote_definition"))
	data = footnoteClose.ReplaceAll(data, []byte("</footnote_definition>"))
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	var stack []*cnode
	var root *cnode
	for {
		tok, err := d.Token()
		if err == io.EOF && root != nil && len(stack) == 0 {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		switch e := tok.(type) {
		case xml.StartElement:
			n := &cnode{name: e.Name.Local, attrs: map[string]string{}}
			if n.name == "footnote_reference" && len(stack) == 1 {
				// An empty definition is written as the empty tag too, and
				// definitions are the document's own children.
				n.name = "footnote_definition"
			}
			for _, a := range e.Attr {
				if a.Name.Local != "sourcepos" && a.Name.Space != "xmlns" && a.Name.Local != "xmlns" && a.Name.Local != "space" {
					n.attrs[a.Name.Local] = a.Value
				}
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(e)
			}
		}
	}
}

// comparableOutline turns the outline into the reference's shape: link
// reference definitions dropped, as the reference keeps none, and footnote
// definitions moved to the end in the order first referenced, dropping any
// nothing references.
func comparableOutline(doc *Node) *cnode {
	defs := map[string]*Node{}
	var order []*Node
	Walk(doc, func(n *Node) bool {
		if n.Type == FootnoteDefinition {
			if _, ok := defs[foldLabel(n.Label)]; !ok {
				defs[foldLabel(n.Label)] = n
			}
		}
		return true
	})
	used := map[*Node]bool{}
	Walk(doc, func(n *Node) bool {
		if n.Type == FootnoteReference {
			if d := defs[foldLabel(n.Label)]; d != nil && !used[d] {
				used[d] = true
				order = append(order, d)
			}
		}
		return true
	})
	root := convert(doc)
	for _, d := range order {
		root.children = append(root.children, convert(d))
	}
	return root
}

func foldLabel(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func convert(n *Node) *cnode {
	c := &cnode{name: n.Type.String(), attrs: map[string]string{}}
	switch n.Type {
	case Heading:
		c.attrs["level"] = strconv.Itoa(n.Level)
	case List:
		c.attrs["type"], c.attrs["tight"] = "bullet", strconv.FormatBool(n.Tight)
		if n.Ordered {
			c.attrs["type"], c.attrs["start"], c.attrs["delim"] = "ordered", strconv.Itoa(n.Number), "period"
			if n.Delimiter == ')' {
				c.attrs["delim"] = "paren"
			}
		}
	case CodeBlock:
		c.text = n.Text
		if n.Info != "" {
			c.attrs["info"] = n.Info
		}
	case HTMLBlock, RawHTML, CodeSpan:
		c.text = n.Text
	case Text:
		c.text = n.Text
	case LineBreak:
		c.name = "softbreak"
		if n.Hard {
			c.name = "linebreak"
		}
	case TableCell:
		if n.Align != "" {
			c.attrs["align"] = n.Align
		}
	case Link, Image:
		c.attrs["destination"], c.attrs["title"] = n.Destination, n.Title
		if n.AutoLink {
			c.children = []*cnode{{name: "text", attrs: map[string]string{}, text: n.Text}}
		}
	}
	for _, ch := range n.Children {
		if ch.Type == LinkReferenceDefinition || ch.Type == FootnoteDefinition {
			continue
		}
		c.children = append(c.children, convert(ch))
	}
	if n.Type == TableHeader {
		// The reference gives alignment only to header cells, and the
		// comparison reads it from there.
		return c
	}
	if n.Type == TableRow {
		for _, ch := range c.children {
			delete(ch.attrs, "align")
		}
	}
	return c
}

// mergeTexts joins adjacent text nodes, as the two trees divide text at
// different places.
func mergeTexts(children []*cnode) []*cnode {
	var out []*cnode
	for _, c := range children {
		if c.name == "text" && len(out) > 0 && out[len(out)-1].name == "text" {
			out[len(out)-1] = &cnode{name: "text", attrs: map[string]string{}, text: out[len(out)-1].text + c.text}
			continue
		}
		out = append(out, c)
	}
	var kept []*cnode
	for _, c := range out {
		if c.name == "text" && c.text == "" {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

// compareNodes compares the outline's node o with the reference's r.
func compareNodes(path string, o, r *cnode) (string, string) {
	if r.name == "tasklist" {
		completed := r.attrs["completed"] == "true"
		r = &cnode{name: "item", attrs: map[string]string{}, text: r.text, children: r.children}
		switch box := stripTaskBox(o); {
		case box == 0:
			return path, "the reference reads a task, and the item does not begin with its box"
		case completed != (box != ' '):
			return path, fmt.Sprintf("the box [%c] where the reference reads the task completed=%v", box, completed)
		}
	}
	if o.name != r.name {
		return path, fmt.Sprintf("%s where the reference has %s", o.name, r.name)
	}
	if o.name == "html_block" || o.name == "code_block" {
		// The reference ends a block's content with a line ending where the
		// body ends without one.
		o.text, r.text = strings.TrimSuffix(o.text, "\n"), strings.TrimSuffix(r.text, "\n")
	}
	if o.text != r.text && o.name != "document" && !isContainer(o.name) {
		return path, fmt.Sprintf("text %q where the reference has %q", o.text, r.text)
	}
	if diff := compareAttrs(o.attrs, r.attrs); diff != "" {
		return path, diff
	}
	oc, rc := mergeTexts(o.children), mergeTexts(r.children)
	count := map[string]int{}
	for i := 0; i < len(oc) && i < len(rc); i++ {
		name := rc[i].name
		if name == "tasklist" {
			name = "item"
		}
		at, diff := compareNodes(fmt.Sprintf("%s/%s[%d]", path, name, count[name]), oc[i], rc[i])
		count[name]++
		if diff != "" {
			return at, diff
		}
	}
	if len(oc) != len(rc) {
		return path, fmt.Sprintf("%d children where the reference has %d: %s against %s", len(oc), len(rc), names(oc), names(rc))
	}
	return "", ""
}

func isContainer(name string) bool {
	switch name {
	case "text", "code", "html_inline", "html_block", "code_block":
		return false
	}
	return true
}

func compareAttrs(o, r map[string]string) string {
	var keys []string
	for k := range r {
		keys = append(keys, k)
	}
	for k := range o {
		if _, ok := r[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if o[k] != r[k] {
			return fmt.Sprintf("%s %q where the reference has %q", k, o[k], r[k])
		}
	}
	return ""
}

func names(ns []*cnode) string {
	var s []string
	for _, n := range ns {
		s = append(s, n.name)
	}
	return "[" + strings.Join(s, " ") + "]"
}

// stripTaskBox removes the box a task's item begins with, as written, from
// the item's first text, returning the character inside it, or 0 where the
// item begins with none.
func stripTaskBox(item *cnode) byte {
	merged := mergeTexts(firstParagraph(item).children)
	if len(merged) == 0 || merged[0].name != "text" {
		return 0
	}
	t := merged[0].text
	if len(t) < 3 || t[0] != '[' || t[2] != ']' || !strings.ContainsRune(" xX", rune(t[1])) {
		return 0
	}
	merged[0] = &cnode{name: "text", attrs: map[string]string{}, text: strings.TrimLeft(t[3:], " \t")}
	if len(merged) == 1 && merged[0].text == "" && len(item.children) == 1 && item.children[0].name == "paragraph" {
		// A task holding only its box is an item with nothing in it.
		item.children = nil
		return t[1]
	}
	firstParagraph(item).children = merged
	return t[1]
}

func firstParagraph(item *cnode) *cnode {
	n := item
	for len(n.children) > 0 && n.children[0].name != "text" {
		n = n.children[0]
	}
	return n
}
