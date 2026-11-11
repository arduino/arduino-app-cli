// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func newUpdateIndexCmd(cfg config.Configuration, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "update-index",
		Short: "Download the latest bricks index",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			updateIndexHandler(cmd.Context(), cfg, version)
		},
	}
}

// remoteIndexMaxAge is how old the bricks index can be before search and install update it.
const remoteIndexMaxAge = 24 * time.Hour

func updateIndexHandler(ctx context.Context, cfg config.Configuration, version string) {
	failIfRoot("update the bricks index")
	_, err := bricksindex.UpdateRemoteIndex(ctx, bricksindex.DownloadConfig(userAgent(version)), cfg.BricksIndexURL, cfg.BricksIndexDir())
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(updateIndexResult{})
}

// loadRemoteIndex returns the bricks index, updating it first if it is older than remoteIndexMaxAge.
// If the update fails, the saved index is used. As root, the index is never updated.
func loadRemoteIndex(ctx context.Context, cfg config.Configuration, version string) *bricksindex.RemoteIndex {
	dir := cfg.BricksIndexDir()
	if bricksindex.RemoteIndexIsRecent(dir, remoteIndexMaxAge) || os.Geteuid() == 0 {
		if index, err := bricksindex.LoadRemoteIndex(dir); err == nil {
			return index
		}
	}
	failIfRoot("download the bricks index")
	out, _, _ := feedback.OutputStreams()
	fmt.Fprintln(out, "Updating the bricks index...")
	index, err := bricksindex.UpdateRemoteIndex(ctx, bricksindex.DownloadConfig(userAgent(version)), cfg.BricksIndexURL, dir)
	if err == nil {
		return index
	}
	saved, loadErr := bricksindex.LoadRemoteIndex(dir)
	if loadErr != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.Warnf("Cannot update the bricks index, using the saved one: %v", err)
	return saved
}

func userAgent(version string) string {
	return "arduino-app-cli/" + version
}

type updateIndexResult struct{}

func (r updateIndexResult) String() string {
	return "Bricks index updated"
}

func (r updateIndexResult) Data() any {
	return r
}
