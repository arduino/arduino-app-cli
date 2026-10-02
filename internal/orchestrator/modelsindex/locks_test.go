package modelsindex

import (
	"testing"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocking(t *testing.T) {

	t.Run("Lock a file two times ", func(t *testing.T) {
		lockDir := paths.New(t.TempDir())
		_, err := lockModel(lockDir, "a")
		assert.NoError(t, err)
		_, err = lockModel(lockDir, "a")
		assert.ErrorIs(t, err, ErrInstallInProgress)
	})

	t.Run("Lock a file, then unlock it", func(t *testing.T) {
		lockDir := paths.New(t.TempDir())
		unlock, err := lockModel(lockDir, "a")
		assert.NoError(t, err)
		unlock()
		_, err = lockModel(lockDir, "a")
		assert.NoError(t, err)
	})

	t.Run("Lock a different files", func(t *testing.T) {
		lockDir := paths.New(t.TempDir())
		unlockA, err := lockModel(lockDir, "a")
		require.NoError(t, err)
		defer unlockA()
		_, err = lockModel(lockDir, "b") // must succeed while "a" is held
		assert.NoError(t, err)
	})

	t.Run("Lock nil dir", func(t *testing.T) {
		_, err := lockModel(nil, "a")
		require.NoError(t, err)
		_, err = lockModel(nil, "a")
		assert.NoError(t, err)
	})

}

// TestLoadWiresTheLocksDir: an index built from a real configuration locks for real. The
// tests above set locksDir by hand, which is how a configuration that never reached the
// index once left locking off in the daemon.
func TestLoadWiresTheLocksDir(t *testing.T) {
	data := paths.New(t.TempDir())
	t.Setenv("ARDUINO_APP_CLI__DATA_DIR", data.String())
	cfg, err := config.NewFromEnv()
	require.NoError(t, err)

	idx, err := Load(platform.Platform{BoardName: "ventunoq"}, paths.New("testdata/with-handlers"), paths.New(t.TempDir()), nil, nil, cfg)
	require.NoError(t, err)
	require.NotNil(t, idx.locksDir, "a configuration with a data dir must not leave locking off")
	assert.Equal(t, cfg.ModelLocksDir().String(), idx.locksDir.String())

	unlock, err := idx.LockModel("some-model")
	require.NoError(t, err)
	defer unlock()
	files, err := idx.locksDir.ReadDir()
	require.NoError(t, err)
	assert.Len(t, files, 1, "the lock is a file on disk, shared with other processes")

	_, err = idx.LockModel("some-model")
	require.ErrorIs(t, err, ErrInstallInProgress)
}
