package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newCacheCleanCmd(cfg config.Configuration) *cobra.Command {
	var forceClean bool
	appCmd := &cobra.Command{
		Use:   "clean-cache <app-id>",
		Short: "Delete app cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := Load(args[0])
			if err != nil {
				return err
			}
			return cacheCleanHandler(cmd.Context(), app, forceClean)
		},
		ValidArgsFunction: completion.ApplicationNames(cfg),
	}
	appCmd.Flags().BoolVarP(&forceClean, "force", "", false, "Forcefully clean the cache even if the app is running")

	return appCmd
}

func cacheCleanHandler(ctx context.Context, app app.ArduinoApp, forceClean bool) error {
	err := orchestrator.CleanAppCache(
		ctx,
		servicelocator.GetDockerClient(),
		app,
		orchestrator.CleanAppCacheRequest{ForceClean: forceClean},
	)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(cacheCleanResult{
		AppName: app.Name,
		Path:    app.ProvisioningStateDir().String(),
	})
	return nil
}

type cacheCleanResult struct {
	AppName string `json:"appName"`
	Path    string `json:"path"`
}

func (r cacheCleanResult) String() string {
	return fmt.Sprintf("✓ Cache of %q App cleaned", r.AppName)
}

func (r cacheCleanResult) Data() interface{} {
	return r
}
