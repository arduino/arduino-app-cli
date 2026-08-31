// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/update"
)

// Service for apt package management operations.
// It manages subscribers and publishes events to all of them.
type Service struct {
	lock     sync.Mutex
	selfKill bool
}

func New() *Service {
	return &Service{}
}

// The daemon is restarted after a self-upgrade either way: only it may take the
// shortcut of killing itself, since systemd respawns it. The CLI must not.
func (s *Service) WithSelfKill() *Service {
	s.selfKill = true
	return s
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

const selfPackageName = "arduino-app-cli"

// Progress milestones on a local 0-100 scale: the Manager rescales them to the
// slice of the whole update process this updater is responsible for. Most of the
// scale is reserved to the docker images download, by far the longest step.
const (
	aptUpgradeProgress       float32 = 0.0
	aptCacheCleanProgress    float32 = 25.0
	imagesDownloadProgress   float32 = 30.0
	imagesCleanupProgress    float32 = 90.0
	upgradeCompletedProgress float32 = 100.0
)

// UpgradePackages upgrades the specified packages using the `apt-get upgrade` command.
// It publishes events to subscribers during the upgrade process.
// It returns an error if the upgrade is already in progress or if the upgrade command fails.
func (s *Service) UpgradePackages(ctx context.Context, packages []update.PackageInfo, eventCB update.EventCallback) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	selfUpgrade := slices.ContainsFunc(packages, func(p update.PackageInfo) bool {
		return p.Name == selfPackageName
	})

	defer func() {
		if !selfUpgrade || !s.selfKill {
			return
		}
		eventCB(update.NewDataEvent(update.RestartEvent, fmt.Sprintf("Upgrade completed. Restarting (pid %d) ...", os.Getpid())))
		// needrestart skips its caller's cgroup, so we signal ourselves
		// to let systemd respawn us on the new binary.
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			if err := p.Signal(syscall.SIGTERM); err != nil {
				slog.Error("failed to send SIGTERM to self after upgrade", slog.String("error", err.Error()))
			}
		}
	}()

	names := f.Map(packages, func(pkg update.PackageInfo) string {
		return pkg.Name
	})
	eventCB(update.NewDataEvent(update.StartEvent, "Upgrade is starting"))
	eventCB(update.NewProgressEvent("apt upgrade", aptUpgradeProgress))
	stream := runUpgradeCommand(ctx, names)
	for line, err := range stream {
		if err != nil {
			return fmt.Errorf("error running upgrade command: %w", err)
		}
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
	}

	eventCB(update.NewDataEvent(update.StartEvent, "apt cleaning cache is starting"))
	eventCB(update.NewProgressEvent("apt cache cleanup", aptCacheCleanProgress))
	for line, err := range runAptCleanCommand(ctx) {
		if err != nil {
			return fmt.Errorf("error running apt clean command: %w", err)
		}
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
	}
	eventCB(update.NewProgressEvent("docker images download", imagesDownloadProgress))

	// Of the two sources reporting a progress, only docker can be trusted: it is
	// the fraction of the bytes of every image, and the total is known before the
	// download starts. The arduino one is per file, out of an unknown number of
	// them, so it is only logged.
	lastImagesProgress := imagesDownloadProgress
	handleInitResult := func(result orchestrator.InitResult) {
		if result.Type == orchestrator.InitResultProgress && result.Source == orchestrator.InitSourceDocker {
			if value := imagesDownloadBand(result.Percent); value > lastImagesProgress {
				lastImagesProgress = value
				eventCB(update.NewProgressEvent("docker images download", value))
			}
		}
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, result.String()))
	}

	for result, err := range runSystemInit(ctx) {
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
			for result, err := range runSystemInit(ctx) {
				if err != nil {
					return fmt.Errorf("error pulling docker images: %w", err)
				}
				handleInitResult(result)
			}
		} else {
			handleInitResult(result)
		}
	}
	eventCB(update.NewProgressEvent("docker images cleanup", imagesCleanupProgress))
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
	// The 100% milestone is emitted here, and not left to the Manager, because the
	// deferred restart of the services runs before this function returns: needrestart
	// restarts the daemon itself, so anything broadcast after it would never reach
	// the subscribers.
	eventCB(update.NewProgressEvent("upgrade completed", upgradeCompletedProgress))
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
	env := []string{"NEEDRESTART_MODE=a"}

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

func runSystemInit(ctx context.Context) iter.Seq2[orchestrator.InitResult, error] {
	return func(yield func(orchestrator.InitResult, error) bool) {
		cmd, err := paths.NewProcess(nil, "arduino-app-cli", "system", "init", "--format", "json-lines")
		if err != nil {
			_ = yield(orchestrator.InitResult{}, err)
			return
		}

		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(parseSystemInitLine(line), nil) {
				if err := cmd.Kill(); err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system init' command", slog.String("error", err.Error()))
				}
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		if err = cmd.RunWithinContext(ctx); err != nil {
			_ = yield(orchestrator.InitResult{}, err)
			return
		}
	}
}

func parseSystemInitLine(line string) orchestrator.InitResult {
	var result orchestrator.InitResult
	if err := json.Unmarshal([]byte(line), &result); err != nil || result.Type == "" {
		return orchestrator.InitResult{Type: orchestrator.InitResultLog, Message: line}
	}
	return result
}

// imagesDownloadBand maps the 0-100 percentage reported by `system init` onto the
// band of the local scale reserved to the images download. The percentage comes
// from another process, so it is clamped rather than trusted.
func imagesDownloadBand(percentage int) float32 {
	percentage = min(max(percentage, 0), 100)
	return imagesDownloadProgress + float32(percentage)/100.0*(imagesCleanupProgress-imagesDownloadProgress)
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
