// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/moby/moby/api/types/container"

	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/launcher"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
)

var (
	ErrAppNotRunning = errors.New("app is not running")
	// ErrRestartNeeded is a reload that was refused with nothing done, because running the
	// Python code again in the same container is not enough: the app has to be stopped
	// and started.
	ErrRestartNeeded = errors.New("app needs a restart")
)

// ReloadApp runs the Python code of a running app again in its container: the app launcher
// replaces the running process with a new one, forked from an interpreter that has the
// libraries imported already. No container is created, started or stopped, and the sketch
// stays as it is.
//
// The reload is done only when the app folder still describes what runs: the project it
// renders now is the one compose brought up, and each of its containers is running. In
// any other case nothing is done and the error is ErrRestartNeeded, or ErrAppNotRunning.
func ReloadApp(
	ctx context.Context,
	docker command.Cli,
	provisioner *Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	appToReload app.ArduinoApp,
	cfg config.Configuration,
	platform platform.Platform,
	cb func(StreamMessage),
) error {
	if cb == nil {
		cb = func(StreamMessage) {}
	}
	refused := func(cause error, reason string) error {
		if reason == "" {
			return fmt.Errorf("cannot reload app %q: %w", appToReload.Name, cause)
		}
		return fmt.Errorf("cannot reload app %q: %w: %s", appToReload.Name, cause, reason)
	}

	if appToReload.IsRelease() {
		return refused(ErrRestartNeeded, "it is installed from a release, which runs without the app launcher")
	}
	if appToReload.MainPythonFile == nil {
		return refused(ErrRestartNeeded, "it has no Python code")
	}

	containers, err := dockerhelper.Containers(ctx, docker.Client(), DockerAppPathLabel+"="+appToReload.FullPath.String())
	if err != nil {
		return err
	}
	if status := parseAppStatus(containers); len(status) == 0 || status[0].Status != StatusRunning {
		return refused(ErrAppNotRunning, "")
	}
	main := launcherContainer(containers)
	if main == nil {
		return refused(ErrRestartNeeded, "its container runs it without the app launcher")
	}

	cb(StreamMessage{data: fmt.Sprintf("Reloading app %q", appToReload.Name)})

	// The project the app folder describes now. Its compose file is not written: a stop
	// reads there the project that was started.
	bricksIndex = appToReload.Bricks(bricksIndex)
	modelsIndex = appToReload.Models(modelsIndex, docker, cfg, platform)
	appEnv := appEnvironment(ctx, appToReload, bricksIndex, modelsIndex, platform)
	if err := provisioner.Resolve(&appToReload, appToReload.ProvisioningStateDir(), bricksIndex, servicesIndex, cfg, appEnv, platform); err != nil {
		return err
	}
	env := hostEnvironment(ctx, appToReload.FullPath, cfg).Merge(appEnv)
	prj, rendered, err := provisioner.Preview(ctx, &appToReload, env, appToReload.Secrets(bricksIndex))
	if err != nil {
		return fmt.Errorf("failed to render the app compose file: %w", err)
	}
	if !pythonFingerprint(appToReload).matches(hashOf(rendered)) {
		return refused(ErrRestartNeeded, "its bricks or its configuration changed since it was started")
	}
	if !projectServicesRunning(prj, containers) {
		return refused(ErrRestartNeeded, "one of its containers is not running")
	}

	started := time.Now()
	client := launcherClient(ctx, docker.Client(), cfg, appToReload.FullPath, main.ID)
	resp, err := client.Start(ctx, nil)
	if reason, restart := launcherRefusal(err); restart {
		return refused(ErrRestartNeeded, reason)
	}
	if err != nil {
		return refused(err, "")
	}
	slog.Info("app reloaded by its launcher",
		slog.String("app", appToReload.FullPath.String()), slog.Int("pid", resp.PID),
		slog.Float64("stop_ms", resp.Timings.StopMs), slog.Float64("fork_ms", resp.Timings.ForkMs),
		slog.Duration("elapsed", time.Since(started)))

	cb(StreamMessage{progress: &Progress{Name: "", Progress: 100.0}})
	return nil
}

// launcherRefusal tells whether a start request failed in a way a restart of the app
// recovers from, with nothing done by the launcher, and why.
func launcherRefusal(err error) (string, bool) {
	switch {
	case err == nil:
		return "", false
	case launcher.IsCode(err, launcher.CodeStale):
		return "its dependencies changed since its container installed them", true
	case launcher.IsCode(err, launcher.CodeBusy):
		return "its launcher is shutting down", true
	case errors.Is(err, syscall.ENOENT), errors.Is(err, syscall.ECONNREFUSED):
		// The container is still installing the dependencies, or its image has no launcher
		return "no app launcher answers in its container", true
	default:
		return "", false
	}
}

// projectServicesRunning tells whether each service of the project has a running container.
func projectServicesRunning(prj *types.Project, containers []container.Summary) bool {
	running := map[string]bool{}
	for _, c := range containers {
		if c.State == container.StateRunning && c.Labels[composeProjectLabel] == prj.Name {
			running[c.Labels[composeServiceLabel]] = true
		}
	}
	for name := range prj.Services {
		if !running[name] {
			return false
		}
	}
	return len(prj.Services) > 0
}
