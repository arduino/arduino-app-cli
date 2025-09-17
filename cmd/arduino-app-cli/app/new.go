package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newCreateCmd(cfg config.Configuration) *cobra.Command {
	var (
		icon        string
		description string
		bricks      []string
		noPyton     bool
		noSketch    bool
		fromApp     string
	)

	cmd := &cobra.Command{
		Use:   "new name",
		Short: "Creates a new app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cobra.MinimumNArgs(1)
			name := args[0]
			return createHandler(cmd.Context(), cfg, name, icon, description, noPyton, noSketch, fromApp)
		},
	}

	cmd.Flags().StringVarP(&icon, "icon", "i", "", "Icon for the app")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description for the app")
	cmd.Flags().StringVarP(&fromApp, "from-app", "", "", "Create the new app from the path of an existing app")
	cmd.Flags().StringArrayVarP(&bricks, "bricks", "b", []string{}, "List of bricks to include in the app")
	cmd.Flags().BoolVarP(&noPyton, "no-python", "", false, "Do not include Python files")
	cmd.Flags().BoolVarP(&noSketch, "no-sketch", "", false, "Do not include Sketch files")
	cmd.MarkFlagsMutuallyExclusive("no-python", "no-sketch")

	return cmd
}

func createHandler(ctx context.Context, cfg config.Configuration, name string, icon string, description string, noPython, noSketch bool, fromApp string) error {
	if fromApp != "" {
		id, err := servicelocator.GetAppIDProvider().ParseID(fromApp)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			return nil
		}

		resp, err := orchestrator.CloneApp(ctx, orchestrator.CloneAppRequest{
			Name:   &name,
			FromID: id,
		}, servicelocator.GetAppIDProvider(), cfg)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		dst := resp.ID.ToPath()

		feedback.PrintResult(createAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    dst.String(),
		})

	} else {
		resp, err := orchestrator.CreateApp(ctx, orchestrator.CreateAppRequest{
			Name:        name,
			Icon:        icon,
			Description: description,
			SkipPython:  noPython,
			SkipSketch:  noSketch,
		}, servicelocator.GetAppIDProvider(), cfg)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
			return nil
		}
		feedback.PrintResult(createAppResult{
			Result:  "ok",
			Message: "App created successfully",
			Path:    resp.ID.ToPath().String(),
		})
	}
	return nil
}

type createAppResult struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

func (r createAppResult) String() string {
	return fmt.Sprintf("%s: %s (%s)", r.Message, r.Path, r.Result)
}

func (r createAppResult) Data() interface{} {
	return r
}
