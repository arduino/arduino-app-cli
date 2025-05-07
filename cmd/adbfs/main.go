package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: goadbfs <command> [args]")
		return
	}
	cmd := os.Args[1]
	switch cmd {
	case "ls":
		path := "."
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		ls(path)
	case "push":
		src := os.Args[2]
		if src == "" {
			fmt.Println("Please provide a path to copy")
			return
		}
		dst := os.Args[3]
		push(dst, src)
	case "pull":
		src := os.Args[2]
		if src == "" {
			fmt.Println("Please provide a path to copy")
		}
		dst := os.Args[3]
		pull(dst, src)
	default:
		fmt.Println("Unknown command:", cmd)
	}
}

func ls(path string) {
	err := fs.WalkDir(adbfs.AdbFS{}, path, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			fmt.Println("Dir: ", path)
		} else {
			fmt.Println("File:", path)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err.Error())
	}
}

func pull(dst, src string) {
	if err := adbfs.SyncFS(adbfs.OsFSWriter{Base: dst}, adbfs.AdbFS{Base: src}); err != nil {
		fmt.Println("Error:", err.Error())
	}
}

func push(dst, src string) {
	if err := adbfs.SyncFS(adbfs.AdbFSWriter{AdbFS: adbfs.AdbFS{Base: dst}}, os.DirFS(src)); err != nil {
		fmt.Println("Error:", err.Error())
	}
}
