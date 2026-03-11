// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package model

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newModelDeleteCmd(cfg config.Configuration) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the provided custom model",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			modelDeleteHandler(cmd.Context(), cfg, args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Delete model in use.")

	return cmd
}

func modelDeleteHandler(ctx context.Context, cfg config.Configuration, id string, force bool) {
	err := orchestrator.AIModelDelete(ctx, servicelocator.GetDockerClient(), cfg, servicelocator.GetModelsIndex(), servicelocator.GetPlatform(), id, servicelocator.GetAppIDProvider(), force)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(deleteModelResult{
		ModelID: id,
	})
}

type deleteModelResult struct {
	ModelID string `json:"model_id"`
}

func (r deleteModelResult) String() string {
	return fmt.Sprintf("✓ Model '%q deleted successfully.", r.ModelID)
}

func (r deleteModelResult) Data() interface{} {
	return r
}
