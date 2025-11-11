package testtools

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	fmt.Printf("Check folder structure and deb downloaded\n")
	ls(t)
	fmt.Println("**** BUILD docker image *****")
	buildDockerImage(t, "test.Dockerfile", "apt-test-update-image", *arch)
	fmt.Println("**** RUN docker image *****")
	runDockerCommand(t, "apt-test-update-image")
	preUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerSystemUpdate(t, "apt-test-update")
	postUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerCleanUp(t, "apt-test-update")
	require.Equal(t, preUpdateVersion, "Arduino App CLI "+tagAppCli+"\n")
	require.Equal(t, postUpdateVersion, "Arduino App CLI "+majorTag+"\n")

}

func TestUnstableToStable(t *testing.T) {
	tagAppCli := FetchDebPackage(t, "arduino-app-cli", "latest", *arch)
	FetchDebPackage(t, "arduino-router", "latest", *arch)
	minorTag := minorTag(t, tagAppCli)
	moveDeb(t, "build/stable", "build/", "arduino-app-cli", tagAppCli, *arch)

	fmt.Printf("Updating from unstable version %s to stable version %s \n", minorTag, tagAppCli)
	fmt.Printf("Building local deb version %s \n", minorTag)
	buildDebVersion(t, minorTag, *arch)
	moveDeb(t, "build/", "build/stable", "arduino-app-cli", tagAppCli, *arch)

	fmt.Println("**** BUILD docker image *****")
	buildDockerImage(t, "test.Dockerfile", "test-apt-update-unstable", *arch)
	fmt.Println("**** RUN docker image *****")
	runDockerCommand(t, "test-apt-update-unstable")
	preUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerSystemUpdate(t, "apt-test-update")
	postUpdateVersion := runDockerSystemVersion(t, "apt-test-update")
	runDockerCleanUp(t, "apt-test-update")
	require.Equal(t, preUpdateVersion, "Arduino App CLI "+tagAppCli+"\n")
	require.Equal(t, postUpdateVersion, "Arduino App CLI "+minorTag+"\n")

}

func FetchDebPackage(t *testing.T, repo, version, arch string) string {
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

	debFile := fmt.Sprintf("build/stable/%s_%s-1_%s.deb", repo, tagPath, arch)
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
		"--dir", "./build/stable",
	)

	out, err := cmd2.CombinedOutput()
	if err != nil {
		log.Fatalf("download failed: %v\nOutput: %s", err, out)
	}

	return tag

}

func buildDebVersion(t *testing.T, tagVersion, arch string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	outputDir := filepath.Join(cwd, "build")

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

func runDockerCommand(t *testing.T, containerImageName string) {
	t.Helper()

	cmd := exec.Command(
		"docker", "run", "--rm", "-d",
		"--privileged",
		"--cgroupns=host",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", "DOCKER_HOST=unix:///var/run/docker.sock",
		"--name", "apt-test-update",
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
