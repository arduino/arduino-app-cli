// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !js

package properties

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gofrs/flock"
)

// getLock attempts to acquire a file lock.
//
// STRATEGY: "Force on Timeout"
// If we cannot acquire the lock within the timeout (3 seconds), we assume the
// lock file is stale (orphaned by a crashed process). In this scenario, we
// force a recovery by deleting the lock file and attempting to acquire a new one.
func getLock(flock *flock.Flock, lockFn lockFunc, errorMsg string) (UnlockFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	locked, err := lockFn(ctx, 100*time.Millisecond)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			return emptyUnlockFunc, fmt.Errorf("failed trying to acquire %s for %s: %w", errorMsg, flock.Path(), err)
		}
		slog.Warn("lock acquisition timed out; assuming stale lock and forcing reset", "path", flock.Path())
		if removeErr := os.Remove(flock.Path()); removeErr != nil && !os.IsNotExist(removeErr) {
			slog.Error("failed to remove stale lock file", "path", flock.Path(), "error", removeErr)
		}
		forceCtx, forceCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer forceCancel()

		locked, err = lockFn(forceCtx, 100*time.Millisecond)
		if err != nil {
			return emptyUnlockFunc, fmt.Errorf("failed to force acquire %s after removing stale file: %w", errorMsg, err)
		}
	}

	if !locked {
		return emptyUnlockFunc, fmt.Errorf("unable to acquire %s for %s", errorMsg, flock.Path())
	}

	return func() error {
		if err := flock.Unlock(); err != nil {
			return fmt.Errorf("failed to unlock file lock for %s: %w", flock.Path(), err)
		}
		return nil
	}, nil
}

func getWriteLock(filePath string) (UnlockFunc, error) {
	fileLock := flock.New(getLockFilePath(filePath))
	return getLock(fileLock, fileLock.TryLockContext, "write lock")
}

func getReadLock(filePath string) (UnlockFunc, error) {
	fileLock := flock.New(getLockFilePath(filePath))
	return getLock(fileLock, fileLock.TryRLockContext, "read lock")
}
