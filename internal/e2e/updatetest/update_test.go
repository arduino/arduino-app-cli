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

		tagAppCli := fetchDebPackageLatest(t, "build/stable", "arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino-router")
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

		tagAppCli := fetchDebPackageLatest(t, "build", "arduino-app-cli")
		fetchDebPackageLatest(t, "build/stable", "arduino-router")
		minorTag := genMinorTag(t, tagAppCli)

		fmt.Printf("Updating from unstable version %s to stable version %s \n", minorTag, tagAppCli)
		fmt.Printf("Building local deb version %s \n", minorTag)
		buildDebVersion(t, "build/stable", minorTag, arch)

		fmt.Println("**** BUILD docker image *****")
		const dockerImageName = "test-apt-update-unstable-image"

		buildDockerImage(t, dockerFile, dockerImageName, arch)
		//TODO: t cleanup remove docker image

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
