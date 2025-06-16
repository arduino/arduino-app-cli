package daemon

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.bug.st/f"
	"go.bug.st/testifyjson/requirejson"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestCreateApp(t *testing.T) {
	cli := e2e.CreateEnvForDaemon(t)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClient(cli.DaemonAddr)
	require.NoError(t, err)

	r, err := httpClient.CreateApp(t.Context(), client.CreateAppRequest{
		Icon: f.Ptr("🌎"),
		Name: "HelloWorld",
	})

	require.NoError(t, err)
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	defer r.Body.Close()

	require.Equal(t, http.StatusCreated, r.StatusCode)
	requirejson.Parse(t, body).Query(".id").MustNotBeEmpty("")
}
