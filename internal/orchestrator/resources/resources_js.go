// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js

package resources

import (
	"context"
	"iter"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// SystemResources where there is no board, as in a browser: no cpu, memory or disk of a
// board to sample. The stream stays open with nothing to say until ctx ends, as an idle
// board's would between samples, so a reader doesn't take it for a failure.
func SystemResources(ctx context.Context, cfg config.Configuration, resourceCfg *SystemResourceConfig) (iter.Seq[SystemResource], error) {
	return func(yield func(SystemResource) bool) {
		<-ctx.Done()
	}, nil
}
