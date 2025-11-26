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
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/arduino/arduino-cli/commands"
	"github.com/arduino/arduino-cli/commands/cmderrors"
	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
	"github.com/sirupsen/logrus"

	"github.com/arduino/arduino-app-cli/internal/helpers"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
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
func (a *ArduinoPlatformUpdater) ListUpgradablePackages(ctx context.Context, _ func(update.UpgradablePackage) bool) ([]update.UpgradablePackage, error) {
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

	if platformSummary.GetLatestVersion() == platformSummary.GetInstalledVersion() {
		return nil, nil // No update available
	}

	return []update.UpgradablePackage{{
		Type:        update.Arduino,
		Name:        "arduino:zephyr",
		FromVersion: platformSummary.GetInstalledVersion(),
		ToVersion:   platformSummary.GetLatestVersion(),
	}}, nil
}

// UpgradePackages implements ServiceUpdater.
func (a *ArduinoPlatformUpdater) UpgradePackages(ctx context.Context, names []string) (<-chan update.Event, error) {
	if !a.lock.TryLock() {
		return nil, update.ErrOperationAlreadyInProgress
	}
	eventsCh := make(chan update.Event, 100)

	go func() {
		defer a.lock.Unlock()
		defer close(eventsCh)

		const indexBase float32 = 0.0
		const indexWeight float32 = 30.0
		const upgradeBase float32 = 30.0
		const upgradeWeight float32 = 60.0

		makeDownloadProgressCallback := func(basePercentage, phaseWeight float32) func(*rpc.DownloadProgress) {
			return func(curr *rpc.DownloadProgress) {
				data := helpers.ArduinoCLIDownloadProgressToString(curr)
				eventsCh <- update.NewDataEvent(update.UpgradeLineEvent, data)
				if updateInfo := curr.GetUpdate(); updateInfo != nil {
					if updateInfo.GetTotalSize() <= 0 {
						return
					}
					localProgress := (float32(updateInfo.GetDownloaded()) / float32(updateInfo.GetTotalSize())) * 100.0
					totalArduinoProgress := basePercentage + (localProgress/100.0)*phaseWeight
					eventsCh <- update.NewProgressEvent(totalArduinoProgress)
				}
			}
		}
		makeTaskProgressCallback := func(basePercentage, phaseWeight float32) func(*rpc.TaskProgress) {
			return func(msg *rpc.TaskProgress) {
				data := helpers.ArduinoCLITaskProgressToString(msg)
				eventsCh <- update.NewDataEvent(update.UpgradeLineEvent, data)
				if !msg.GetCompleted() {
					localProgress := msg.GetPercent()
					totalArduinoProgress := basePercentage + (localProgress/100.0)*phaseWeight
					eventsCh <- update.NewProgressEvent(totalArduinoProgress)
				}
			}
		}

		eventsCh <- update.NewDataEvent(update.StartEvent, "Upgrade is starting")

		logrus.SetLevel(logrus.ErrorLevel)
		srv := commands.NewArduinoCoreServer()

		if err := setConfig(ctx, srv); err != nil {
			eventsCh <- update.NewErrorEvent(fmt.Errorf("error setting config: %w", err))
			return
		}

		var inst *rpc.Instance
		if resp, err := srv.Create(ctx, &rpc.CreateRequest{}); err != nil {
			eventsCh <- update.NewErrorEvent(fmt.Errorf("error creating arduino-cli instance: %w", err))
			return
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
			updateIndexProgressCB := makeDownloadProgressCallback(indexBase, indexWeight)
			stream, _ := commands.UpdateIndexStreamResponseToCallbackFunction(ctx, updateIndexProgressCB)
			if err := srv.UpdateIndex(&rpc.UpdateIndexRequest{Instance: inst}, stream); err != nil {
				eventsCh <- update.NewErrorEvent(fmt.Errorf("error updating index: %w", err))
				return
			}

			eventsCh <- update.NewProgressEvent(indexBase + indexWeight)

			if err := srv.Init(&rpc.InitRequest{Instance: inst}, commands.InitStreamResponseToCallbackFunction(ctx, nil)); err != nil {
				eventsCh <- update.NewErrorEvent(fmt.Errorf("error initializing instance: %w", err))
				return
			}
		}

		platformDownloadCB := makeDownloadProgressCallback(upgradeBase, upgradeWeight)
		platformTaskCB := makeTaskProgressCallback(upgradeBase, upgradeWeight)

		stream, respCB := commands.PlatformUpgradeStreamResponseToCallbackFunction(
			ctx,
			platformDownloadCB,
			platformTaskCB,
		)
		if err := srv.PlatformUpgrade(
			&rpc.PlatformUpgradeRequest{
				Instance:         inst,
				PlatformPackage:  "arduino",
				Architecture:     "zephyr",
				SkipPostInstall:  false,
				SkipPreUninstall: false,
			},
			stream,
		); err != nil {
			var alreadyPresent *cmderrors.PlatformAlreadyAtTheLatestVersionError
			if errors.As(err, &alreadyPresent) {
				eventsCh <- update.NewDataEvent(update.UpgradeLineEvent, alreadyPresent.Error())
				return
			}

			var notFound *cmderrors.PlatformNotFoundError
			if !errors.As(err, &notFound) {
				eventsCh <- update.NewErrorEvent(fmt.Errorf("error upgrading platform: %w", err))
				return
			}
			// If the platform is not found, we will try to install it
			err := srv.PlatformInstall(
				&rpc.PlatformInstallRequest{
					Instance:        inst,
					PlatformPackage: "arduino",
					Architecture:    "zephyr",
				},
				commands.PlatformInstallStreamResponseToCallbackFunction(
					ctx,
					platformDownloadCB,
					platformTaskCB,
				),
			)
			if err != nil {
				eventsCh <- update.NewErrorEvent(fmt.Errorf("error installing platform: %w", err))
				return
			}
		} else if respCB().GetPlatform() == nil {
			eventsCh <- update.NewErrorEvent(fmt.Errorf("platform upgrade failed"))
			return
		}

		cbw := orchestrator.NewCallbackWriter(func(line string) {
			eventsCh <- update.NewDataEvent(update.UpgradeLineEvent, line)
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
			eventsCh <- update.NewErrorEvent(fmt.Errorf("error burning bootloader: %w", err))
			return
		}
		eventsCh <- update.NewProgressEvent(100.0)
	}()

	return eventsCh, nil
}
