package repo

import "os"

// StubWriteFile replaces the write that fills every file a document write
// creates, until the returned function restores it.
func StubWriteFile(f func(file *os.File, source []byte) error) (restore func()) {
	old := fill
	fill = f
	return func() { fill = old }
}
