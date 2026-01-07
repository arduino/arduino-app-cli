// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package arduino

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/update"
)

type ArduinoPlatformUpdater struct {
	lock sync.Mutex
}

func NewArduinoPlatformUpdater() *ArduinoPlatformUpdater {
	return &ArduinoPlatformUpdater{}
}

func setConfig(ctx context.Context, srv rpc.ArduinoCoreServiceServer) error {
	if _, err := srv.SettingsSetValue(ctx, &rpc.SettingsSetValueRequest{
		Key:          "network.connection_timeout",
		EncodedValue: "600s",
		ValueFormat:  "cli",
	}); err != nil {
		return err
	}

	return nil
}

// ListUpgradablePackages implements ServiceUpdater.
func (a *ArduinoPlatformUpdater) ListUpgradablePackages(cfg config.Configuration, ctx context.Context, _ func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
	if !a.lock.TryLock() {
		return nil, update.ErrOperationAlreadyInProgress
	}
	defer a.lock.Unlock()

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()
	if err := setConfig(ctx, srv); err != nil {
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
		if v.GetMetadata().GetId() == "arduino:zephyr" {
			platformSummary = v
			break
		}
	}

	if platformSummary == nil {
		return nil, nil // No platform found
	}
	releasesMap := platformSummary.GetReleases()

	releases := make([]string, 0, len(releasesMap))

	for k := range releasesMap {
		releases = append(releases, k)
	}
	bestVersion, err := findBestCandidate(
		platformSummary.GetInstalledVersion(),
		releases,
		cfg.VersionConstraint,
	)

	if bestVersion == "" || err != nil {
		return nil, nil
	}
	return []update.UpgradablePackage{{
		Type:        update.Arduino,
		Name:        "arduino:zephyr",
		FromVersion: platformSummary.GetInstalledVersion(),
		ToVersion:   bestVersion,
	}}, nil
}

func findBestCandidate(installedStr string, availableVersions []string, constraint semver.Constraint) (string, error) {
	installedV, err := semver.Parse(installedStr)
	if err != nil {
		return "", fmt.Errorf("invalid installed version '%s': %w", installedStr, err)
	}

	if !constraint.Match(installedV) {

		vStr := string(installedV.NormalizedString())
		vStr = strings.TrimPrefix(vStr, "v")
		parts := strings.Split(vStr, ".")
		if len(parts) > 0 {
			majorInt, err := strconv.Atoi(parts[0])
			if err == nil {
				newConstraintStr := fmt.Sprintf("<%d.0.0", majorInt+1)
				newC, err := semver.ParseConstraint(newConstraintStr)
				if err == nil {
					constraint = newC
				}
			}
		}
	}

	var bestUpdateV *semver.Version

	for _, verStr := range availableVersions {
		candidate, err := semver.Parse(verStr)
		if err != nil {
			continue
		}

		if !constraint.Match(candidate) {
			continue
		}

		if candidate.LessThan(installedV) {
			continue
		}

		if bestUpdateV == nil || candidate.GreaterThan(bestUpdateV) {
			bestUpdateV = candidate
		}
	}

	if bestUpdateV == nil {
		return "", nil
	}

	return bestUpdateV.String(), nil
}

// UpgradePackages implements ServiceUpdater.
func (a *ArduinoPlatformUpdater) UpgradePackages(ctx context.Context, packages []update.PackageInfo, eventCB update.EventCallback) error {
	if !a.lock.TryLock() {
		return update.ErrOperationAlreadyInProgress
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

	defer a.lock.Unlock()

	eventCB(update.NewDataEvent(update.StartEvent, "Upgrade is starting"))

	logrus.SetLevel(logrus.ErrorLevel) // Reduce the log level of arduino-cli
	srv := commands.NewArduinoCoreServer()

	if err := setConfig(ctx, srv); err != nil {
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

	var targetVersion string

	const CorePackageName = "arduino"

	for _, pkg := range packages {
		if pkg.Name == CorePackageName {
			targetVersion = pkg.ToVersion
			break
		}
	}

	if targetVersion == "" {
		if len(packages) > 0 {
			return fmt.Errorf("no package of type '%s' found in the upgrade request", CorePackageName)
		}
		return fmt.Errorf("package list is empty")
	}

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
			Fqbn:       "arduino:zephyr:unoq",
			Programmer: "jlink",
		},
		commands.BurnBootloaderToServerStreams(ctx, cbw, cbw),
	)
	if err != nil {
		return fmt.Errorf("error burning bootloader: %w", err)
	}

	return nil
}
