// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newInstallCmd(cfg config.Configuration, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "install ID[@VERSION]...",
		Short: "Install bricks from the bricks index",
		Long: "Install bricks from the bricks index. Without a version, the latest one that supports the board is installed.\n" +
			"Several versions of the same brick can be installed side by side.",
		Example: "  arduino-app-cli brick install acme:camera\n  arduino-app-cli brick install acme:camera@1.0.0 acme:camera@1.1.0",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			installHandler(cmd.Context(), args, cfg, version)
		},
	}
}

func installHandler(ctx context.Context, args []string, cfg config.Configuration, version string) {
	failIfRoot("install bricks")
	refs := parseBrickReferences(args)
	index := loadRemoteIndex(ctx, cfg, version)

	// Everything is resolved first, so that an unknown brick does not leave the others half done.
	platform := servicelocator.GetPlatform()
	resolved := make([]*bricksindex.RemoteBrick, 0, len(refs))
	for _, ref := range refs {
		brick, err := index.FindRelease(ref.id, ref.version, platform)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		resolved = append(resolved, brick)
	}

	out, _, _ := feedback.OutputStreams()
	var res installResult
	for _, brick := range resolved {
		if brick.Deprecated {
			msg := fmt.Sprintf("Brick %s is deprecated", brick.ID)
			if brick.DeprecationNote != "" {
				msg += ": " + brick.DeprecationNote
			}
			feedback.Warnf("%s", msg)
		}
		progress := downloadProgress(out, brick.ID+"@"+brick.Version.String())
		installed, err := bricksindex.Install(ctx, bricksindex.DownloadConfig(userAgent(version)), cfg.BricksDir(), brick, platform, progress, printWaiting)
		if err != nil {
			if len(res.Bricks) > 0 {
				feedback.PrintResult(res)
			}
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		}
		res.Bricks = append(res.Bricks, installedBrick{
			ID:               brick.ID,
			Version:          brick.Version.String(),
			Path:             installed.Path.String(),
			AlreadyInstalled: installed.AlreadyInstalled,
		})
	}
	feedback.PrintResult(res)
}

// downloadProgress prints the progress of the download of a brick, once per percent.
func downloadProgress(out io.Writer, ref string) func(downloaded, total int64) {
	last := -1
	return func(downloaded, total int64) {
		percent := int(downloaded * 100 / total)
		if percent != last {
			last = percent
			fmt.Fprintf(out, "Downloading %s: %d%%\n", ref, percent)
		}
	}
}

type brickReference struct {
	id      string
	version *semver.Version
}

// parseBrickReferences validates all the references before any of them is processed.
func parseBrickReferences(args []string) []brickReference {
	refs := make([]brickReference, 0, len(args))
	for _, arg := range args {
		id, version, err := bricksindex.ParseBrickReference(arg)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		refs = append(refs, brickReference{id, version})
	}
	return refs
}

type installedBrick struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	Path             string `json:"path"`
	AlreadyInstalled bool   `json:"already_installed"`
}

type installResult struct {
	Bricks []installedBrick `json:"bricks"`
}

func (r installResult) String() string {
	lines := make([]string, 0, len(r.Bricks))
	for _, b := range r.Bricks {
		if b.AlreadyInstalled {
			lines = append(lines, fmt.Sprintf("Brick %s@%s is already installed in %s", b.ID, b.Version, b.Path))
		} else {
			lines = append(lines, fmt.Sprintf("Brick %s@%s installed in %s", b.ID, b.Version, b.Path))
		}
	}
	return strings.Join(lines, "\n")
}

func (r installResult) Data() any {
	return r
}
