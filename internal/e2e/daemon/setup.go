package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func GetHttpclient(t *testing.T) *client.ClientWithResponses {
	t.Helper()
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)

	return httpClient
}
