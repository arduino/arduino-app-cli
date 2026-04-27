//go:build !windows

package fatomic

import (
	"os"

	"github.com/google/renameio/v2"
)

func WriteFile(filename string, data []byte, perm os.FileMode, opts ...renameio.Option) error {
	return renameio.WriteFile(filename, data, perm)
}
