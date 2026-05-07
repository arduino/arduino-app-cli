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

func TestUpdatePackage(t *testing.T) {
	fmt.Printf("***** ARCH %s ***** \n", arch)

	t.Run("Stable To Current", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		tagAppCli := fetchDebPackageLatest(t, "build/stable", "arduino/arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")
		fetchDebPackageLatest(t, "build/stable", "bcmi-labs/arduino-deb-packages")

		majorTag := genMajorTag(t, tagAppCli)

		fmt.Printf("Updating from stable version %s to unstable version %s \n", tagAppCli, majorTag)
		fmt.Printf("Building local deb version %s \n", majorTag)
		buildDebVersion(t, "build", majorTag, arch)

		const dockerImageName = "apt-test-update-image"
		fmt.Println("**** BUILD docker image *****")
		buildDockerImage(t, dockerFile, dockerImageName, arch)
		//TODO: t cleanup remove docker image

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			fmt.Println("**** RUN docker image *****")
			startDockerContainer(t, containerName, dockerImageName)
			waitForPort(t, daemonHost, 5*time.Second)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)
			runSystemUpdate(t, containerName)
			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update-http"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			startDockerContainer(t, containerName, dockerImageName)
			waitForPort(t, daemonHost, 5*time.Second)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, tagAppCli)

			putUpdateRequest(t, daemonHost)
			waitForUpgrade(t, daemonHost)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, majorTag)
		})

	})

	t.Run("CurrentToStable", func(t *testing.T) {
		t.Cleanup(func() { os.RemoveAll("build") })

		tagAppCli := fetchDebPackageLatest(t, "build", "arduino/arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino/arduino-router")
		fetchDebPackageLatest(t, "build/stable", "bcmi-labs/arduino-deb-packages")

		minorTag := genMinorTag(t, tagAppCli)

		fmt.Printf("Updating from unstable version %s to stable version %s \n", minorTag, tagAppCli)
		fmt.Printf("Building local deb version %s \n", minorTag)
		buildDebVersion(t, "build/stable", minorTag, arch)

		fmt.Println("**** BUILD docker image *****")
		const dockerImageName = "test-apt-update-unstable-image"

		buildDockerImage(t, dockerFile, dockerImageName, arch)
		// TODO: t cleanup remove docker image

		t.Run("CLI Command", func(t *testing.T) {
			const containerName = "apt-test-update-unstable"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			fmt.Println("**** RUN docker image *****")
			startDockerContainer(t, containerName, dockerImageName)
			waitForPort(t, daemonHost, 5*time.Second)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)
			runSystemUpdate(t, containerName)
			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})

		t.Run("HTTP Request", func(t *testing.T) {
			const containerName = "apt-test-update--unstable-http"
			t.Cleanup(func() { stopDockerContainer(t, containerName) })

			startDockerContainer(t, containerName, dockerImageName)
			waitForPort(t, daemonHost, 5*time.Second)

			preUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+preUpdateVersion, minorTag)

			putUpdateRequest(t, daemonHost)
			waitForUpgrade(t, daemonHost)

			postUpdateVersion := getAppCliVersion(t, containerName)
			require.Equal(t, "v"+postUpdateVersion, tagAppCli)
		})

	})

}
