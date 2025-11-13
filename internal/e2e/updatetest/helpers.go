package updatetest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/stretchr/testify/require"
)

func fetchDebPackageLatest(t *testing.T, path, repo string) string {
	t.Helper()

	repo = fmt.Sprintf("github.com/arduino/%s", repo)
	cmd := exec.Command(
		"gh", "release", "list",
		"--repo", repo,
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

	fmt.Println("Detected tag:", tag)
	cmd2 := exec.Command(
		"gh", "release", "download",
		tag,
		"--repo", repo,
		"--pattern", "*.deb",
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

	tagVersion = fmt.Sprintf("VERSION=%s", tagVersion)
	arch = fmt.Sprintf("ARCH=%s", arch)
	outputDir = fmt.Sprintf("OUTPUT=%s", outputDir)

	cmd := exec.Command(
		"go", "tool", "task", "build-deb",
		tagVersion,
		arch,
		outputDir,
	)

	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run build command: %v", err)
	}
}

func genMajorTag(t *testing.T, tag string) string {
	t.Helper()

	parts := strings.Split(tag, ".")
	last := parts[len(parts)-1]

	lastNum, _ := strconv.Atoi(strings.TrimPrefix(last, "v"))
	lastNum++

	parts[len(parts)-1] = strconv.Itoa(lastNum)
	newTag := strings.Join(parts, ".")

	return newTag
}

func genMinorTag(t *testing.T, tag string) string {
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

	arch = fmt.Sprintf("ARCH=%s", arch)

	cmd := exec.Command("docker", "build", "--build-arg", arch, "-t", name, "-f", dockerfile, ".")
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
}

func startDockerContainer(t *testing.T, containerName string, containerImageName string) {
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

func getAppCliVersion(t *testing.T, containerName string) string {
	t.Helper()

	cmd := exec.Command(
		"docker", "exec",
		"--user", "arduino",
		containerName,
		"arduino-app-cli", "version", "--format", "json",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	var version struct {
		Version       string `json:"version"`
		DaemonVersion string `json:"daemon_version"`
	}
	err = json.Unmarshal(output, &version)
	require.NoError(t, err)
	// TODO to enable after 0.6.7
	// require.Equal(t, version.Version, version.DaemonVersion, "client and daemon versions should match")
	require.NotEmpty(t, version.Version)
	return version.Version

}

func runSystemUpdate(t *testing.T, containerName string) {
	t.Helper()

	cmd := exec.Command(
		"docker", "exec",
		"--user", "arduino",
		containerName,
		"arduino-app-cli", "system", "update", "--yes",
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "system update failed: %s", output)
	t.Logf("system update output: %s", output)
}

func stopDockerContainer(t *testing.T, containerName string) {
	t.Helper()

	cleanupCmd := exec.Command("docker", "rm", "-f", containerName)

	fmt.Println("🧹 Removing Docker container " + containerName)
	if err := cleanupCmd.Run(); err != nil {
		fmt.Printf("⚠️  Warning: could not remove container (might not exist): %v\n", err)
	}

}

func putUpdateRequest(t *testing.T, host string) {

	t.Helper()

	url := fmt.Sprintf("http://%s/v1/system/update/apply", host)

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

	require.Equal(t, 202, resp.StatusCode)

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

func waitForPort(t *testing.T, host string, timeout time.Duration) { // nolint:unparam
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			t.Logf("Server is up on %s", host)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Server at %s did not start within %v", host, timeout)
}

func waitForUpgrade(t *testing.T, host string) {
	t.Helper()

	url := fmt.Sprintf("http://%s/v1/system/update/events", host)

	itr := NewSSEClient(t.Context(), "GET", url)
	for event, err := range itr {
		require.NoError(t, err)
		t.Logf("Received event: ID=%s, Event=%s, Data=%s\n", event.ID, event.Event, string(event.Data))
		if event.Event == "restarting" {
			break
		}
	}

}
