package repo

import (
	"iter"
	"os"
	"testing"
)

// StubWriteFile replaces the write that fills every file a document write
// creates, until the returned function restores it.
func StubWriteFile(f func(file *os.File, source []byte) error) (restore func()) {
	old := fill
	fill = f
	return func() { fill = old }
}

// ReadEveryBody reads a body of any length until the test ends, for a test
// timing a reader on a body longer than MaxBody.
func ReadEveryBody(t *testing.T) {
	old := maxBody
	maxBody = 1 << 62
	t.Cleanup(func() { maxBody = old })
}

// ProseOf is the prose of body read as a document's whose first body line is
// bodyLine.
func ProseOf(body string, bodyLine int) iter.Seq[ProseLine] {
	d := &Document{Body: body, BodyLine: bodyLine}
	d.readHeadings()
	return d.Prose()
}
