// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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

	"github.com/vmihailenco/msgpack/v5"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
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
	return fatomic.WriteFile(filePath, newData, 0644)
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
	return true, fatomic.WriteFile(filePath, newData, 0644)
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

// getLockFilePath returns the path to a sidecar lock file (e.g., "data.json.lock").
// We must use a separate file for locking because the main data file is written atomically
// (via renameio), which changes its Inode on every save.
// This sidecar file remains stable (same Inode) and acts as a persistent mutex anchor.
func getLockFilePath(path string) string {
	return path + ".lock"
}
