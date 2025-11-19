package startapp

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var arch = runtime.GOARCH

const (
	dockerFile    = "test.Dockerfile"
	daemonHost    = "127.0.0.1:8800"
	versionToTest = "v1.0.0"
)

func TestStartApp(t *testing.T) {
	fmt.Printf("***** ARCH %s ***** \n", arch)

	t.Cleanup(func() { os.RemoveAll("build") })

	fmt.Printf("Building local deb version %s \n", versionToTest)
	buildDebVersion(t, "build", versionToTest, arch)

	fmt.Println("Fetching 'arduino-router' dependency...")
	fetchDebPackageLatest(t, "build", "arduino-router")

	const dockerImageName = "e2e-start-test-image"
	fmt.Println("**** BUILD docker image (e2e-test-runner) *****")
	buildDockerImage(t, dockerFile, dockerImageName, arch)

	t.Run("Test App Start Command", func(t *testing.T) {

		const containerA_Name = "e2e-test-runner"
		const appToStart = "user:helloworld"

		t.Cleanup(func() {
			//stopDockerContainer(t, containerA_Name)
			//stopAppContainer(t, appToStart)
		})

		fmt.Println("**** RUN docker image (board container) *****")
		startDockerContainer(t, containerA_Name, dockerImageName)

		waitForPort(t, daemonHost, 20*time.Second)

		fmt.Println("**** Creating user app 'user:helloworld' *****")
		postCreateApp(t, daemonHost)

		fmt.Printf("**** Telling e2e-test-runner to start app '%s' *****\n", appToStart)
		runAppStart(t, containerA_Name, appToStart)

		fmt.Printf("**** Verifying on HOST if '%s' ( app to start) is running *****\n", appToStart)

		time.Sleep(1 * time.Second)

		isRunning := checkContainerRunningOnHost(t, appToStart)

		require.True(t, isRunning, "Il container B (%s) not foud", appToStart)

		fmt.Printf("Success: e2e-test-runner successfully launched  app to start (%s) on host.\n", appToStart)
	})
}
