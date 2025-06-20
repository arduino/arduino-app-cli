package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/adb"
	"github.com/arduino/arduino-app-cli/pkg/adbfs"
	"github.com/arduino/arduino-app-cli/pkg/appsync"
)

const boardHomePath = "/home/arduino"

type contextKey string

const adbConnectionKey contextKey = "adbConnectionKey"

func newFSCmd() *cobra.Command {
	var fqbn, host string
	fsCmd := &cobra.Command{
		Use:   "fs",
		Short: "Manage board fs",
		Long:  "",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var conn *adb.ADBConnection
			var err error
			if host != "" {
				conn, err = adb.FromHost(host, "")
				if err != nil {
					return fmt.Errorf("failed to connect to ADB host %s: %w", host, err)
				}
			} else {
				conn, err = adb.FromFQBN(cmd.Context(), fqbn, "")
				if err != nil {
					return fmt.Errorf("failed to connect to board with fqbn %s: %w", fqbn, err)
				}
			}
			cmd.SetContext(context.WithValue(cmd.Context(), adbConnectionKey, conn))
			return nil
		},
	}
	fsCmd.PersistentFlags().StringVarP(&fqbn, "fqbn", "b", "dev:zephyr:jomla", "fqbn of the board")
	fsCmd.PersistentFlags().StringVar(&host, "host", "", "ADB host address")

	fsCmd.AddCommand(newPushCmd())
	fsCmd.AddCommand(newPullCmd())
	fsCmd.AddCommand(newSyncAppCmd())

	return fsCmd
}

func newPushCmd() *cobra.Command {
	pushCmd := &cobra.Command{
		Use:   "push <local> <remote>",
		Short: "Push and sync a directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(adbConnectionKey).(*adb.ADBConnection)

			local, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("failed to get absolute path of local file: %w", err)
			}
			remote := path.Join(boardHomePath, args[1])

			if err := adbfs.SyncFS(adbfs.NewAdbFS(remote, conn).ToWriter(), os.DirFS(local), ".cache"); err != nil {
				return fmt.Errorf("failed to push files: %w", err)
			}
			return nil
		},
	}

	return pushCmd
}

func newPullCmd() *cobra.Command {
	pullCmd := &cobra.Command{
		Use:   "pull <remote> <local>",
		Short: "Pull and sync a directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(adbConnectionKey).(*adb.ADBConnection)

			remote := path.Join(boardHomePath, args[0])
			local, err := filepath.Abs(args[1])
			if err != nil {
				return fmt.Errorf("failed to get absolute path of local file: %w", err)
			}

			if err := adbfs.SyncFS(adbfs.OsFSWriter{Base: local}, adbfs.NewAdbFS(remote, conn), ".cache"); err != nil {
				return fmt.Errorf("failed to pull files: %w", err)
			}
			return nil
		},
	}
	return pullCmd
}

func newSyncAppCmd() *cobra.Command {
	syncAppCmd := &cobra.Command{
		Use:   "enable-sync <app-name>",
		Short: "Enable sync of an app from the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(adbConnectionKey).(*adb.ADBConnection)

			appName := args[0]

			s, err := appsync.New(conn, path.Join(boardHomePath, "arduino-apps"))
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

	return syncAppCmd
}
