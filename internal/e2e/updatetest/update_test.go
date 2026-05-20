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
	"time"

	"github.com/stretchr/testify/require"
)

var arch = runtime.GOARCH

const dockerFile = "test.Dockerfile"
const daemonHost = "127.0.0.1:8800"

// logStep logs the step name with a timestamp, runs fn, then logs how long it took.
func logStep(t *testing.T, name string, fn func()) {
	t.Helper()
	fmt.Printf("➡️ [%s] %s starting...\n", time.Now().Format("15:04:05"), name)
	start := time.Now()
	fn()
	fmt.Printf("⬅️ [%s] %s done in %s\n", time.Now().Format("15:04:05"), name, time.Since(start).Round(time.Millisecond))
}

func TestUpdatePackage(t *testing.T) {
	fmt.Printf("***** ARCH %s ***** \n", arch)

	t.Run("StableToCurrent", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		var tagAppCli string
		logStep(t, "fetch latest stable packages (StableToCurrent)", func() {
			tagAppCli = fetchDebPackageLatest(t, "build/stable", "arduino/arduino-app-cli")
			fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")
			fetchDebPackageLatest(t, "build/stable", "bcmi-labs/arduino-deb-packages")
		})

		majorTag := genMajorTag(t, tagAppCli)
		t.Logf("Updating from stable version %s to unstable version %s", tagAppCli, majorTag)

		logStep(t, fmt.Sprintf("build deb version %s", majorTag), func() {
			buildDebVersion(t, "build", majorTag, arch)
		})

		const dockerImageName = "apt-test-update-image"
		logStep(t, fmt.Sprintf("build docker image %s", dockerImageName), func() {
			buildDockerImage(t, dockerFile, dockerImageName, arch)
		})
		t.Cleanup(func() { removeDockerImage(t, dockerImageName) })

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			logStep(t, fmt.Sprintf("start container %s and wait for daemon", containerName), func() {
				startDockerContainer(t, containerName, dockerImageName)
				waitForPort(t, daemonHost, 5*time.Second)
			})

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)

			logStep(t, "system update (CLI command)", func() {
				runSystemUpdate(t, containerName)
			})

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update-http"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			logStep(t, fmt.Sprintf("start container %s and wait for daemon", containerName), func() {
				startDockerContainer(t, containerName, dockerImageName)
				waitForPort(t, daemonHost, 5*time.Second)
			})

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)

			logStep(t, "system update (HTTP request)", func() {
				putUpdateRequest(t, daemonHost)
				waitForUpgrade(t, daemonHost)
			})

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})

	})

	t.Run("CurrentToStable", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		var tagAppCli string
		logStep(t, "fetch latest stable packages (CurrentToStable)", func() {
			tagAppCli = fetchDebPackageLatest(t, "build", "arduino/arduino-app-cli")
			fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")
			fetchDebPackageLatest(t, "build/stable", "bcmi-labs/arduino-deb-packages")
		})

		minorTag := genMinorTag(t, tagAppCli)
		t.Logf("Updating from unstable version %s to stable version %s", minorTag, tagAppCli)

		logStep(t, fmt.Sprintf("build deb version %s", minorTag), func() {
			buildDebVersion(t, "build/stable", minorTag, arch)
		})

		const dockerImageName = "test-apt-update-unstable-image"
		logStep(t, fmt.Sprintf("build docker image %s", dockerImageName), func() {
			buildDockerImage(t, dockerFile, dockerImageName, arch)
		})
		t.Cleanup(func() { removeDockerImage(t, dockerImageName) })

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update-unstable"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			logStep(t, fmt.Sprintf("start container %s and wait for daemon", containerName), func() {
				startDockerContainer(t, containerName, dockerImageName)
				waitForPort(t, daemonHost, 5*time.Second)
			})

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)

			logStep(t, "system update (CLI command)", func() {
				runSystemUpdate(t, containerName)
			})

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update--unstable-http"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			logStep(t, fmt.Sprintf("start container %s and wait for daemon", containerName), func() {
				startDockerContainer(t, containerName, dockerImageName)
				waitForPort(t, daemonHost, 5*time.Second)
			})

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)

			logStep(t, "system update (HTTP request)", func() {
				putUpdateRequest(t, daemonHost)
				waitForUpgrade(t, daemonHost)
			})

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})

	})

}
