package testtools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func FetchDebPackage(t *testing.T, path, repo, version, arch string) string {
	t.Helper()

	cmd := exec.Command(
		"gh", "release", "list",
		"--repo", "github.com/arduino/"+repo,
		"--exclude-pre-releases",
		"--limit", "1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	fmt.Println(string(output))

	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		log.Fatal("could not parse tag from gh release list output")
	}
	tag := fields[0]
	tagPath := strings.TrimPrefix(tag, "v")

	debFile := fmt.Sprintf("%s/%s_%s-1_%s.deb", path, repo, tagPath, arch)
	fmt.Println(debFile)
	if _, err := os.Stat(debFile); err == nil {
		fmt.Printf("✅ %s already exists, skipping download.\n", debFile)
		return tag
	}
	fmt.Println("Detected tag:", tag)
	cmd2 := exec.Command(
		"gh", "release", "download",
		tag,
		"--repo", "github.com/arduino/"+repo,
		"--pattern", "*",
		"--dir", path,
	)

	out, err := cmd2.CombinedOutput()
	if err != nil {
		log.Fatalf("download failed: %v\nOutput: %s", err, out)
	}

	return tag

}

func buildDebVersion(t *testing.T, storePath, tagVersion, arch string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	outputDir := filepath.Join(cwd, storePath)

	cmd := exec.Command(
		"go", "tool", "task", "build-deb",
		fmt.Sprintf("VERSION=%s", tagVersion),
		fmt.Sprintf("ARCH=%s", arch),
		fmt.Sprintf("OUTPUT=%s", outputDir),
	)

	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run build command: %v", err)
	}
}

func majorTag(t *testing.T, tag string) string {
	t.Helper()

	parts := strings.Split(tag, ".")
	last := parts[len(parts)-1]

	lastNum, _ := strconv.Atoi(strings.TrimPrefix(last, "v"))
	lastNum++

	parts[len(parts)-1] = strconv.Itoa(lastNum)
	newTag := strings.Join(parts, ".")

	return newTag
}

func minorTag(t *testing.T, tag string) string {
	t.Helper()

	parts := strings.Split(tag, ".")
	last := parts[len(parts)-1]

	lastNum, _ := strconv.Atoi(strings.TrimPrefix(last, "v"))
	if lastNum > 0 {
		lastNum--
	}

	parts[len(parts)-1] = strconv.Itoa(lastNum)
	newTag := strings.Join(parts, ".")

	if !strings.HasPrefix(newTag, "v") {
		newTag = "v" + newTag
	}
	return newTag
}

func buildDockerImage(t *testing.T, dockerfile, name, arch string) {
	t.Helper()

	cmd := exec.Command("docker", "build", "--build-arg", "ARCH="+arch, "-t", name, "-f", dockerfile, ".")
	// Capture both stdout and stderr
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		fmt.Printf("❌ Docker build failed: %v\n", err)
		fmt.Printf("---- STDERR ----\n%s\n", stderr.String())
		fmt.Printf("---- STDOUT ----\n%s\n", out.String())
		return
	}

	fmt.Println("✅ Docker build succeeded!")
	fmt.Println(out.String())

}

func runDockerContainer(t *testing.T, containerName string, containerImageName string) {
	t.Helper()

	cmd := exec.Command(
		"docker", "run", "--rm", "-d",
		"-p", "8800:8800",
		"--privileged",
		"--cgroupns=host",
		"--network", "host",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", "DOCKER_HOST=unix:///var/run/docker.sock",
		"--name", containerName,
		containerImageName,
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run container: %v", err)
	}

}

func runDockerSystemVersion(t *testing.T, containerName string) string {
	t.Helper()

	cmd := exec.Command(
		"docker", "exec",
		"--user", "arduino",
		containerName,
		"arduino-app-cli", "version",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	return string(output)

}

func runDockerSystemUpdate(t *testing.T, containerName string) {
	t.Helper()
	var buf bytes.Buffer

	cmd := exec.Command(
		"docker", "exec",
		containerName,
		"sh", "-lc",
		`su - arduino -c "yes | arduino-app-cli system update"`,
	)

	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running system update: %v\n", err)
		os.Exit(1)
	}

}

func runDockerDaemon(t *testing.T, containerName string) string {
	t.Helper()

	cmd := exec.Command(
		"docker", "exec",
		"-d",
		"--user", "arduino",
		containerName,
		"systemctl", "start", "arduino-app-cli",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command failed: %v\n Output: %s", err, output)
	}

	return string(output)
}

func runDockerCleanUp(t *testing.T, containerName string) {
	t.Helper()

	cleanupCmd := exec.Command("docker", "rm", "-f", containerName)

	fmt.Println("🧹 Removing Docker container " + containerName)
	if err := cleanupCmd.Run(); err != nil {
		fmt.Printf("⚠️  Warning: could not remove container (might not exist): %v\n", err)
	}

}

func moveDeb(t *testing.T, startDir, targetDir, repo string, tagVersion string, arch string) {
	t.Helper()
	tagPath := strings.TrimPrefix(tagVersion, "v")

	debFile := fmt.Sprintf("%s/%s_%s-1_%s.deb", startDir, repo, tagPath, arch)

	moveCmd := exec.Command("cp", debFile, targetDir)

	fmt.Printf("📦 Moving %s → %s\n", debFile, targetDir)
	if err := moveCmd.Run(); err != nil {
		panic(fmt.Errorf("failed to move deb file: %w", err))
	}

	rm(t, debFile)
}

func ls(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting working directory:", err)
		return
	}

	fmt.Println("Current directory:", cwd)
	fmt.Println("Listing all files and folders recursively:")

	// Walk through all files and subdirectories
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	})

}

func rm(t *testing.T, pathFile string) {
	t.Helper()
	removeCmd := exec.Command("rm", pathFile)

	err := removeCmd.Run()
	if err != nil {
		log.Fatalf("Failed to remove file: %v", err)
	}

	fmt.Printf("📦 Removed %s\n", pathFile)

}

func putUpdateRequest(t *testing.T, url string) string {

	t.Helper()

	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	return resp.Status
}

func rmrf(t *testing.T, pathFile string) {
	t.Helper()
	// Check if the folder exists
	if _, err := os.Stat(pathFile); os.IsNotExist(err) {
		fmt.Println("No build directory found.")
		return
	}

	// Run the Linux command to remove it
	cmd := exec.Command("rm", "-rf", pathFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Removing build directory...")

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing build folder: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Build directory removed successfully.")
}

func NewSSEClient(ctx context.Context, method, url string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			_ = yield(Event{}, err)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			_ = yield(Event{}, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			_ = yield(Event{}, fmt.Errorf("got response status code %d", resp.StatusCode))
			return
		}

		reader := bufio.NewReader(resp.Body)

		evt := Event{}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				_ = yield(Event{}, err)
				return
			}
			switch {
			case strings.HasPrefix(line, "data:"):
				evt.Data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			case strings.HasPrefix(line, "event:"):
				evt.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "id:"):
				evt.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "\n"):
				if !yield(evt, nil) {
					return
				}
				evt = Event{}
			default:
				_ = yield(Event{}, fmt.Errorf("unknown line: '%s'", line))
				return
			}
		}
	}
}

type Event struct {
	ID    string
	Event string
	Data  []byte // json
}

// WaitForPort waits until a TCP port is open or fails after timeout.
func WaitForPort(t *testing.T, host string, port string, timeout time.Duration) {
	t.Helper()
	addr := fmt.Sprintf("%s:%s", host, port)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			t.Logf("Server is up on %s", addr)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Server at %s did not start within %v", addr, timeout)
}
