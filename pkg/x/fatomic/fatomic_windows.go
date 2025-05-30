package fatomic

import (
	"os"
)

// WriteFile this is used just to not break go build on Windows. We do not support
// atomic rename on Windows. In the scope of this project that aims to run only
// on Linux this function is only used to allow dev that runs on windows to test
// other part of the program.
func WriteFile(filename string, data []byte, perm os.FileMode, opts ...any) error {
	f, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return nil
}
