// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package properties

import "sync"

// In a page the daemon is the only process at its files, and there is no flock: the lock
// is the process's own.
var fileLock sync.RWMutex

func getWriteLock(filePath string) (UnlockFunc, error) {
	fileLock.Lock()
	return func() error {
		fileLock.Unlock()
		return nil
	}, nil
}

func getReadLock(filePath string) (UnlockFunc, error) {
	fileLock.RLock()
	return func() error {
		fileLock.RUnlock()
		return nil
	}, nil
}
