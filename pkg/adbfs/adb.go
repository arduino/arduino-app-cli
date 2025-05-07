package adbfs

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
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

type fileInfo struct {
	name  string
	isDir bool
}

func adbList(path string) ([]fileInfo, error) {
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

	var files []fileInfo
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
		files = append(files, fileInfo{
			name:  name,
			isDir: line[0] == 'd',
		})
	}

	return files, nil
}

func adbStats(path string) (fileInfo, error) {
	cmd := exec.Command(adbPath, "shell", "file", path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return fileInfo{}, err
	}
	defer output.Close()
	if err := cmd.Start(); err != nil {
		return fileInfo{}, err
	}

	r := bufio.NewReader(output)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return fileInfo{}, err
	}

	line = bytes.TrimSpace(line)
	parts := bytes.Split(line, []byte(":"))
	if len(parts) < 2 {
		return fileInfo{}, fmt.Errorf("unexpected file command output: %s", line)
	}

	name := string(bytes.TrimSpace(parts[0]))
	other := string(bytes.TrimSpace(parts[1]))

	if strings.Contains(other, "cannot open") {
		return fileInfo{}, fs.ErrNotExist
	}

	return fileInfo{
		name:  name,
		isDir: other == "directory",
	}, nil
}

func adbCatOut(path string) (io.ReadCloser, error) {
	cmd := exec.Command(adbPath, "shell", "cat", path)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return output, nil
}

func adbCatIn(r io.Reader, path string) error {
	cmd := exec.Command(adbPath, "shell", "cat", ">", path)
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	fmt.Printf("DEBUG: adbCatIn %q: %s\n", path, string(out))
	return nil
}

func adbMkDirAll(path string) error {
	cmd := exec.Command(adbPath, "shell", "mkdir", "-p", path)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	fmt.Printf("DEBUG: adbMkDirAll %q\n", path)
	return nil
}

func adbRm(path string) error {
	cmd := exec.Command(adbPath, "shell", "rm", "-r", path)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}
