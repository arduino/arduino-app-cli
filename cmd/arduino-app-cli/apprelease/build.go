// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apprelease

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/completion"
	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newBuildCmd(cfg config.Configuration, cliVersion string) *cobra.Command {
	var overwrite bool
	var noModels bool
	var keepSecrets bool
	var releaseNumber string
	var filename string

	cmd := &cobra.Command{
		Use:   "build app_path",
		Short: "Build a reproducible Arduino App Release from a pre-built app",
		Long: `Build a reproducible Arduino App Release (.tar.gz) from an app that has already
been built and started at least once.

The app must be pre-built: the command does not compile the sketch nor provision the Python
environment. Start the app once before packaging it.

By default the release is written to the current directory with a generated filename derived
from the app name (shortened to 10 characters) and the release number, e.g.
"my-weather_20260709120000.tar.gz". Use --filename to choose a different name or path; a
".tar.gz" extension is appended automatically if omitted.

Required AI models that are stored on disk (e.g. custom or Edge Impulse models) are bundled
into the release so it is self-contained. Use --no-models to exclude them.

Brick secret variables (API keys, passwords) are by default scrubbed from the release and
replaced with ${NAME} placeholders; the user must provide them on the destination in the
app's data/secrets.env file. Use --keep-secrets to embed the secret values instead (which
makes the archive sensitive).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			appToRelease, err := loadApp(args[0])
			if err != nil {
				feedback.Fatal(err.Error(), feedback.ErrBadArgument)
			}
			return buildHandler(cmd.Context(), cfg, cliVersion, releaseNumber, appToRelease, filename, overwrite, !noModels, keepSecrets)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return completion.ApplicationNamesWithFilterFunc(cfg, func(apps orchestrator.AppInfo) bool {
				return !apps.Example
			})(cmd, args, toComplete)
		},
	}

	cmd.Flags().StringVar(&filename, "filename", "", "Output file name or path (default: <appname>_<release-number>.tar.gz in the current directory); a .tar.gz extension is added if missing")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite the output file if it exists")
	cmd.Flags().BoolVar(&noModels, "no-models", false, "Do not bundle required AI models into the release")
	cmd.Flags().BoolVar(&keepSecrets, "keep-secrets", false, "Embed secret variable values in the release instead of scrubbing them (sensitive)")
	cmd.Flags().StringVar(&releaseNumber, "release-number", "", "Release number to record in the release (default: current timestamp YYYYMMDDhhmmss)")

	return cmd
}

func buildHandler(ctx context.Context, cfg config.Configuration, cliVersion string, releaseNumber string, appToRelease app.ArduinoApp, filename string, overwrite bool, includeModels bool, keepSecrets bool) error {
	// Resolve the release number here so it matches the one recorded in the release and
	// can be embedded into the generated default filename.
	if releaseNumber == "" {
		releaseNumber = time.Now().Format("20060102150405")
	}

	var outputPath *paths.Path
	if filename == "" {
		outputPath = paths.New(releaseDefaultFileName(appToRelease.Name, releaseNumber))
	} else {
		outputPath = paths.New(filename)
		// A trailing separator or an existing directory means "write the default name here".
		dirIntent := outputPath.IsDir() ||
			strings.HasSuffix(filename, "/") || strings.HasSuffix(filename, string(os.PathSeparator))
		if dirIntent {
			outputPath = outputPath.Join(releaseDefaultFileName(appToRelease.Name, releaseNumber))
		} else if !strings.HasSuffix(strings.ToLower(outputPath.String()), ".tar.gz") {
			outputPath = paths.New(outputPath.String() + ".tar.gz")
		}
	}
	// Create the parent directory up front so a non-existent target dir fails here with a clear
	// message instead of deep inside BuildRelease after all the packaging work.
	if parent := outputPath.Parent(); parent != nil {
		if err := parent.MkdirAll(); err != nil {
			feedback.Fatal(fmt.Sprintf("Cannot create output directory %q: %s", parent, err), feedback.ErrGeneric)
		}
	}
	if outputPath.Exist() && !overwrite {
		feedback.Fatal(fmt.Sprintf("File %q already exists. Use --overwrite to overwrite.", outputPath), feedback.ErrGeneric)
	}

	out, _, _ := feedback.OutputStreams()

	err := orchestrator.BuildRelease(
		ctx,
		servicelocator.GetBricksIndex(),
		servicelocator.GetModelsIndex(),
		appToRelease,
		cfg,
		servicelocator.GetPlatform(),
		cliVersion,
		releaseNumber,
		outputPath,
		includeModels,
		keepSecrets,
		func(message orchestrator.StreamMessage) {
			if message.GetType() == orchestrator.InfoType {
				fmt.Fprintln(out, "[INFO]", message.GetData())
			}
		},
	)
	if err != nil {
		feedback.Fatal(fmt.Sprintf("Failed to build release: %s", err), feedback.ErrGeneric)
	}

	feedback.PrintResult(buildReleaseResult{
		Result:  "ok",
		Message: "Release built",
		Path:    outputPath.String(),
	})
	return nil
}

// releaseDefaultFileName builds a meaningful default file name for a release: the app name
// (sanitized to a filesystem-safe slug and shortened to 10 characters), an underscore, and the
// release number. Sanitization keeps only [a-z0-9-] so path separators and other special
// characters can never turn the default into a nested/invalid path, and truncation operates on
// the already-ASCII slug so it can never split a multibyte rune.
func releaseDefaultFileName(appName, releaseNumber string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(appName) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '\\' || r == '.':
			b.WriteByte('-')
		default:
			// drop anything else (punctuation, non-ASCII, control chars)
		}
	}
	name := b.String()
	if len(name) > 10 {
		name = name[:10]
	}
	name = strings.Trim(name, "-")
	if name == "" {
		name = "app"
	}
	return fmt.Sprintf("%s_%s.tar.gz", name, releaseNumber)
}

type buildReleaseResult struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

func (r buildReleaseResult) String() string {
	return fmt.Sprintf("✓ %s at %q", r.Message, r.Path)
}

func (r buildReleaseResult) Data() any {
	return r
}
