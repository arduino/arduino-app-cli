package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/adb"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
	"github.com/arduino/arduino-app-cli/pkg/appsync"
)

const boardAppPath = "/home/arduino/arduino-apps"

func main() {
	var rootCmd = &cobra.Command{
		Use:   "arduino-fs-cli",
		Short: "A CLI tool for interacting with arduino apps file system",
	}

	var fqbn, host string
	rootCmd.PersistentFlags().StringVarP(&fqbn, "fqbn", "b", "dev:zephyr:jomla", "fqbn of the board")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "ADB host address")

	getAdbConnection := func(ctx context.Context) (*adb.ADBConnection, error) {
		if host != "" {
			return adb.FromHost(host, "")
		}
		return adb.FromFQBN(ctx, fqbn, "")
	}

	var lsCmd = &cobra.Command{
		Use:   "ls [path]",
		Short: "List files in the specified path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := "."
			if len(args) > 0 {
				p = args[0]
			}
			adb, err := getAdbConnection(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to connect to board: %w", err)
			}
			files, err := adb.List(path.Join(boardAppPath, p))
			if err != nil {
				return fmt.Errorf("failed to list files: %w", err)
			}
			for _, file := range files {
				if file.IsDir {
					fmt.Println("📁 ", file.Name)
				} else {
					fmt.Println("📄 ", file.Name)
				}
			}
			return nil
		},
	}

	var treeCmd = &cobra.Command{
		Use:   "tree [path]",
		Short: "List files in the specified path (ignore hidden files)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := "."
			if len(args) > 0 {
				p = args[0]
			}

			adb, err := getAdbConnection(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to connect to board: %w", err)
			}

			err = fs.WalkDir(adbfs.NewAdbFS(boardAppPath, adb), p, func(p string, info fs.DirEntry, err error) error {
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
				return fmt.Errorf("failed to list files: %w", err)
			}
			return nil
		},
	}

	var pushCmd = &cobra.Command{
		Use:   "push <local> <remote>",
		Short: "Push a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			local := args[0]
			remote := path.Join(boardAppPath, args[1])

			adb, err := getAdbConnection(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to connect to board: %w", err)
			}

			if err := adbfs.SyncFS(adbfs.NewAdbFS(remote, adb).ToWriter(), os.DirFS(local)); err != nil {
				return fmt.Errorf("failed to push files: %w", err)
			}
			return nil
		},
	}

	var pullCmd = &cobra.Command{
		Use:   "pull <remote> <local>",
		Short: "Pull a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			remote := path.Join(boardAppPath, args[0])
			local := filepath.Join(args[1], path.Base(remote))

			adb, err := getAdbConnection(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to connect to board: %w", err)
			}

			if err := adbfs.SyncFS(adbfs.OsFSWriter{Base: local}, adbfs.NewAdbFS(remote, adb), ".cache"); err != nil {
				return fmt.Errorf("failed to pull files: %w", err)
			}
			return nil
		},
	}

	var syncAppCmd = &cobra.Command{
		Use:   "enable-sync <app-name>",
		Short: "Enable sync of an app from the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appName := args[0]

			adb, err := getAdbConnection(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to connect to board: %w", err)
			}

			s, err := appsync.New(adb, boardAppPath)
			if err != nil {
				return fmt.Errorf("failed to create apps sync: %w", err)
			}
			defer s.Close()
			s.OnPull = func(name, path string) {
				fmt.Printf(" ⬆️ Pulled app %q to folder %q\n", name, path)
			}
			s.OnPush = func(name string) {
				fmt.Printf(" ⬇️ Pushed app %q to the board\n", name)
			}

			tmp, err := s.EnableSyncApp(appName)
			if err != nil {
				return fmt.Errorf("failed to enable sync for app %q: %w", appName, err)
			}

			fmt.Printf("Enable sync of %q at %q\n", appName, tmp)

			<-cmd.Context().Done()
			_ = s.DisableSyncApp(appName)
			return nil
		},
	}

	var slogOptions *slog.HandlerOptions
	if os.Getenv("LOG_LEVEL") == "debug" {
		slogOptions = &slog.HandlerOptions{Level: slog.LevelDebug}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, slogOptions)))

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	rootCmd.AddCommand(lsCmd, treeCmd, pushCmd, pullCmd, syncAppCmd)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
