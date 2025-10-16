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
	"time"

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
	if !s.lock.TryLock() {
		return nil, update.ErrOperationAlreadyInProgress
	}
	defer s.lock.Unlock()

	if err := runDpkgConfigureCommand(ctx); err != nil {
		return nil, fmt.Errorf("error running dpkg configure command: %w", err)
	}

	err := runUpdateCommand(ctx)
	if err != nil {
		return nil, fmt.Errorf("error running apt-get update command: %w", err)
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
func (s *Service) UpgradePackages(ctx context.Context, names []string) (<-chan update.Event, error) {
	if !s.lock.TryLock() {
		return nil, update.ErrOperationAlreadyInProgress
	}
	eventsCh := make(chan update.Event, 100)

	go func() {
		defer s.lock.Unlock()
		defer close(eventsCh)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		eventsCh <- update.Event{Type: update.StartEvent, Data: "Upgrade is starting"}
		stream := runUpgradeCommand(ctx, names)
		for line, err := range stream {
			if err != nil {
				eventsCh <- update.Event{
					Type: update.ErrorEvent,
					Err:  err,
					Data: "Error running upgrade command",
				}
				slog.Error("error processing upgrade command output", "error", err)
				return
			}
			eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: line}
		}
		eventsCh <- update.Event{Type: update.StartEvent, Data: "apt cleaning cache is starting"}
		for line, err := range runAptCleanCommand(ctx) {
			if err != nil {
				eventsCh <- update.Event{
					Type: update.ErrorEvent,
					Err:  err,
					Data: "Error running apt clean command",
				}
				slog.Error("error processing apt clean command output", "error", err)
				return
			}
			eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: line}
		}
		// TEMPORARY PATCH: stopping and destroying docker containers and images since IDE does not implement it yet.
		// TODO: Remove this workaround once IDE implements it.
		// Tracking issue: https://github.com/arduino/arduino-app-cli/issues/623
		eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: "Stop and destroy docker containers and images ..."}
		streamCleanup := cleanupDockerContainers(ctx)
		for line, err := range streamCleanup {
			if err != nil {
				// TODO: maybe we should retun an error or a better feedback to the user?
				// currently, we just log the error and continue considenring not blocking
				slog.Error("Error stopping and destroying docker containers", "error", err)
			}
			eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: line}
		}

		// TEMPORARY PATCH: Install the latest docker images and show the logs to the users.
		// TODO: Remove this workaround once docker image versions are no longer hardcoded in arduino-app-cli.
		// Tracking issue: https://github.com/arduino/arduino-app-cli/issues/600
		// Currently, we need to launch `arduino-app-cli system init` to pull the latest docker images because
		// the version of the docker images are hardcoded in the (new downloaded) version of the arduino-app-cli.
		eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: "Pulling the latest docker images ..."}
		streamDocker := pullDockerImages(ctx)
		for line, err := range streamDocker {
			if err != nil {
				eventsCh <- update.Event{
					Type: update.ErrorEvent,
					Err:  err,
					Data: "Error upgrading docker images",
				}
				slog.Error("error upgrading docker images", "error", err)
				return
			}
			eventsCh <- update.Event{Type: update.UpgradeLineEvent, Data: line}
		}
		eventsCh <- update.Event{Type: update.RestartEvent, Data: "Upgrade completed. Restarting ..."}

		err := restartServices(ctx)
		if err != nil {
			eventsCh <- update.Event{
				Type: update.ErrorEvent,
				Err:  err,
				Data: "Error restart services after upgrade",
			}
			slog.Error("failed to restart services", "error", err)
			return
		}
	}()

	return eventsCh, nil
}

// runDpkgConfigureCommand is need in case an upgrade was interrupted in the middle
// and the dpkg database is in an inconsistent state.
func runDpkgConfigureCommand(ctx context.Context) error {
	dpkgCmd, err := paths.NewProcess(nil, "sudo", "dpkg", "--configure", "-a")
	if err != nil {
		return err
	}
	err = dpkgCmd.RunWithinContext(ctx)
	if err != nil {
		return fmt.Errorf("error running dpkg configure command: %w", err)
	}
	return nil
}

func runUpdateCommand(ctx context.Context) error {
	updateCmd, err := paths.NewProcess(nil, "sudo", "apt-get", "update")
	if err != nil {
		return err
	}
	err = updateCmd.RunWithinContext(ctx)
	if err != nil {
		return err
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
	args := []string{"sudo", "apt-get", "install", "--only-upgrade", "-y"}
	args = append(args, aptOptions...)
	args = append(args, names...)

	return func(yield func(string, error) bool) {
		upgradeCmd, err := paths.NewProcess(env, args...)
		if err != nil {
			_ = yield("", err)
			return
		}
		stdout := orchestrator.NewCallbackWriter(func(line string) {
			if !yield(line, nil) {
				err := upgradeCmd.Kill()
				if err != nil {
					slog.Error("Failed to kill upgrade command", slog.String("error", err.Error()))
				}
				return
			}
		})
		upgradeCmd.RedirectStderrTo(stdout)
		upgradeCmd.RedirectStdoutTo(stdout)
		if err := upgradeCmd.RunWithinContext(ctx); err != nil {
			return
		}
	}

}

func runAptCleanCommand(ctx context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		cleanCmd, err := paths.NewProcess(nil, "sudo", "apt-get", "clean", "-y")
		if err != nil {
			_ = yield("", err)
			return
		}
		stdout := orchestrator.NewCallbackWriter(func(line string) {
			_ = yield(line, nil)
		})
		cleanCmd.RedirectStderrTo(stdout)
		cleanCmd.RedirectStdoutTo(stdout)
		if err := cleanCmd.RunWithinContext(ctx); err != nil {
			_ = yield("", err)
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
				err := cmd.Kill()
				if err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system init' command", slog.String("error", err.Error()))
				}
				return
			}
		})
		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)
		err = cmd.RunWithinContext(ctx)
		if err != nil {
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
				err := cmd.Kill()
				if err != nil {
					slog.Error("Failed to kill 'arduino-app-cli system cleanup' command", slog.String("error", err.Error()))
				}
				return
			}
		})

		cmd.RedirectStderrTo(stdout)
		cmd.RedirectStdoutTo(stdout)

		err = cmd.RunWithinContext(ctx)
		if err != nil {
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
	err = needRestartCmd.RunWithinContext(ctx)
	if err != nil {
		return err
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
