// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package brick

import (
	"context"
	"fmt"
	"net/http"

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

func updateIndexHandler(ctx context.Context, cfg config.Configuration, version string) {
	res, err := bricksindex.UpdateRemoteIndex(ctx, http.DefaultClient, cfg.BricksIndexURL,
		cfg.BricksIndexDir(), "arduino-app-cli/"+version)
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(updateIndexResult{
		Updated: res.Updated,
		Bricks:  len(res.Index.Bricks),
	})
}

type updateIndexResult struct {
	Updated bool `json:"updated"`
	Bricks  int  `json:"bricks"`
}

func (r updateIndexResult) String() string {
	if r.Updated {
		return fmt.Sprintf("Bricks index updated: %d bricks available", r.Bricks)
	}
	return fmt.Sprintf("Bricks index already up to date: %d bricks available", r.Bricks)
}

func (r updateIndexResult) Data() any {
	return r
}
