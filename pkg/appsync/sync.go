package appsync

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"slices"
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
func SyncFS(dstFS FSWriter, srcFS fs.FS, ignorePath ...string) error {
	shoudlIgnore := func(src string) bool {
		if idx := slices.IndexFunc(ignorePath, func(ignore string) bool {
			return path.Base(ignore) == path.Base(src)
		}); idx != -1 {
			return true
		}
		return false
	}
	err := fs.WalkDir(srcFS, ".", func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// ignore paths
		if shoudlIgnore(src) {
			return fs.SkipDir
		}

		if d.IsDir() {
			return dstFS.MkDirAll(src)
		}

		if !d.Type().IsRegular() {
			slog.Warn("sync skipping file", "file", src, "type", d.Type())
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

		if shoudlIgnore(src) {
			return fs.SkipDir
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
