// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func newModelInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "install model-id",
		Short:             "Install the provided model",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ModelIDs(),
		Run: func(cmd *cobra.Command, args []string) {
			modelInstallHandler(cmd.Context(), args[0])
		},
	}

	return cmd
}

func modelInstallHandler(ctx context.Context, id string) {
	out, _, getResult := feedback.OutputStreams()

	// The handler names each download after its own file, and the file name and URL are
	// not something a user needs to see; --log-level debug still gets them.
	installed, err := orchestrator.AIModelInstall(ctx, servicelocator.GetDockerClient(), servicelocator.GetModelsIndex(),
		servicelocator.GetPlatform(), id, func(m modelsindex.StreamMessage) {
			switch m.GetType() {
			case modelsindex.ProgressType:
				fmt.Fprintf(out, "Downloading model: %.0f%%\n", m.GetProgress().Progress)
			case modelsindex.InfoType:
				slog.Debug("model install", "model", id, "message", m.GetData())
			case modelsindex.ErrorType:
				fmt.Fprintln(out, "[ERROR]", m.GetError())
			case modelsindex.DoneType:
				slog.Debug("model install", "model", id, "message", m.GetDone())
			}
		})
	switch {
	case errors.Is(err, modelsindex.ErrUnknownModel):
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	case errors.Is(err, modelsindex.ErrInsufficientStorage):
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	case errors.Is(err, modelsindex.ErrDownloadReported):
		feedback.Fatal(fmt.Sprintf("model %q could not be installed", id), feedback.ErrGeneric)
	case err != nil:
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	feedback.PrintResult(installModelResult{Model: installed, Output: getResult()})
}

type installModelResult struct {
	Model  modelsindex.AIModel           `json:"model"`
	Output *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r installModelResult) String() string {
	return fmt.Sprintf("✓ Model %q installed successfully.", r.Model.ID)
}

func (r installModelResult) Data() any {
	return r
}
