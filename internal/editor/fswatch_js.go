// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package editor

import (
	"errors"
	"os"
	"path"
	"sync"
	"syscall"
	"syscall/js"

	"github.com/fsnotify/fsnotify"
)

// jsWatcher watches through the fs object Go's os package runs on under
// js/wasm, with the Node API's fs.watch(dir, listener): the listener gets
// ("rename", name) when an entry of dir appears or goes, ("change", name) when
// one is written. Node's fs has it; a page that gives Go an fs of its own
// implements it from the changes Go makes through that fs.
//
// Node says no more than that, and a stat tells which a rename was, after the
// fact: a file saved by renaming a temporary one onto it can look like a
// rename of that file. An fs that knows passes a third argument, the change as
// fsnotify names it (create, write, remove, rename), and the events are then
// inotify's.
type jsWatcher struct {
	id     int
	events chan fsnotify.Event
	errors chan error
	done   chan struct{}

	mu      sync.Mutex
	watches map[string]js.Value // dir → its FSWatcher
	closed  bool

	// What the listener reported and run has yet to turn into events: the
	// listener runs as a js.Func, which must not block, so it only queues.
	qmu   sync.Mutex
	queue []jsChange
	wake  chan struct{}
}

type jsChange struct {
	path string
	kind string // rename|change|error
	op   string // create|write|remove|rename, where the fs tells
	err  error
}

// One listener serves every fs.watch, bound to its watcher's id and directory,
// and lives as long as the program: an fs may still report a change once its
// watcher has closed, and calling a released js.Func panics.
var (
	listenerOnce sync.Once
	listener     js.Func

	watchersMu sync.Mutex
	watchers   = map[int]*jsWatcher{}
	lastID     int
)

func newFSWatcher() (fsWatcher, error) {
	if fs := js.Global().Get("fs"); fs.Type() != js.TypeObject || fs.Get("watch").Type() != js.TypeFunction {
		return nil, errors.New("editor: the fs Go runs on has no watch")
	}
	listenerOnce.Do(func() { listener = js.FuncOf(onFSChange) })

	w := &jsWatcher{
		events:  make(chan fsnotify.Event),
		errors:  make(chan error),
		done:    make(chan struct{}),
		watches: map[string]js.Value{},
		wake:    make(chan struct{}, 1),
	}
	watchersMu.Lock()
	lastID++
	w.id = lastID
	watchers[w.id] = w
	watchersMu.Unlock()
	go w.run()
	return w, nil
}

func (w *jsWatcher) Add(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fsnotify.ErrClosed
	}
	if _, ok := w.watches[name]; ok {
		return nil
	}
	var fw js.Value
	err := catchJS(func() {
		fw = js.Global().Get("fs").Call("watch", name, listener.Call("bind", nil, w.id, name))
		// An FSWatcher that emits errors throws an unheard one.
		if fw.Get("on").Type() == js.TypeFunction {
			fw.Call("on", "error", listener.Call("bind", nil, w.id, name, "error"))
		}
	})
	if err != nil {
		return &os.PathError{Op: "watch", Path: name, Err: err}
	}
	w.watches[name] = fw
	return nil
}

func (w *jsWatcher) Remove(name string) error {
	w.mu.Lock()
	fw, ok := w.watches[name]
	delete(w.watches, name)
	w.mu.Unlock()
	if !ok {
		return fsnotify.ErrNonExistentWatch
	}
	return catchJS(func() { fw.Call("close") })
}

func (w *jsWatcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	watches := w.watches
	w.watches = nil
	w.mu.Unlock()

	watchersMu.Lock()
	delete(watchers, w.id)
	watchersMu.Unlock()
	for _, fw := range watches {
		_ = catchJS(func() { fw.Call("close") })
	}
	close(w.done)
	return nil
}

func (w *jsWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *jsWatcher) Errors() <-chan error          { return w.errors }

// run turns what the listener queued into fsnotify's events, until Close.
func (w *jsWatcher) run() {
	defer close(w.events)
	defer close(w.errors)
	for {
		select {
		case <-w.done:
			return
		case <-w.wake:
		}
		w.qmu.Lock()
		batch := w.queue
		w.queue = nil
		w.qmu.Unlock()
		for _, c := range batch {
			if c.err != nil {
				select {
				case w.errors <- c.err:
				case <-w.done:
					return
				}
				continue
			}
			ev := fsnotify.Event{Name: c.path}
			switch {
			case c.op == "create":
				ev.Op = fsnotify.Create
			case c.op == "write":
				ev.Op = fsnotify.Write
			case c.op == "remove":
				ev.Op = fsnotify.Remove
			case c.op == "rename":
				ev.Op = fsnotify.Rename
			case c.kind == "rename":
				ev.Op = fsnotify.Remove
				if _, err := os.Lstat(c.path); err == nil {
					ev.Op = fsnotify.Create
				}
			default:
				ev.Op = fsnotify.Write
			}
			select {
			case w.events <- ev:
			case <-w.done:
				return
			}
		}
	}
}

// onFSChange is the listener. Its arguments are the watcher's id and directory,
// bound, then what fs.watch reports: the kind, the entry's name and maybe the
// change, or "error" (bound too) and what the FSWatcher emitted.
func onFSChange(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return nil
	}
	watchersMu.Lock()
	w := watchers[args[0].Int()]
	watchersMu.Unlock()
	if w == nil {
		return nil
	}
	c := jsChange{path: args[1].String(), kind: args[2].String()}
	switch {
	case c.kind == "error":
		detail := js.Undefined()
		if len(args) > 3 {
			detail = args[3]
		}
		c.err = errors.New("editor: watching " + c.path + ": " + jsText(detail))
	case len(args) > 3 && args[3].Type() == js.TypeString:
		c.path = path.Join(c.path, args[3].String())
		if len(args) > 4 && args[4].Type() == js.TypeString {
			c.op = args[4].String()
		}
	}
	w.qmu.Lock()
	w.queue = append(w.queue, c)
	w.qmu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return nil
}

// catchJS runs f, and returns what JS threw in it as an error.
func catchJS(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			thrown, ok := r.(js.Error)
			if !ok {
				panic(r)
			}
			err = jsErrno(thrown.Value)
		}
	}()
	f()
	return nil
}

// jsErrno is the errno a thrown error's code names, as syscall maps the fs
// calls' own, so that errors.Is finds os.ErrNotExist in a missing directory.
func jsErrno(v js.Value) error {
	if v.Type() == js.TypeObject {
		switch v.Get("code").String() {
		case "ENOENT":
			return syscall.ENOENT
		case "ENOTDIR":
			return syscall.ENOTDIR
		case "EACCES":
			return syscall.EACCES
		case "EPERM":
			return syscall.EPERM
		}
	}
	return errors.New(jsText(v))
}

// jsText is what a JS error says.
func jsText(v js.Value) string {
	if v.Type() == js.TypeObject {
		if message := v.Get("message"); message.Type() == js.TypeString {
			return message.String()
		}
	}
	return v.String()
}
