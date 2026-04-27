package board

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsurePlatformInstalled(t *testing.T) {
	// We skip it in CI, as downloading andinstalling the core takes ~6 minutes
	if os.Getenv("CI") != "" {
		t.Skip("Skipping slow test")
	}
	// Example test function
	err := EnsurePlatformInstalled(t.Context(), "arduino:zephyr:unoq")
	require.NoError(t, err)
}
