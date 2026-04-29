// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package arduino

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/update"
)

type ArduinoPlatformUpdater struct {
	lock       sync.Mutex
	constraint semver.Constraint
	platform   platform.Platform
}

func NewArduinoPlatformUpdater(platform platform.Platform, versionConstraint semver.Constraint) *ArduinoPlatformUpdater {
	return &ArduinoPlatformUpdater{
		constraint: versionConstraint,
		platform:   platform,
	}
}

// ListUpgradablePackages implements ServiceUpdater.
func (a *ArduinoPlatformUpdater) ListUpgradablePackages(ctx context.Context, _ func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := orchestrator.SetArduinoCliConfig(ctx, srv); err != nil {
		return nil, err
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return nil, err
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	stream, _ := commands.UpdateIndexStreamResponseToCallbackFunction(ctx, func(curr *rpc.DownloadProgress) {
		slog.Debug("Update index progress", slog.String("download_progress", curr.String()))
	})
	if err := srv.UpdateIndex(&rpc.UpdateIndexRequest{Instance: inst}, stream); err != nil {
		return nil, err
	}

	streamLibIndex, _ := commands.UpdateLibrariesIndexStreamResponseToCallbackFunction(ctx, func(curr *rpc.DownloadProgress) {
		slog.Debug("downloading library index", "progress", curr.GetMessage())
	})

	req := &rpc.UpdateLibrariesIndexRequest{Instance: inst}
	if err := srv.UpdateLibrariesIndex(req, streamLibIndex); err != nil {
		slog.Warn("error updating library index, skipping", slog.String("error", err.Error()))
	}

	if err := srv.Init(
		&rpc.InitRequest{Instance: inst},
		commands.InitStreamResponseToCallbackFunction(ctx, func(r *rpc.InitResponse) error {
			slog.Debug("Arduino init instance", slog.String("instance", r.String()))
			return nil
		}),
	); err != nil {
		return nil, err
	}

	platforms, err := srv.PlatformSearch(ctx, &rpc.PlatformSearchRequest{
		Instance:          inst,
		ManuallyInstalled: true,
	})
	if err != nil {
		return nil, err
	}

	var platformSummary *rpc.PlatformSummary
	for _, v := range platforms.GetSearchOutput() {
		if v.GetMetadata().GetId() == a.platform.PlatformID {
			platformSummary = v
			break
		}
	}

	if platformSummary == nil {
		return nil, nil // No platform found
	}

	installedV, err := semver.Parse(platformSummary.GetInstalledVersion())
	if err != nil {
		return nil, fmt.Errorf("invalid installed version '%s': %w", platformSummary.GetInstalledVersion(), err)
	}

	availableReleases := make([]string, 0, len(platformSummary.GetReleases()))
	for k := range platformSummary.GetReleases() {
		availableReleases = append(availableReleases, k)
	}

	bestVersion := selectBestVersion(availableReleases, installedV, a.constraint)

	if bestVersion == nil {
		return []update.UpgradablePackage{}, nil
	}

	if bestVersion.Equal(installedV) {
		return []update.UpgradablePackage{}, nil
	}

	return []update.UpgradablePackage{{
		Type:        update.Arduino,
		Name:        a.platform.PlatformID,
		FromVersion: platformSummary.GetInstalledVersion(),
		ToVersion:   bestVersion.String(),
	}}, nil
}

func selectBestVersion(available []string, installed *semver.Version, constraint semver.Constraint) *semver.Version {
	candidates := make([]*semver.Version, 0, len(available))

	for _, verStr := range available {
		v, err := semver.Parse(verStr)
		if err != nil {
			continue
		}

		if !constraint.Match(v) {
			continue
		}
		if installed != nil && v.LessThan(installed) {
			continue
		}

		candidates = append(candidates, v)
	}

	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b *semver.Version) int {
		return a.CompareTo(b)
	})

	return candidates[len(candidates)-1]
}

// UpgradePackages implements ServiceUpdater.
func (a *ArduinoPlatformUpdater) UpgradePackages(ctx context.Context, packages []update.PackageInfo, eventCB update.EventCallback) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if len(packages) == 0 {
		return nil
	}
	if len(packages) > 1 {
		return fmt.Errorf("expected exactly one package to upgrade, got %d", len(packages))
	}
	pkg := packages[0]
	if pkg.Name != a.platform.PlatformID {
		return fmt.Errorf("unexpected package name '%s': this updater only supports '%s'", pkg.Name, a.platform.PlatformID)
	}
	targetVersion := pkg.ToVersion
	if targetVersion == "" {
		return fmt.Errorf("target version is empty for package '%s'", pkg.Name)
	}

	downloadProgressCB := func(curr *rpc.DownloadProgress) {
		data := helpers.ArduinoCLIDownloadProgressToString(curr)
		slog.Debug("Download progress", slog.String("download_progress", data))
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, data))
	}
	taskProgressCB := func(msg *rpc.TaskProgress) {
		data := helpers.ArduinoCLITaskProgressToString(msg)
		slog.Debug("Task progress", slog.String("task_progress", data))
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, data))
	}

	eventCB(update.NewDataEvent(update.StartEvent, "Upgrade is starting"))

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := orchestrator.SetArduinoCliConfig(ctx, srv); err != nil {
		return fmt.Errorf("error setting config: %w", err)
	}

	var inst *rpc.Instance
	if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
		return fmt.Errorf("error creating arduino-cli instance: %w", err)
	} else {
		inst = resp.GetInstance()
	}
	defer func() {
		_, err := srv.CleanDownloadCacheDirectory(ctx, &rpc.CleanDownloadCacheDirectoryRequest{})
		if err != nil {
			slog.Error("Error cleaning cache directory", slog.Any("error", err))
		}
		_, _ = srv.Destroy(ctx, &rpc.DestroyRequest{Instance: inst})
	}()

	{
		stream, _ := commands.UpdateIndexStreamResponseToCallbackFunction(ctx, downloadProgressCB)
		if err := srv.UpdateIndex(&rpc.UpdateIndexRequest{Instance: inst}, stream); err != nil {
			return fmt.Errorf("error updating index: %w", err)
		}
		if err := srv.Init(&rpc.InitRequest{Instance: inst}, commands.InitStreamResponseToCallbackFunction(ctx, nil)); err != nil {
			return fmt.Errorf("error initializing instance: %w", err)
		}
	}

	stream := commands.PlatformInstallStreamResponseToCallbackFunction(
		ctx,
		downloadProgressCB,
		taskProgressCB,
	)

	if err := srv.PlatformInstall(
		&rpc.PlatformInstallRequest{
			Instance:        inst,
			PlatformPackage: "arduino",
			Architecture:    "zephyr",
			Version:         targetVersion,
		},
		stream,
	); err != nil {
		return fmt.Errorf("error installing platform version %s: %w", targetVersion, err)
	}

	cbw := orchestrator.NewCallbackWriter(func(line string) {
		eventCB(update.NewDataEvent(update.UpgradeLineEvent, line))
	})

	err := srv.BurnBootloader(
		&rpc.BurnBootloaderRequest{
			Instance:   inst,
			Fqbn:       a.platform.FQBN,
			Programmer: "jlink",
		},
		commands.BurnBootloaderToServerStreams(ctx, cbw, cbw),
	)
	if err != nil {
		return fmt.Errorf("error burning bootloader: %w", err)
	}

	return nil
}
