// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !js

package editor

import "github.com/fsnotify/fsnotify"

// notifyWatcher is an fsnotify.Watcher as a watchSub uses it.
type notifyWatcher struct{ w *fsnotify.Watcher }

func newFSWatcher() (fsWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return notifyWatcher{w}, nil
}

func (n notifyWatcher) Add(name string) error         { return n.w.Add(name) }
func (n notifyWatcher) Remove(name string) error      { return n.w.Remove(name) }
func (n notifyWatcher) Close() error                  { return n.w.Close() }
func (n notifyWatcher) Events() <-chan fsnotify.Event { return n.w.Events }
func (n notifyWatcher) Errors() <-chan error          { return n.w.Errors }
