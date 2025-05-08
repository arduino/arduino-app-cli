package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
		adbPath = path
	}

	fmt.Printf("DEBUG: use adb at %q\n", adbPath)
}

type FileInfo struct {
	Name  string
	IsDir bool
}

func List(path string) ([]FileInfo, error) {
	cmd := exec.Command(adbPath, "shell", "ls", "-la", path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer output.Close()
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

func Stats(path string) (FileInfo, error) {
	cmd := exec.Command(adbPath, "shell", "file", path)
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

func CatOut(path string) (io.ReadCloser, error) {
	cmd := exec.Command(adbPath, "shell", "cat", path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	fmt.Printf("DEBUG: CatIn %q: ...\n", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return output, nil
}

func CatIn(r io.Reader, path string) error {
	cmd := exec.Command(adbPath, "shell", "cat", ">", path)
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: CatIn %q: %s\n", cmd.String(), string(out))
	if err != nil {
		return err
	}
	return nil
}

func MkDirAll(path string) error {
	cmd := exec.Command(adbPath, "shell", "mkdir", "-p", path)
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: MkDirAll %q: %s\n", cmd.String(), string(out))
	if err != nil {
		return err
	}
	return nil
}

func Remove(path string) error {
	cmd := exec.Command(adbPath, "shell", "rm", "-r", path)
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: Remove %q: %s\n", cmd.String(), string(out))
	if err != nil {
		return err
	}
	return nil
}

func PushSync(localPath, remotePath string) error {
	cmd := exec.Command(adbPath, "push", "--sync", localPath, remotePath)
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: PushSync %q: %s\n", cmd.String(), string(out))
	if err != nil {
		return err
	}
	return nil
}

// PullSync pulls files from the remote path to the local path, ignoring any top-level directories that match the ignorePath.
// TODO: probably we should make this smarted an be able to ignore any subdir.
func PullSync(remotePath, localPath string, ignorePath []string) error {
	files, err := List(remotePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(localPath, 0755); err != nil {
		return err
	}

	for _, file := range files {
		if idx := slices.IndexFunc(ignorePath, func(ignore string) bool {
			return path.Base(ignore) == path.Base(file.Name)
		}); idx != -1 {
			continue
		}

		if err := pullSync(
			path.Join(remotePath, file.Name),
			filepath.Join(localPath, file.Name),
		); err != nil {
			return err
		}
	}
	return nil
}

func pullSync(remotePath, localPath string) error {
	cmd := exec.Command(adbPath, "pull", "--sync", remotePath, localPath)
	out, err := cmd.CombinedOutput()
	fmt.Printf("DEBUG: pullSync %q: %s\n", cmd.String(), string(out))
	if err != nil {
		return err
	}
	return nil
}
