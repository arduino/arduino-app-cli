package app

import (
	"context"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/results"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func newCreateCmd() *cobra.Command {
	var (
		icon     string
		bricks   []string
		noPyton  bool
		noSketch bool
		fromApp  string
	)

	cmd := &cobra.Command{
		Use:   "new name",
		Short: "Creates a new app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cobra.MinimumNArgs(1)
			name := args[0]
			return createHandler(cmd.Context(), name, icon, noPyton, noSketch, fromApp)
		},
	}

	cmd.Flags().StringVarP(&icon, "icon", "i", "", "Icon for the app")
	cmd.Flags().StringVarP(&fromApp, "from-app", "", "", "Create the new app from the path of an existing app")
	cmd.Flags().StringArrayVarP(&bricks, "bricks", "b", []string{}, "List of bricks to include in the app")
	cmd.Flags().BoolVarP(&noPyton, "no-python", "", false, "Do not include Python files")
	cmd.Flags().BoolVarP(&noSketch, "no-sketch", "", false, "Do not include Sketch files")
	cmd.MarkFlagsMutuallyExclusive("no-python", "no-sketch")

	return cmd
}

func createHandler(ctx context.Context, name string, icon string, noPython, noSketch bool, fromApp string) error {
	if fromApp != "" {
		wd, err := paths.Getwd()
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		fromPath := paths.New(fromApp)
		if !fromPath.IsAbs() {
			fromPath = wd.JoinPath(fromPath)
		}
		id, err := orchestrator.NewIDFromPath(fromPath)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			return nil
		}

		resp, err := orchestrator.CloneApp(ctx, orchestrator.CloneAppRequest{
			Name:   &name,
			FromID: id,
		})
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		dst := resp.ID.ToPath()

		feedback.PrintResult(results.CreateAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    dst.String(),
		})

	} else {
		resp, err := orchestrator.CreateApp(ctx, orchestrator.CreateAppRequest{
			Name:       name,
			Icon:       icon,
			SkipPython: noPython,
			SkipSketch: noSketch,
		})
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		feedback.PrintResult(results.CreateAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    resp.ID.ToPath().String(),
		})
	}
	return nil
}
