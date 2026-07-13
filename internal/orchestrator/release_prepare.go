// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"fmt"
	"slices"

	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

// PrepareRelease pre-pulls the Docker images referenced by an installed app's localized
// compose file (`.cache/app-compose.yaml`) so that a subsequent `app start` finds them
// locally and does not need to reach the network.
//
// It deliberately does NOT start the app, and it never downloads AI models — model
// artifacts are already on disk (restored at install time); only containers are ever
// pulled. Images that are already present locally are skipped, mirroring the
// `docker compose up --pull missing` behavior used at start.
func PrepareRelease(ctx context.Context, arduinoApp app.ArduinoApp, dockerClient command.Cli, cb func(StreamMessage)) error {
	emit := func(msg string) {
		if cb != nil {
			cb(StreamMessage{data: msg})
		}
	}

	composePath := arduinoApp.AppComposeFilePath()
	if composePath == nil || !composePath.Exist() {
		return fmt.Errorf("%w: app has no prepared compose file (%s); install the release first", ErrBadRequest, composePath)
	}
	composeBytes, err := composePath.ReadFile()
	if err != nil {
		return fmt.Errorf("failed to read compose file %s: %w", composePath, err)
	}

	images := extractComposeImages(composeBytes)
	if len(images) == 0 {
		emit("No container images referenced by the app; nothing to prepare")
		return nil
	}

	alreadyPulled, err := listImagesAlreadyPulled(ctx, dockerClient.Client())
	if err != nil {
		return fmt.Errorf("failed to list local docker images: %w", err)
	}

	toPull := imagesToPull(images, alreadyPulled)
	emit(fmt.Sprintf("%d image(s) referenced, %d already present, %d to pull",
		len(images), len(images)-len(toPull), len(toPull)))

	writer := NewCallbackWriter(func(line string) { emit(line) })
	for _, image := range toPull {
		emit(fmt.Sprintf("Pulling %s", image))
		if err := pullImage(ctx, writer, dockerClient.Client(), image); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}

	emit("All required container images are present locally")
	return nil
}

// imagesToPull returns, in input order, the subset of images that are not already present
// locally. Order is preserved and inputs are assumed already de-duplicated (as produced by
// extractComposeImages).
func imagesToPull(images []string, alreadyPulled []string) []string {
	var toPull []string
	for _, image := range images {
		if slices.Contains(alreadyPulled, image) {
			continue
		}
		toPull = append(toPull, image)
	}
	return toPull
}
