package modelsindex

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/arduino/go-paths-helper"
	"github.com/gofrs/flock"
)

var ErrInstallInProgress = errors.New("an install or delete of this model is in progress")

func (m *ModelsIndex) LockModel(key string) (unlock func(), err error) {
	return lockModel(m.locksDir, key)
}

func lockModel(dir *paths.Path, key string) (unlock func(), err error) {
	if dir == nil {
		// locking is off only when there is no data dir
		return func() {}, nil
	}

	if err := dir.MkdirAll(); err != nil {
		return func() {}, fmt.Errorf("creating the model locks dir: %w", err)
	}

	sha := sha256.Sum256([]byte(key))
	lockFile := dir.Join(fmt.Sprintf("%x.lock", sha))
	l := flock.New(lockFile.String())
	ok, err := l.TryLock()
	if err != nil {
		return func() {}, fmt.Errorf("locking model %q: %w", key, err)
	}
	if !ok {
		return func() {}, fmt.Errorf("locking model %q: %w", key, ErrInstallInProgress)
	}

	return func() { _ = l.Unlock() }, nil
}

// lockKey is the lock a model already known to the index is installed or deleted under.
func lockKey(m AIModel, board string) string {
	if m.Origin == UserOrigin && m.Deployment != nil {
		if url := m.Deployment.VariablesForPlatform(board)["model_url"]; url != "" {
			return url
		}
	}
	return m.ID
}
