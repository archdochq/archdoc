package repo

import (
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
