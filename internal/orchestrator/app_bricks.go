// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/cli/cli/command"
	"go.bug.st/downloader/v3"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

// bricksUserAgent identifies the downloads of the bricks a start installs.
// TODO: add the version of the cli, as brick install and update-index do.
const bricksUserAgent = "arduino-app-cli"

// missingBricks returns the third-party bricks of the app that are not installed in the
// version the app pins or, for an unpinned brick, in any version. index is the one of the app.
func missingBricks(a app.ArduinoApp, index *bricksindex.BricksIndex) []app.Brick {
	var missing []app.Brick
	for _, brick := range a.ThirdPartyBricks() {
		if _, found := index.FindBrick(brick.ID, brick.Version); !found {
			missing = append(missing, brick)
		}
	}
	return missing
}

// installBricks installs the given third-party bricks from the bricks index, in the version
// they are pinned to or, if they are not pinned, in the latest version that supports the board.
func installBricks(
	ctx context.Context,
	docker command.Cli,
	cfg config.Configuration,
	servicesIndex *servicesindex.ServicesIndex,
	platform platform.Platform,
	bricks []app.Brick,
	cb func(StreamMessage),
) error {
	downloadConfig := bricksindex.DownloadConfig(bricksUserAgent)
	index, err := loadOrUpdateRemoteIndex(ctx, downloadConfig, cfg, cb)
	if err != nil {
		return err
	}
	for _, brick := range bricks {
		var version *semver.Version
		if brick.Version != "" {
			if version, err = semver.Parse(brick.Version); err != nil {
				return fmt.Errorf("invalid version %q for brick %q: %w", brick.Version, brick.ID, err)
			}
		}
		remoteBrick, err := index.FindBrick(brick.ID)
		var release *bricksindex.RemoteRelease
		if err == nil {
			release, err = remoteBrick.FindRelease(version, platform)
		}
		if errors.Is(err, bricksindex.ErrBrickNotFound) || errors.Is(err, bricksindex.ErrReleaseNotFound) {
			return fmt.Errorf("%w: if it was published recently, run 'arduino-app-cli brick update-index'", err)
		}
		if err != nil {
			return err
		}

		cb(StreamMessage{data: fmt.Sprintf("installing the brick %s@%s", brick.ID, release.Version)})
		if _, err := InstallBrick(ctx, docker, downloadConfig, cfg.BricksDir(), servicesIndex, remoteBrick, release, platform,
			nil, func(line string) { cb(StreamMessage{data: line}) }, nil); err != nil {
			return err
		}
	}
	return nil
}

// loadOrUpdateRemoteIndex returns the bricks index saved on the board. It is downloaded when
// it is missing or broken, and updated when it is older than bricksindex.RemoteIndexMaxAge: if
// that update fails, the saved index is used.
func loadOrUpdateRemoteIndex(ctx context.Context, downloadConfig downloader.Config, cfg config.Configuration, cb func(StreamMessage)) (*bricksindex.RemoteIndex, error) {
	dir := cfg.BricksIndexDir()
	update := func() (*bricksindex.RemoteIndex, error) {
		cb(StreamMessage{data: "updating the bricks index"})
		return bricksindex.UpdateRemoteIndex(ctx, downloadConfig, cfg.BricksIndexURL, dir)
	}

	index, err := bricksindex.LoadRemoteIndex(dir)
	if err != nil {
		return update()
	}
	if !bricksindex.RemoteIndexIsRecent(dir, bricksindex.RemoteIndexMaxAge) {
		updated, err := update()
		if err == nil {
			return updated, nil
		}
		cb(StreamMessage{data: fmt.Sprintf("cannot update the bricks index, using the saved one: %v", err)})
	}
	return index, nil
}
