// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package apt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/update"
)

// Service for apt package management operations.
// It manages subscribers and publishes events to all of them.
type Service struct {
	lock sync.Mutex
}

func New() *Service {
	return &Service{}
}

// ListUpgradablePackages lists all upgradable packages using the `apt list --upgradable` command.
// It runs the `apt-get update` command before listing the packages to ensure the package list is up to date.
// It filters the packages using the provided matcher function.
// It returns a slice of UpgradablePackage or an error if the command fails.
func (s *Service) ListUpgradablePackages(ctx context.Context, matcher func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Attempt to fix dpkg database in case an upgrade was interrupted in the middle.
	if err := runDpkgConfigureCommand(ctx); err != nil {
		slog.Warn("error running dpkg configure command, skipped", "error", err)
	}

	err := runUpdateCommand(ctx)
	if err != nil {
		return nil, err
	}

	pkgs, err := listUpgradablePackages(ctx, matcher)
	if err != nil {
		return nil, fmt.Errorf("failed to list upgradable packages: %w", err)
	}
	return pkgs, nil
}

// UpgradePackages upgrades the specified packages using the `apt-get upgrade` command.
// It publishes events to subscribers during the upgrade process.
// It returns an error if the upgrade is already in progress or if the upgrade command fails.
func (s *Service) UpgradePackages(ctx context.Context, packages []update.PackageInfo, eventCB update.EventCallback) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// At the end of the upgrade, always try to restart the services (that need it).
	// This makes sure key services are restarted even if an error happens in the upgrade steps (for examples container images download).
	defer func() {
		eventCB(update.NewDataEvent(update.RestartEvent, "Upgrade completed. Restarting ..."))

		err := restartServices(ctx)
		if err != nil {
			eventCB(update.NewErrorEvent(fmt.Errorf("error restarting services after upgrade: %w", err)))
			return
		}
	}()

	names := f.Map(packages, func(pkg update.PackageInfo) string {
		return pkg.Name
	})
	eventCB(update.NewDataEvent(update.StartEvent, "Upgrade is starting"))
	stream := runUpgradeCommand(ctx, names)
	for line, err := range stream {
		if err != nil {
			return fmt.Errorf("error running upgrade command: %w", err)
		}
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
	}

	eventCB(update.NewDataEvent(update.StartEvent, "apt cleaning cache is starting"))
	for line, err := range runAptCleanCommand(ctx) {
		if err != nil {
			return fmt.Errorf("error running apt clean command: %w", err)
		}
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
	}

	eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Pulling the latest docker images ..."))
	for line, err := range pullDockerImages(ctx) {
		if err != nil {
			// In case of errors, including "out of disk space" erros, do a cleanup and then retry once.

			eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Stop and destroy docker containers and images, to free up space ..."))
			streamCleanup := cleanupDockerContainers(ctx)
			for line, err := range streamCleanup {
				if err != nil {
					slog.Warn("Error during cleanup of container and images", "error", err)
				} else {
					eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
				}
			}

			// Try again to pull the docker containers.
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Pulling the latest docker images (again) ..."))
			for line, err := range pullDockerImages(ctx) {
				if err != nil {
					return fmt.Errorf("error pulling docker images: %w", err)
				}
				eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
			}
		} else {
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
		}
	}

	// After pulling new images is completed, remove old images to free up space.
	eventCB(update.NewDataEvent(update.UpgradeLineEvent, "Cleanup docker containers and images, to remove old unused images"))
	streamCleanup := cleanupDockerContainers(ctx)
	for line, err := range streamCleanup {
		if err != nil {
			slog.Warn("Error during cleanup of container and images", "error", err)
		} else {
			eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
		}
	}

	return nil
}

// runDpkgConfigureCommand is need in case an upgrade was interrupted in the middle
// and the dpkg database is in an inconsistent state.
func runDpkgConfigureCommand(ctx context.Context) error {
	cmd, err := paths.NewProcess(nil, "sudo", "dpkg", "--configure", "-a")
	if err != nil {
		return err
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("error running dpkg configure command: %w: %s", err, out)
	}
	return nil
}

func runUpdateCommand(ctx context.Context) error {
	cmd, err := paths.NewProcess(nil, "sudo", "apt-get", "update")
	if err != nil {
		return err
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("error running apt-get update command: %w: %s", err, out)
	}
	return nil
}

func runUpgradeCommand(ctx context.Context, names []string) iter.Seq2[string, error] {
	env := []string{"NEEDRESTART_MODE=l"}

	aptOptions := []string{
		"-o", "Acquire::Retries=3",
		"-o", "Acquire::http::Timeout=30",
		"-o", "Acquire::https::Timeout=30",
	}
	args := make([]string, 0, 5+len(aptOptions)+len(names))
	args = append(args, "sudo", "apt-get", "install", "--only-upgrade", "-y")
	args = append(args, aptOptions...)
	args = append(args, names...)

	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(env, args...)
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill upgrade command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err := cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}

}

func runAptCleanCommand(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(nil, "sudo", "apt-get", "clean", "-y")
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill apt clean command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err := cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}
}

func pullDockerImages(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(nil, "arduino-app-cli", "system", "init")
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system init' command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err = cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}
}

// Remove all stopped containers
func cleanupDockerContainers(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cmd, err := paths.NewProcess(nil, "arduino-app-cli", "system", "cleanup")
		if err != nil {
			_ = yield("", err)
			return
		}

		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system cleanup' command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err = cmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
			return
		}
	}
}

// RestartServices restarts services that need to be restarted after an upgrade.
// It uses the `needrestart` command to determine which services need to be restarted.
// It returns an error if the command fails to start or if it fails to wait for the command to finish.
// It uses the '-r a' option to restart all services that need to be restarted automatically without prompting the user
// Note: This function does not take the list of services as an argument because
// `needrestart` automatically detects which services need to be restarted based on the system state.
func restartServices(ctx context.Context) error {
	needRestartCmd, err := paths.NewProcess(nil, "sudo", "needrestart", "-r", "a")
	if err != nil {
		return err
	}
	if out, err := needRestartCmd.RunAndCaptureCombinedOutput(ctx); err != nil {
		return fmt.Errorf("error running needrestart command: %w: %s", err, out)
	}
	return nil
}

func listUpgradablePackages(ctx context.Context, matcher func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
	listUpgradable, err := paths.NewProcess(nil, "apt", "list", "--upgradable")
	if err != nil {
		return nil, err
	}

	out, err := listUpgradable.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = listUpgradable.Start()
	if err != nil {
		return nil, err
	}

	packages := parseListUpgradableOutput(out)

	if err := listUpgradable.WaitWithinContext(ctx); err != nil {
		return nil, err
	}

	filtered := f.Filter(packages, matcher)

	return filtered, nil
}

// parseListUpgradableOutput parses the output of `apt list --upgradable` command
// Example: apt/focal-updates 2.0.11 amd64 [upgradable from: 2.0.10]
func parseListUpgradableOutput(r io.Reader) []update.UpgradablePackage {
	re := regexp.MustCompile(`^([^ ]+) ([^ ]+) ([^ ]+)(?: \[upgradable from: ([^\[\]]*)\])?`)

	res := []update.UpgradablePackage{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) == 0 {
			continue
		}

		// Remove repository information in name
		// example: "libgweather-common/zesty-updates,zesty-updates"
		//       -> "libgweather-common"
		name := strings.Split(matches[1], "/")[0]

		pkg := update.UpgradablePackage{
			Type:         update.Debian,
			Name:         name,
			ToVersion:    matches[2],
			Architecture: matches[3],
			FromVersion:  matches[4],
		}
		res = append(res, pkg)
	}
	return res
}
