package testtools

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
)

func StartAdbDContainer(t *testing.T) (string, string, string) {
	if runtime.GOOS != "linux" && os.Getenv("CI") != "" {
		t.Skip("Skipping tests in CI that requires docker on non-Linux systems")
	}
	t.Helper()

	cmd := exec.Command("docker", "build", "-t", "adbd", "-f", "adbd.Dockerfile", ".")
	cmd.Dir = getBaseProjectPath(t)
	err := cmd.Run()
	if err != nil {
		t.Fatalf("failed to build adb daemon: %v", err)
	}

	containerName := genContainerName(t)
	var adbPort, sshPort string
	for range 10 {
		adbPort = getRandPort(t)
		sshPort = getRandPort(t)
		out, err := exec.Command("docker", "run", "-d", "--rm", "--name", containerName, "-p", adbPort+":5555", "-p", sshPort+":22", "adbd").CombinedOutput()
		if err == nil {
			break
		}
		t.Logf("attempt to start adb container with port %q, %q: %s, %s", adbPort, sshPort, err, strings.TrimSpace(string(out)))
	}

	adbPath := adb.FindAdbPath()
	for {
		select {
		case <-time.After(10 * time.Second):
			t.Fatalf("adb daemon did not start within the timeout period")
		case <-time.Tick(500 * time.Millisecond):
			out, err := exec.Command(adbPath, "connect", "localhost:"+adbPort).CombinedOutput()
			if err == nil && strings.Contains(string(out), "connected to localhost:"+adbPort) {
				return containerName, adbPort, sshPort
			}
		}
	}
}

func StopAdbDContainer(t *testing.T, name string) {
	t.Helper()

	out, err := exec.Command("docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		t.Logf("adb daemon stop output: %v: %v", err, string(out))
	}
}

func genContainerName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("adbd-testing-%d", time.Now().UnixNano())
}

func getRandPort(t *testing.T) string {
	t.Helper()

	// Random port between 1000 and 9999
	port := 1000 + rand.IntN(9000) // nolint:gosec
	return strconv.Itoa(port)
}

func getBaseProjectPath(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}
		parentDir := filepath.Dir(dir)
		if parentDir == dir { // Reached the root directory
			break
		}
		dir = parentDir
	}

	t.Fatalf("go.mod not found in any parent directory")
	return "" // Unreachable, but required for compilation
}
