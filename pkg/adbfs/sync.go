package adbfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

type FSWriter interface {
	fs.FS

	MkDirAll(path string) error
	WriteFile(path string, data io.ReadCloser) error
	RmFile(path string) error
}

// SyncFS synchronizes the contents of a source file system (srcFS) with a destination file system (dstFS).
// It also removes files from the destination that are not present in the source.
// TODO: be smarter and only copy files that are different.
func SyncFS(dstFS FSWriter, srcFS fs.FS) error {
	err := fs.WalkDir(srcFS, ".", func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return dstFS.MkDirAll(src)
		}

		if !d.Type().IsRegular() {
			fmt.Printf("WARNING: skipping file %q of type %s\n", src, d.Type())
			return nil
		}

		f, err := srcFS.Open(src)
		if err != nil {
			return fmt.Errorf("error opening source file %q: %w", src, err)
		}
		defer f.Close()
		return dstFS.WriteFile(src, f)
	})
	if err != nil {
		return fmt.Errorf("error walking source fs: %w", err)
	}

	return fs.WalkDir(dstFS, ".", func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		f, err := srcFS.Open(src)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return dstFS.RmFile(src)
			}
			return fmt.Errorf("error opening source file %q: %w", src, err)
		}
		return f.Close()
	})
}
