package testtools

import (
	"context"
	"flag"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var arch = flag.String("arch", "amd64", "target architecture")

func TestStableToUnstable(t *testing.T) {
	fmt.Printf("***** ARCH %s ***** \n", *arch)
	tagAppCli := FetchDebPackage(t, "arduino-app-cli", "latest", *arch)
	FetchDebPackage(t, "arduino-router", "latest", *arch)
	majorTag := majorTag(t, tagAppCli)
	_ = minorTag(t, tagAppCli)

	fmt.Printf("Updating from stable version %s to unstable version %s \n", tagAppCli, majorTag)
	fmt.Printf("Building local deb version %s \n", majorTag)
	buildDebVersion(t, majorTag, *arch)
	// fmt.Printf("Check folder structure and deb downloaded\n")
	// ls(t)
	fmt.Println("**** BUILD docker image *****")
	buildDockerImage(t, "test.Dockerfile", "apt-test-update-image", *arch)
	fmt.Println("**** RUN docker image *****")
	runDockerContainer(t, "apt-test-update", "apt-test-update-image")
	preUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerSystemUpdate(t, "apt-test-update")
	postUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerCleanUp(t, "apt-test-update")
	require.Equal(t, preUpdateVersion, "Arduino App CLI "+tagAppCli+"\n")
	require.Equal(t, postUpdateVersion, "Arduino App CLI "+majorTag+"\n")
}

func TestClientUpdate(t *testing.T) {

	fmt.Printf("***** ARCH %s ***** \n", *arch)
	tagAppCli := FetchDebPackage(t, "arduino-app-cli", "latest", *arch)
	FetchDebPackage(t, "arduino-router", "latest", *arch)
	majorTag := majorTag(t, tagAppCli)

	fmt.Println("**** RUN docker image *****")
	runDockerContainer(t, "apt-test-update", "apt-test-update-image")
	preUpdateVersion := runDockerSystemVersion(t, "apt-test-update")

	runDockerDaemon(t, "apt-test-update")
	time.Sleep(5 * time.Second) //wait for the daemon to be fully started
	status := putUpdateRequest(t, "http://127.0.0.1:8800/v1/system/update/apply")
	fmt.Printf("Response status: %s\n", status)

	itr := NewSSEClient(context.Background(), "GET", "http://localhost:8800/v1/system/update/events")

	for event, err := range itr {
		if err != nil {
			log.Printf("Error receiving SSE event: %v", err)
		}
		fmt.Printf("Received event: ID=%s, Event=%s, Data=%s\n", event.ID, event.Event, string(event.Data))
		if string(event.Data) == "Download complete" {
			fmt.Println("✅ Download complete — exiting successfully.")
		}
	}

	postUpdateVersion := runDockerSystemVersion(t, "apt-test-update")

	require.Equal(t, preUpdateVersion, "Arduino App CLI "+tagAppCli+"\n")
	require.Equal(t, postUpdateVersion, "Arduino App CLI "+majorTag+"\n")
	runDockerCleanUp(t, "apt-test-update")

}

func TestUnstableToStable(t *testing.T) {
	tagAppCli := FetchDebPackage(t, "arduino-app-cli", "latest", *arch)
	FetchDebPackage(t, "arduino-router", "latest", *arch)
	minorTag := minorTag(t, tagAppCli)
	moveDeb(t, "build/stable", "build/", "arduino-app-cli", tagAppCli, *arch)

	fmt.Printf("Updating from unstable version %s to stable version %s \n", minorTag, tagAppCli)
	fmt.Printf("Building local deb version %s \n", minorTag)
	buildDebVersion(t, minorTag, *arch)
	moveDeb(t, "build/", "build/stable", "arduino-app-cli", minorTag, *arch)

	fmt.Printf("Check folder structure and deb downloaded\n")
	ls(t)

	fmt.Println("**** BUILD docker image *****")
	buildDockerImage(t, "test.Dockerfile", "test-apt-update-unstable-image", *arch)
	fmt.Println("**** RUN docker image *****")
	runDockerContainer(t, "apt-test-update-unstable", "test-apt-update-unstable-image")
	preUpdateVersion := runDockerSystemVersion(t, "apt-test-update-unstable")
	runDockerSystemUpdate(t, "apt-test-update-unstable")
	postUpdateVersion := runDockerSystemVersion(t, "apt-test-update-unstable")
	runDockerCleanUp(t, "apt-test-update-unstable")
	require.Equal(t, preUpdateVersion, "Arduino App CLI "+minorTag+"\n")
	require.Equal(t, postUpdateVersion, "Arduino App CLI "+tagAppCli+"\n")

}
