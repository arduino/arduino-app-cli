package appsync

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/adb"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

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

const adbdContainerName = "adbd-testing"
const adbPort = "6666"

func startAdbDaemon(t *testing.T) {
	cmd := exec.Command("docker", "build", "-t", "adbd", ".")
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}
	cmd.Dir = filepath.Join(dir, "../../adbd")
	err = cmd.Run()
	if err != nil {
		t.Fatalf("failed to build adb daemon: %v", err)
	}

	err = exec.Command("docker", "run", "-d", "--rm", "--name", adbdContainerName, "-p", adbPort+":5555", "adbd").Run()
	if err != nil {
		t.Fatalf("failed to start adb daemon: %v", err)
	}
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			t.Fatalf("adb daemon did not start within the timeout period")
		case <-tick:
			out, err := exec.Command("adb", "connect", "localhost:"+adbPort).CombinedOutput()
			if err == nil && strings.Contains(string(out), "connected to localhost:"+adbPort) {
				return // adb daemon is ready
			}
		}
	}
}

func stopAdbDaemon(t *testing.T) {
	out, err := exec.Command("docker", "rm", "-f", adbdContainerName).CombinedOutput()
	if err != nil {
		t.Logf("DEBUG: adb daemon stop output: %q", string(out))
	}
}

func runAdbCmd(t *testing.T, args ...string) {
	adbPath := getAdbPath()
	out, err := exec.Command(adbPath, "connect", "localhost:"+adbPort).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to connect to adb daemon: %v: %s", err, string(out))
	}
	out, err = exec.Command(adbPath, append([]string{"-s", "localhost:" + adbPort, "shell"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run adb command: %v: %s", err, string(out))
	}
}

func TestEnableSyncApp(t *testing.T) {
	t.Cleanup(func() {
		stopAdbDaemon(t)
	})
	startAdbDaemon(t)

	getFiles := func(f fs.FS) []string {
		var files []string
		err := fs.WalkDir(f, ".", func(p string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == "." {
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			t.Fatalf("failed to walk dir: %v", err)
		}
		slices.Sort(files)
		return files
	}

	runAdbCmd(t, "mkdir", "/apps/test")
	runAdbCmd(t, "mkdir", "/apps/test/python")
	runAdbCmd(t, "touch", "/apps/test/python/main.py")
	runAdbCmd(t, "mkdir", "/apps/test/sketch/")
	runAdbCmd(t, "touch", "/apps/test/sketch/sketch.ino")
	runAdbCmd(t, "touch", "/apps/test/app.yml")

	sync, err := NewAppsSync()
	require.NoError(t, err)
	sync.Host = "localhost:" + adbPort
	tmp, err := sync.EnableSyncApp("test")
	require.NoError(t, err)
	files := getFiles(os.DirFS(tmp))
	require.Equal(t, []string{
		"app.yml",
		"python",
		"python/main.py",
		"sketch",
		"sketch/sketch.ino",
	}, files)

	t.Run("test new file in root folder", func(t *testing.T) {
		err = os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("test"), 0600)
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the file is created
		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"python",
			"python/main.py",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)
	})

	t.Run("test new file in subdir", func(t *testing.T) {
		err = os.WriteFile(filepath.Join(tmp, "python", "test.txt"), []byte("test"), 0600)
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the file is created
		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"python",
			"python/main.py",
			"python/test.txt",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)
	})

	t.Run("test new dir", func(t *testing.T) {
		err = os.MkdirAll(filepath.Join(tmp, "python", "test"), 0700)
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)
		// check if the dir is created

		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"python",
			"python/main.py",
			"python/test",
			"python/test.txt",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)

		// add file in the new dir
		err = os.WriteFile(filepath.Join(tmp, "python", "test", "test.txt"), []byte("test"), 0600)
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the file is created
		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"python",
			"python/main.py",
			"python/test",
			"python/test.txt",
			"python/test/test.txt",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)
	})

	t.Run("test update file", func(t *testing.T) {
		err = os.WriteFile(filepath.Join(tmp, "python/main.py"), []byte("print('Hello')"), 0600)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the file is updated
		buff, err := adb.CatOut("/apps/test/python/main.py", "localhost:"+adbPort)
		require.NoError(t, err)
		b, err := io.ReadAll(buff)
		require.NoError(t, err)
		require.Equal(t, "print('Hello')", string(b))
	})

	t.Run("test delete file", func(t *testing.T) {
		err = os.Remove(filepath.Join(tmp, "python/test.txt"))
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the file is deleted
		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"python",
			"python/main.py",
			"python/test",
			"python/test/test.txt",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)
	})

	t.Run("test delete dir", func(t *testing.T) {
		err = os.RemoveAll(filepath.Join(tmp, "python"))
		require.NoError(t, err)

		// wait for the event to be triggered
		time.Sleep(1 * time.Second)

		// check if the dir is deleted
		files = getFiles(adbfs.AdbFS{Base: "/apps/test", Host: "localhost:" + adbPort})
		require.Equal(t, []string{
			"app.yml",
			"sketch",
			"sketch/sketch.ino",
			"test.txt",
		}, files)
	})
}
