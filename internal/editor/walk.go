// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"encoding/base64"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
)

// walkOptions configures a walk. Zero value means "no depth limit, no glob
// filtering, no pagination". Decoupled from the JSON-RPC wire type so the
// walker stays transport-agnostic.
type walkOptions struct {
	// Depth == nil means unlimited. Depth == 0 returns only the root.
	// Depth == N descends up to N levels below the root.
	Depth    *int
	Includes []string
	Excludes []string
	// Cursor is the opaque token returned by a previous call's nextCursor.
	// Empty means "start from the beginning".
	Cursor string
	// Limit caps the number of entries returned. 0 means "no limit".
	Limit int
}

// walkEntry is one entry returned by walk. Path is a POSIX-style absolute
// path (matches the FileEntry.path spec). Depth is relative to the walk
// root (0 = the root itself).
type walkEntry struct {
	Path  string
	Size  int64
	MTime time.Time
	IsDir bool
	Depth int
}

// walk returns entries under root in lexicographic order by absolute path.
// When opts.Limit > 0 it returns at most Limit entries and a nextCursor to
// resume; nextCursor is empty when the walk is exhausted.
func walk(root *paths.Path, opts walkOptions) ([]walkEntry, string, error) {
	rootAbs := filepath.ToSlash(root.String())

	out := []walkEntry{}
	if e, ok := makeWalkEntry(root, rootAbs, 0); ok {
		out = append(out, e)
	}
	if opts.Depth == nil || *opts.Depth > 0 {
		relOf := func(child *paths.Path) string {
			rel, err := child.RelFrom(root)
			if err != nil {
				return child.String()
			}
			return rel.String()
		}

		recurse := func(child *paths.Path) bool {
			if !child.IsDir() {
				return false
			}
			rel := relOf(child)
			if matchExcluded(rel, opts.Excludes) {
				return false
			}
			return opts.Depth == nil || relDepth(rel) < *opts.Depth
		}

		// Directories are pruned by excludes only: an include like "**/*.go"
		// should still surface the dirs on the way to matching files.
		keep := func(child *paths.Path) bool {
			rel := relOf(child)
			if opts.Depth != nil && relDepth(rel) > *opts.Depth {
				return false
			}
			if child.IsDir() {
				return !matchExcluded(rel, opts.Excludes)
			}
			return matchGlobs(rel, opts.Includes, opts.Excludes)
		}

		list, err := root.ReadDirRecursiveFiltered(recurse, keep)
		if err != nil {
			return nil, "", err
		}
		for _, child := range list {
			rel := relOf(child)
			abs := filepath.ToSlash(child.String())
			if e, ok := makeWalkEntry(child, abs, relDepth(rel)); ok {
				out = append(out, e)
			}
		}
	}

	slices.SortFunc(out, func(a, b walkEntry) int { return strings.Compare(a.Path, b.Path) })

	if opts.Cursor != "" {
		after, err := decodeCursor(opts.Cursor)
		if err != nil {
			return nil, "", err
		}
		idx, _ := slices.BinarySearchFunc(out, after, func(e walkEntry, target string) int {
			return strings.Compare(e.Path, target)
		})
		// BinarySearch returns the position of `after` if present, else the
		// insertion point. Either way we want the first entry strictly after.
		for idx < len(out) && out[idx].Path <= after {
			idx++
		}
		out = out[idx:]
	}

	nextCursor := ""
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
		nextCursor = encodeCursor(out[len(out)-1].Path)
	}
	return out, nextCursor, nil
}

// makeWalkEntry stats p and builds a walkEntry with the given absolute POSIX
// path. Directories always report size=0 per the FileEntry spec. Entries
// with a failing stat are skipped (ok == false).
func makeWalkEntry(p *paths.Path, absPath string, depth int) (walkEntry, bool) {
	info, err := p.Stat()
	if err != nil {
		return walkEntry{}, false
	}
	size := info.Size()
	if info.IsDir() {
		size = 0
	}
	return walkEntry{
		Path:  absPath,
		Size:  size,
		MTime: info.ModTime(),
		IsDir: info.IsDir(),
		Depth: depth,
	}, true
}

// relDepth returns the number of path segments in a relative path: 0 for the
// root ("."), 1 for immediate children, and so on.
func relDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// encodeCursor / decodeCursor turn the last emitted absolute path into an
// opaque token. The scheme is intentionally stateless: resuming just walks
// the tree again and skips everything up to and including this path.
func encodeCursor(lastPath string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(lastPath))
}

func decodeCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
