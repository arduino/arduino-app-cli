// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package editor

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"go.bug.st/f"
)

const defaultDebounce = 50 * time.Millisecond

type rawEvent struct {
	Op    fsnotify.Op
	Path  *paths.Path
	IsDir bool
}

type watchSub struct {
	path     *paths.Path
	includes []string
	excludes []string
	log      *slog.Logger

	fsw    *fsnotify.Watcher
	events chan []changeEvent
	errors chan error

	mu      sync.Mutex
	watched map[string]struct{}
}

func newWatchSub(ctx context.Context, target *paths.Path, p watchParams, log *slog.Logger) (*watchSub, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	deb := time.Duration(p.DebounceMs) * time.Millisecond
	if deb <= 0 {
		deb = defaultDebounce
	}
	s := &watchSub{
		path:     target,
		includes: p.Includes, excludes: p.Excludes, log: log,
		fsw:    fw,
		events: make(chan []changeEvent, 16), errors: make(chan error, 4),
		watched: map[string]struct{}{},
	}
	switch {
	case !s.path.IsDir():
		err = s.addDir(s.path.Parent())
	case !p.Recursive:
		err = s.addDir(s.path)
	default:
		err = s.walkAdd(s.path, map[string]struct{}{}, nil)
	}
	if err != nil {
		_ = fw.Close()
		return nil, err
	}
	go s.loop(ctx, p.Recursive, deb)
	return s, nil
}

// walkAdd installs watches under dir (canonical, cycle-safe). When emit is
// non-nil it also appends synthesized Create events for every entry found —
// used to close the recursive-watch race on `mkdir -p ... && touch ...`.
func (s *watchSub) walkAdd(dir *paths.Path, visited map[string]struct{}, emit *[]rawEvent) error {
	canon := dir.Canonical()
	key := canon.String()
	if _, seen := visited[key]; seen {
		return nil
	}
	visited[key] = struct{}{}
	if err := s.addDir(canon); err != nil {
		return err
	}
	entries, err := canon.ReadDir()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if emit != nil {
			*emit = append(*emit, rawEvent{Op: fsnotify.Create, Path: e, IsDir: e.IsDir()})
		}
		if !e.IsDir() {
			continue
		}
		rel, err := e.RelFrom(s.path)
		if err == nil && !matchGlobs(rel.String(), nil, s.excludes) {
			continue
		}
		if err := s.walkAdd(e, visited, emit); err != nil {
			s.log.Debug("editor: walkAdd", slog.String("path", e.String()), slog.String("err", err.Error()))
		}
	}
	return nil
}

func (s *watchSub) addDir(dir *paths.Path) error {
	key := dir.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watched[key]; ok {
		return nil
	}
	if err := s.fsw.Add(key); err != nil {
		return fmt.Errorf("watch %q: %w", key, err)
	}
	s.watched[key] = struct{}{}
	return nil
}

func (s *watchSub) loop(ctx context.Context, recursive bool, debounce time.Duration) {
	defer close(s.events)
	defer close(s.errors)

	var buf []rawEvent
	var timer *time.Timer
	timerC := func() <-chan time.Time {
		if timer == nil {
			return nil
		}
		return timer.C
	}
	relTo := func(p *paths.Path) string {
		r, err := p.RelFrom(s.path)
		if err != nil {
			return p.String()
		}
		return r.String()
	}
	flush := func() {
		if len(buf) == 0 {
			return
		}
		out := coalesce(buf)
		buf = buf[:0]
		out = f.Filter(out, func(e changeEvent) bool {
			return matchGlobs(relTo(e.Path), s.includes, s.excludes)
		})
		if len(out) == 0 {
			return
		}
		select {
		case s.events <- out:
		case <-ctx.Done():
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-s.fsw.Events:
			if !ok {
				flush()
				return
			}
			p := paths.New(ev.Name)
			isDir := p.IsDir()
			buf = append(buf, rawEvent{Op: ev.Op, Path: p, IsDir: isDir})
			if recursive && isDir && ev.Op&fsnotify.Create != 0 {
				if err := s.walkAdd(p, map[string]struct{}{}, &buf); err != nil {
					s.log.Debug("editor: recursive add", slog.String("path", ev.Name), slog.String("err", err.Error()))
				}
			}
			if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				s.mu.Lock()
				if _, ok := s.watched[ev.Name]; ok {
					_ = s.fsw.Remove(ev.Name)
					delete(s.watched, ev.Name)
				}
				s.mu.Unlock()
			}
			if timer == nil {
				timer = time.NewTimer(debounce)
			}
		case err, ok := <-s.fsw.Errors:
			if !ok {
				return
			}
			select {
			case s.errors <- err:
			default:
			}
		case <-timerC():
			timer = nil
			flush()
		}
	}
}

// coalesce collapses a debounce window: CREATE+WRITE→create, multi-WRITE→update,
// CREATE+REMOVE cancels, and a lone remove plus a lone top-level create in one
// parent becomes a rename — whose synthesized descendant creates are then
// dropped, being implied by it.
func coalesce(batch []rawEvent) []changeEvent {
	type state struct {
		created, written, removed, isDir bool
		path                             *paths.Path
	}
	byPath := map[string]*state{}
	var order paths.PathList
	for _, e := range batch {
		key := e.Path.String()
		st, ok := byPath[key]
		if !ok {
			st = &state{isDir: e.IsDir, path: e.Path}
			byPath[key] = st
			order.Add(e.Path)
		}
		if e.IsDir {
			st.isDir = true
		}
		switch {
		case e.Op&fsnotify.Create != 0:
			st.created = true
		case e.Op&fsnotify.Write != 0:
			st.written = true
		case e.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
			st.removed = true
		}
	}

	var created, removed paths.PathList
	for _, p := range order {
		st := byPath[p.String()]
		if st.created && !st.removed {
			created.Add(p)
		}
		if st.removed && !st.created {
			removed.Add(p)
		}
	}
	var topCreated, topRemoved paths.PathList
	for _, p := range created {
		if !created.ContainsEquivalentTo(p.Parent()) {
			topCreated.Add(p)
		}
	}
	for _, p := range removed {
		if !removed.ContainsEquivalentTo(p.Parent()) {
			topRemoved.Add(p)
		}
	}
	var rm, cr *paths.Path
	renamePair := len(topCreated) == 1 && len(topRemoved) == 1 &&
		topRemoved[0].Parent().EquivalentTo(topCreated[0].Parent())
	if renamePair {
		rm, cr = topRemoved[0], topCreated[0]
	}
	// When renaming a directory, drop synthesized descendant creates.
	var suppress paths.PathList
	if renamePair && byPath[cr.String()].isDir {
		for _, p := range created {
			if p.EquivalentTo(cr) {
				continue
			}
			if inside, err := p.IsInsideDir(cr); err == nil && inside {
				suppress.Add(p)
			}
		}
	}

	order.Sort()
	out := make([]changeEvent, 0, len(order))
	for _, p := range order {
		st := byPath[p.String()]
		switch {
		case renamePair && p.EquivalentTo(cr):
			out = append(out, changeEvent{Type: "rename", Path: p, OldPath: rm, IsDir: st.isDir})
		case renamePair && p.EquivalentTo(rm):
		case suppress.ContainsEquivalentTo(p):
		case st.removed && !st.created:
			out = append(out, changeEvent{Type: "delete", Path: p, IsDir: st.isDir})
		case st.created && st.removed:
			// Typically an atomic-replace save (write temp, rename onto
			// target), which emits Rename + Create + Write. Still existing
			// means an update; gone means a true transient.
			if p.Exist() {
				out = append(out, changeEvent{Type: "update", Path: p, IsDir: st.isDir})
			}
		case st.created:
			out = append(out, changeEvent{Type: "create", Path: p, IsDir: st.isDir})
		case st.written:
			out = append(out, changeEvent{Type: "update", Path: p, IsDir: st.isDir})
		}
	}
	return out
}

func matchGlobs(rel string, includes, excludes []string) bool {
	match := func(pat string) bool { ok, _ := doublestar.PathMatch(pat, rel); return ok }
	if slices.ContainsFunc(excludes, match) {
		return false
	}
	return len(includes) == 0 || slices.ContainsFunc(includes, match)
}

func matchExcluded(rel string, excludes []string) bool {
	match := func(pat string) bool { ok, _ := doublestar.PathMatch(pat, rel); return ok }
	return slices.ContainsFunc(excludes, match)
}
