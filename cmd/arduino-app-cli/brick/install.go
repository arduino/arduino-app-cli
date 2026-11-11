// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newInstallCmd(cfg config.Configuration, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "install ID[@VERSION]...",
		Short: "Install bricks from the bricks index",
		Long: "Install bricks from the bricks index. Without a version, the latest one is installed.\n" +
			"Several versions of the same brick can be installed side by side.",
		Example: "  arduino-app-cli brick install acme:camera\n  arduino-app-cli brick install acme:camera@1.0.0 acme:camera@1.1.0",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			installHandler(cmd.Context(), args, cfg, version)
		},
	}
}

func installHandler(ctx context.Context, args []string, cfg config.Configuration, version string) {
	refs := parseBrickReferences(args)
	index, err := bricksindex.LoadRemoteIndex(cfg.BricksIndexDir())
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}

	// Everything is resolved first, so that an unknown brick does not leave the others half done.
	type toInstall struct {
		brick   *bricksindex.RemoteBrick
		release *bricksindex.RemoteRelease
	}
	resolved := make([]toInstall, 0, len(refs))
	for _, ref := range refs {
		brick, release, err := index.FindRelease(ref.id, ref.version)
		if err != nil {
			feedback.Fatal(err.Error(), feedback.ErrBadArgument)
		}
		resolved = append(resolved, toInstall{brick, release})
	}

	var res installResult
	for _, r := range resolved {
		if r.brick.Deprecated {
			msg := fmt.Sprintf("Brick %s is deprecated", r.brick.ID)
			if r.brick.DeprecationNote != "" {
				msg += ": " + r.brick.DeprecationNote
			}
			feedback.Warnf("%s", msg)
		}
		installed, err := bricksindex.Install(ctx, http.DefaultClient, cfg.BricksDir(),
			r.brick, r.release, "arduino-app-cli/"+version)
		if err != nil {
			if len(res.Bricks) > 0 {
				feedback.PrintResult(res)
			}
			feedback.Fatal(err.Error(), feedback.ErrGeneric)
		}
		res.Bricks = append(res.Bricks, installedBrick{
			ID:               r.brick.ID,
			Version:          r.release.Version,
			Path:             installed.Path.String(),
			AlreadyInstalled: installed.AlreadyInstalled,
		})
	}
	feedback.PrintResult(res)
}

type brickReference struct {
	id, version string
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
