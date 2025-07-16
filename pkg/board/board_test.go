package board

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsurePlatformInstalled(t *testing.T) {
	// Example test function
	err := EnsurePlatformInstalled(t.Context(), "arduino:zephyr:unoq")
	require.NoError(t, err)
}
