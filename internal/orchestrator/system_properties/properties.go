// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package properties

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/renameio/v2"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	ErrInvalidKey = errors.New("invalid property key")
)

func ReadPropertyKeys(filePath string) ([]string, error) {
	unlock, err := getReadLock(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			slog.Error("failed to release read lock", "file", filePath, "error", unlockErr)
		}
	}()

	propertiesMap, err := readPropertyMap(filePath)
	if err != nil {
		return nil, err
	}
	mapKeys := slices.Collect(maps.Keys(propertiesMap))
	slices.Sort(mapKeys)

	return mapKeys, err
}

// We use renameio to ensure atomic writes. This prevents data corruption (partial writes)
// in case of a system crash or power loss during the save operation.
// NOTE: This mechanism changes the file's Inode on every write, which is why we cannot
// use the data file itself for file locking (flock).
func UpsertProperty(filePath string, key string, value []byte) error {
	if err := validateKey(key); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	unlock, err := getWriteLock(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			slog.Error("failed to release read lock", "file", filePath, "error", unlockErr)
		}
	}()

	propertiesMap, err := readPropertyMap(filePath)
	if err != nil {
		return err
	}
	propertiesMap[key] = value
	newData, err := msgpack.Marshal(propertiesMap)
	if err != nil {
		return err
	}
	return renameio.WriteFile(filePath, newData, 0644)
}

func DeleteProperty(filePath string, key string) (bool, error) {
	if err := validateKey(key); err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	unlock, err := getWriteLock(filePath)
	if err != nil {
		return false, err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			slog.Error("failed to release read lock", "file", filePath, "error", unlockErr)
		}
	}()

	propertiesMap, err := readPropertyMap(filePath)
	if err != nil {
		return false, err
	}
	_, found := propertiesMap[key]
	if !found {
		return false, nil
	}

	delete(propertiesMap, key)

	newData, err := msgpack.Marshal(propertiesMap)
	if err != nil {
		return true, err
	}
	return true, renameio.WriteFile(filePath, newData, 0644)
}

func GetProperty(filePath string, key string) ([]byte, bool, error) {
	if err := validateKey(key); err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	unlock, err := getReadLock(filePath)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			slog.Error("failed to release read lock", "file", filePath, "error", unlockErr)
		}
	}()

	propertiesMap, err := readPropertyMap(filePath)
	if err != nil {
		return nil, false, err
	}
	result, found := propertiesMap[key]
	return result, found, nil
}

func readPropertyMap(filePath string) (map[string][]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]byte), nil
		}
		return nil, err
	}
	if len(content) == 0 {
		return make(map[string][]byte), nil
	}
	var propertiesMap map[string][]byte
	if err := msgpack.Unmarshal(content, &propertiesMap); err != nil {
		return nil, err
	}

	return propertiesMap, nil
}

const maxKeyLength = 100

var keyValidationRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if !keyValidationRegex.MatchString(key) {
		return fmt.Errorf("key '%s' contains invalid characters; only alphanumeric, '-', '_', and '.' are allowed", key)
	}
	if len(key) > maxKeyLength {
		return fmt.Errorf("key exceeds max length of %d characters", maxKeyLength)
	}

	return nil
}

type lockFunc func(context.Context, time.Duration) (bool, error)

type UnlockFunc func() error

func emptyUnlockFunc() error {
	return nil
}

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

// getLockFilePath returns the path to a sidecar lock file (e.g., "data.json.lock").
// We must use a separate file for locking because the main data file is written atomically
// (via renameio), which changes its Inode on every save.
// This sidecar file remains stable (same Inode) and acts as a persistent mutex anchor.
func getLockFilePath(path string) string {
	return path + ".lock"
}
