// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
	err := orchestrator.AIModelDelete(ctx, servicelocator.GetDockerClient(), cfg, servicelocator.GetModelsIndex(), servicelocator.GetBricksIndex(), servicelocator.GetPlatform(), id, servicelocator.GetAppIDProvider(), force)
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

func (r deleteModelResult) Data() any {
	return r
}
