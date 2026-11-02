package repo

import "os"

// StubWriteFile replaces the write every document write makes, until the
// returned function restores it.
func StubWriteFile(f func(name string, data []byte, perm os.FileMode) error) (restore func()) {
	old := writeFile
	writeFile = f
	return func() { writeFile = old }
}
