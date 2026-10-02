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
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

func newModelInstallCmd() *cobra.Command {
	var mmprojURL string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the provided model",
		Long: `Install the provided model.

The argument is a model id from the catalog, a llama.cpp Hugging Face id
("llamacpp:owner/repo[:quant]"), or the URL of a model file the catalog does not
declare.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ModelIDs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if mmprojURL != "" && !isDownloadSource(servicelocator.GetModelsIndex(), args[0]) {
				return errors.New("--mmproj-url only applies when installing from a URL or a llama.cpp id")
			}
			modelInstallHandler(cmd.Context(), args[0], mmprojURL)
			return nil
		},
	}

	cmd.Flags().StringVar(&mmprojURL, "mmproj-url", "", "URL of the companion mmproj file, when installing from a URL or a llama.cpp id")

	return cmd
}

func modelInstallHandler(ctx context.Context, idOrURL, mmprojURL string) {
	out, _, getResult := feedback.OutputStreams()

	// Info and done messages hold the file name and URL; only --log-level debug shows them.
	publish := func(m modelsindex.StreamMessage) {
		switch m.GetType() {
		case modelsindex.ProgressType:
			fmt.Fprintf(out, "Downloading model: %.0f%%\n", m.GetProgress().Progress)
		case modelsindex.InfoType:
			slog.Debug("model install", "model", idOrURL, "message", m.GetData())
		case modelsindex.ErrorType:
			fmt.Fprintln(out, "[ERROR]", m.GetError())
		case modelsindex.DoneType:
			slog.Debug("model install", "model", idOrURL, "message", m.GetDone())
		}
	}

	modelsIndex := servicelocator.GetModelsIndex()
	var installed modelsindex.AIModel
	var err error
	if isDownloadSource(modelsIndex, idOrURL) {
		installed, err = orchestrator.AIModelDownload(ctx, servicelocator.GetDockerClient(), modelsIndex,
			servicelocator.GetPlatform(), idOrURL, mmprojURL, publish)
	} else {
		installed, err = orchestrator.AIModelInstall(ctx, servicelocator.GetDockerClient(), modelsIndex,
			servicelocator.GetPlatform(), idOrURL, publish)
	}
	switch {
	case errors.Is(err, modelsindex.ErrUnknownModel):
		feedback.Fatal(err.Error(), feedback.ErrBadArgument)
	case errors.Is(err, modelsindex.ErrInsufficientStorage):
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	case errors.Is(err, modelsindex.ErrDownloadReported):
		feedback.Fatal(fmt.Sprintf("model %q could not be installed", idOrURL), feedback.ErrGeneric)
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

// isDownloadSource reports whether idOrURL is a URL or a llama.cpp id, not a catalog id.
// The catalog is asked first; a string it does not declare falls back to shape alone.
func isDownloadSource(modelsIndex *modelsindex.ModelsIndex, idOrURL string) bool {
	if modelsIndex.IsKnown(idOrURL) {
		return false
	}
	return strings.HasPrefix(idOrURL, "http://") ||
		strings.HasPrefix(idOrURL, "https://") ||
		strings.HasPrefix(idOrURL, "llamacpp:") ||
		strings.Contains(idOrURL, "/")
}
