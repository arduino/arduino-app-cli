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

func getLock(flock *flock.Flock, lockFn lockFunc, errorMsg string) (UnlockFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	locked, err := lockFn(ctx, 100*time.Millisecond)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if err := flock.Unlock(); err != nil {
				slog.Error("failed to unlock file lock", "path", flock.Path(), "error", err)
			}
			if err := os.Remove(flock.Path()); err != nil {
				slog.Error("failed to delete lock file", "path", flock.Path(), "error", err)
			}
			locked = false
			slog.Warn("lock file removed due to timeout", "path", flock.Path())
		} else {
			return emptyUnlockFunc, fmt.Errorf("failed trying to acquire %s for %s: %w", errorMsg, flock.Path(), err)
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

func getLockFilePath(path string) string {
	return path + ".lock"
}
