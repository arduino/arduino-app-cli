package testtools

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func StartAdbDContainer(t *testing.T) (string, string, string) {
	t.Helper()

	cmd := exec.Command("docker", "build", "-t", "adbd", ".")
	base := getBaseProjectPath(t)
	cmd.Dir = filepath.Join(base, "adbd")
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

	adbPath := getAdbPath()
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

func getAdbPath() string {
	// Attempt to find the adb path in the Arduino15 directory
	const arduino15adbPath = "packages/arduino/tools/adb/32.0.0/adb"
	var path string
	switch runtime.GOOS {
	case "darwin":
		user, err := user.Current()
		if err != nil {
			fmt.Println("WARNING: Unable to get current user:", err)
			break
		}
		path = filepath.Join(user.HomeDir, "/Library/Arduino15/", arduino15adbPath)
	case "linux":
		user, err := user.Current()
		if err != nil {
			fmt.Println("WARNING: Unable to get current user:", err)
			break
		}
		path = filepath.Join(user.HomeDir, ".arduino15/", arduino15adbPath)
	case "windows":
		user, err := user.Current()
		if err != nil {
			fmt.Println("WARNING: Unable to get current user:", err)
			break
		}
		path = filepath.Join(user.HomeDir, "AppData/Local/Arduino15/", arduino15adbPath)
	}
	s, err := os.Stat(path)
	if err == nil && !s.IsDir() {
		return path
	}
	return "adb"
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
