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
	// TODO: aggiungere t.Cleanup per rimuovere l'immagine docker. da controllare se mi serve!

	t.Run("Test App Start Command", func(t *testing.T) {

		const containerA_Name = "e2e-test-runner"
		const containerB_AppName = "examples:object-detection"

		//todo devo creare un app senza sketch, ma con qualche brick magari! oppure usare un esempio?

		t.Cleanup(func() {
			stopDockerContainer(t, containerA_Name)
			stopAppContainer(t, containerB_AppName)
		})

		fmt.Println("**** RUN docker image (board container) *****")
		startDockerContainer(t, containerA_Name, dockerImageName)

		waitForPort(t, daemonHost, 20*time.Second)

		fmt.Printf("**** Telling e2e-test-runner to start app '%s' *****\n", containerB_AppName)
		runAppStart(t, containerA_Name, containerB_AppName)

		fmt.Printf("**** Verifying on HOST if '%s' (Container B) is running *****\n", containerB_AppName)

		time.Sleep(1 * time.Second)

		isRunning := checkContainerRunningOnHost(t, containerB_AppName)

		require.True(t, isRunning, "Il container B (%s) not foud", containerB_AppName)

		fmt.Printf("Success: e2e-test-runner successfully launched Container B (%s) on host.\n", containerB_AppName)
	})
}
