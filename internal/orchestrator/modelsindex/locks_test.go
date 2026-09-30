package modelsindex

import (
	"testing"

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
