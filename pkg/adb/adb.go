package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"go.bug.st/f"
)

var adbPath = "adb"

func init() {
	// Attempt to find the adb path in the Arduino15 directory
	const arduino15adbPath = "packages/arduino/tools/adb/32.0.0/adb"
	var path string
	switch runtime.GOOS {
	case "darwin":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, "/Library/Arduino15/", arduino15adbPath)
	case "linux":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, ".arduino15/", arduino15adbPath)
	case "windows":
		user, err := user.Current()
		if err != nil {
			slog.Warn("Unable to get current user", "error", err)
			break
		}
		path = filepath.Join(user.HomeDir, "AppData/Local/Arduino15/", arduino15adbPath)
	}
	s, err := os.Stat(path)
	if err == nil && !s.IsDir() {
		adbPath = path
	}

	slog.Debug("get adb path", "path", adbPath)
}

type FileInfo struct {
	Name  string
	IsDir bool
}

func List(path string, host ...string) ([]FileInfo, error) {
	f.Assert(len(host) <= 1, "List: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "ls", "-la", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "ls", "-la", path)
	}

	cmd.Stderr = os.Stdout
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer output.Close()
	slog.Debug("adb List", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	r := bufio.NewReader(output)
	_, err = r.ReadBytes('\n') // Skip the first line
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		parts := bytes.Split(line, []byte(" "))
		name := string(parts[len(parts)-1])
		if name == "." || name == ".." {
			continue
		}
		files = append(files, FileInfo{
			Name:  name,
			IsDir: line[0] == 'd',
		})
	}

	return files, nil
}

func Stats(path string, host ...string) (FileInfo, error) {
	f.Assert(len(host) <= 1, "Stats: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "file", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "file", path)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return FileInfo{}, err
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		return FileInfo{}, err
	}

	r := bufio.NewReader(output)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return FileInfo{}, err
	}

	line = bytes.TrimSpace(line)
	parts := bytes.Split(line, []byte(":"))
	if len(parts) < 2 {
		return FileInfo{}, fmt.Errorf("unexpected file command output: %s", line)
	}

	name := string(bytes.TrimSpace(parts[0]))
	other := string(bytes.TrimSpace(parts[1]))

	if strings.Contains(other, "cannot open") {
		return FileInfo{}, fs.ErrNotExist
	}

	return FileInfo{
		Name:  name,
		IsDir: other == "directory",
	}, nil
}

func CatOut(path string, host ...string) (io.ReadCloser, error) {
	f.Assert(len(host) <= 1, "CatOut: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "cat", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "cat", path)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	slog.Debug("CatOut", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return output, nil
}

func CatIn(r io.Reader, path string, host ...string) error {
	f.Assert(len(host) <= 1, "CatIn: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "cat", ">", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "cat", ">", path)
	}
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	slog.Debug("adb CatIn", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

func MkDirAll(path string, host ...string) error {
	f.Assert(len(host) <= 1, "MkDirAll: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "mkdir", "-p", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "mkdir", "-p", path)
	}
	out, err := cmd.CombinedOutput()
	slog.Debug("adb MkDirAll", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

func Remove(path string, host ...string) error {
	f.Assert(len(host) <= 1, "Remove: single host only")
	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "shell", "rm", "-r", path) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "shell", "rm", "-r", path)
	}
	out, err := cmd.CombinedOutput()
	slog.Debug("adb Remove", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}

// Push folder from the local machine to the remote device.
func Push(localPath, remotePath string, host ...string) error {
	f.Assert(len(host) <= 1, "PushSync: single host only")

	remotePathDir := path.Dir(remotePath)

	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "push", "--sync", localPath, remotePathDir) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "push", "--sync", localPath, remotePathDir)
	}
	out, err := cmd.CombinedOutput()
	slog.Debug("adb PushSync", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}

	return nil
}

// Pull folder from the remote device to the local machine.
func Pull(remotePath, localPath string, host ...string) error {
	f.Assert(len(host) <= 1, "PushSync: single host only")

	localPath = filepath.Dir(localPath)

	var cmd *exec.Cmd
	if len(host) == 1 && host[0] != "" {
		cmd = exec.Command(adbPath, "-s", host[0], "pull", "--sync", remotePath, localPath) // nolint:gosec
	} else {
		cmd = exec.Command(adbPath, "pull", "--sync", remotePath, localPath)
	}
	out, err := cmd.CombinedOutput()
	slog.Debug("adb PullSync", "cmd", cmd.String(), "out", string(out))
	if err != nil {
		return err
	}
	return nil
}
