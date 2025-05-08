package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/adb"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
)

const boardAppPath = "/apps"

func main() {
	var rootCmd = &cobra.Command{
		Use:   "arduino-fs-cli",
		Short: "A CLI tool for interacting with arduino apps file system",
	}

	var lsCmd = &cobra.Command{
		Use:   "ls [path]",
		Short: "List files in the specified path",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			p := "."
			if len(args) > 0 {
				p = args[0]
			}
			files, err := adb.List(path.Join(boardAppPath, p))
			if err != nil {
				fmt.Println("Error:", err.Error())
				return
			}
			for _, file := range files {
				if file.IsDir {
					fmt.Println("📁 ", file.Name)
				} else {
					fmt.Println("📄 ", file.Name)
				}
			}
		},
	}

	var treeCmd = &cobra.Command{
		Use:   "tree [path]",
		Short: "List files in the specified path (ignore hidden files)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			p := "."
			if len(args) > 0 {
				p = args[0]
			}

			err := fs.WalkDir(adbfs.AdbFS{Base: boardAppPath}, p, func(p string, info fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Ignore hidden files and directories
				base := path.Base(p)
				if strings.HasPrefix(base, ".") && len(base) > 1 {
					return fs.SkipDir
				}

				if info.IsDir() {
					fmt.Println("📁 ", p)
				} else {
					fmt.Println("📄 ", p)
				}
				return nil
			})
			if err != nil {
				fmt.Println("Error:", err.Error())
			}
		},
	}

	var pushCmd = &cobra.Command{
		Use:   "push <local> <remote>",
		Short: "Push a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			local := args[0]
			remote := path.Join(boardAppPath, args[1])
			if err := adb.PushSync(local, remote); err != nil {
				fmt.Println("Error:", err.Error())
			}
		},
	}

	var pullCmd = &cobra.Command{
		Use:   "pull <remote> <local>",
		Short: "Pull a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			remote := path.Join(boardAppPath, args[0])
			local := filepath.Join(args[1], path.Base(remote))
			if err := adb.PullSync(remote, local, []string{".cache"}); err != nil {
				fmt.Println("Error:", err.Error())
			}
		},
	}

	rootCmd.AddCommand(lsCmd, treeCmd, pushCmd, pullCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
