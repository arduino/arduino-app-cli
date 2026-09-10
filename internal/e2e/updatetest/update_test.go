// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package updatetest

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

var arch = runtime.GOARCH

const dockerFile = "test.Dockerfile"

func TestUpdatePackage(t *testing.T) {
	fmt.Printf("***** ARCH %s ***** \n", arch)

	// Test that the upgrade works from the current version to a newer one (created on the fly).
	t.Run("StableToCurrent", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		tagAppCli := fetchDebPackageLatest(t, "build/stable", "arduino/arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")

		majorTag := genMajorTag(t, tagAppCli)
		t.Logf("Updating from stable version %s to unstable version %s", tagAppCli, majorTag)

		t.Logf("Building local deb version %s \n", majorTag)
		buildDebVersion(t, "build", majorTag, arch)

		const dockerImageName = "apt-test-update-image"
		t.Logf("Build docker image %s", dockerImageName)
		buildDockerImage(t, dockerFile, dockerImageName, arch)
		t.Cleanup(func() { removeDockerImage(t, dockerImageName) })

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)

			runSystemUpdate(t, containerName)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update-http"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)

			putUpdateRequest(t, daemonHost)
			waitForUpgrade(t, daemonHost)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})
	})

	// Test the steady state, with both ends of the upgrade built from the current
	// source: it is the only case where the changes in the branch are in place
	// both while the upgrade runs and after it.
	t.Run("CurrentToCurrent", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		// Both debs are built here, so the versions only need to be ordered.
		const fromTag = "v9.9.0"
		const toTag = "v9.9.1"

		fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")

		t.Logf("Updating from current version %s to current version %s", fromTag, toTag)
		t.Logf("build deb version %s", fromTag)
		buildDebVersion(t, "build/stable", fromTag, arch)
		t.Logf("build deb version %s", toTag)
		buildDebVersion(t, "build", toTag, arch)

		const dockerImageName = "test-apt-update-current-image"
		t.Logf("build docker image %s", dockerImageName)
		buildDockerImage(t, dockerFile, dockerImageName, arch)
		t.Cleanup(func() { removeDockerImage(t, dockerImageName) })

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update-current"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, fromTag)

			runSystemUpdate(t, containerName)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, toTag)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update-current-http"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, fromTag)

			putUpdateRequest(t, daemonHost)
			waitForUpgrade(t, daemonHost)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, toTag)
		})
	})

	// The destination is a deb published before this branch, so none of the
	// changes in the current source survive the upgrade. A failure here means
	// this version cannot be downgraded, not that the upgrade is broken.
	t.Run("DowngradeToPublishedRelease", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		tagAppCli := fetchDebPackageLatest(t, "build", "arduino/arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")

		minorTag := genMinorTag(t, tagAppCli)
		t.Logf("Updating from unstable version %s to stable version %s", minorTag, tagAppCli)

		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("DOWNGRADE NOT SUPPORTED: the published %s predates this build, so nothing "+
					"%s changes in the package survives the upgrade", tagAppCli, minorTag)
			}
		})

		t.Logf("build deb version %s", minorTag)
		buildDebVersion(t, "build/stable", minorTag, arch)

		const dockerImageName = "test-apt-update-unstable-image"
		t.Logf("build docker image %s", dockerImageName)
		buildDockerImage(t, dockerFile, dockerImageName, arch)
		t.Cleanup(func() { removeDockerImage(t, dockerImageName) })

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update-unstable"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)

			runSystemUpdate(t, containerName)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update--unstable-http"
			startDaemonContainer(t, containerName, dockerImageName)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)

			putUpdateRequest(t, daemonHost)
			waitForUpgrade(t, daemonHost)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})
	})
}
