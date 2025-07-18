package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/appsync"
	"github.com/arduino/arduino-app-cli/pkg/board"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
)

const boardHomePath = "/home/arduino"

type contextKey string

const remoteConnKey contextKey = "remoteConn"

func newFSCmd() *cobra.Command {
	var fqbn, host string
	fsCmd := &cobra.Command{
		Use:   "fs",
		Short: "Manage board fs",
		Long:  "",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if host != "" {
				conn, err := adb.FromHost(host, "")
				if err != nil {
					panic(fmt.Errorf("failed to connect to ADB host %s: %w", host, err))
				}
				cmd.SetContext(context.WithValue(cmd.Context(), remoteConnKey, conn))
				return
			}

			boards, err := board.FromFQBN(cmd.Context(), fqbn)
			if err != nil {
				panic(err)
			}
			if len(boards) == 0 {
				panic(fmt.Errorf("no boards found for FQBN %s", fqbn))
			}
			conn, err := boards[0].Connect()
			if err != nil {
				panic(fmt.Errorf("failed to connect to board: %w", err))
			}

			cmd.SetContext(context.WithValue(cmd.Context(), remoteConnKey, conn))

		},
	}
	fsCmd.PersistentFlags().StringVarP(&fqbn, "fqbn", "b", "arduino:zephyr:unoq", "fqbn of the board")
	fsCmd.PersistentFlags().StringVar(&host, "host", "", "ADB host address")

	fsCmd.AddCommand(newPushCmd())
	fsCmd.AddCommand(newPullCmd())
	fsCmd.AddCommand(newSyncAppCmd())

	return fsCmd
}

func newPushCmd() *cobra.Command {
	pushCmd := &cobra.Command{
		Use:   "push <local> <remote>",
		Short: "Push a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			local, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("failed to get absolute path of local file: %w", err)
			}
			remote := path.Join(boardHomePath, args[1])

			if err := appsync.SyncFS(remotefs.New(remote, conn).ToWriter(), os.DirFS(local), ".cache"); err != nil {
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
		Short: "Pull a file or directory from the local machine to the board",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			remote := path.Join(boardHomePath, args[0])
			local, err := filepath.Abs(args[1])
			if err != nil {
				return fmt.Errorf("failed to get absolute path of local file: %w", err)
			}

			if err := appsync.SyncFS(appsync.OsFSWriter{Base: local}, remotefs.New(remote, conn), ".cache"); err != nil {
				return fmt.Errorf("failed to pull files: %w", err)
			}
			return nil
		},
	}
	return pullCmd
}

func newSyncAppCmd() *cobra.Command {
	syncAppCmd := &cobra.Command{
		Use:   "enable-sync <path>",
		Short: "Enable sync of an path from the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			remote := path.Join(boardHomePath, args[0])

			s, err := appsync.New(conn)
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

			tmp, err := s.EnableSyncApp(remote)
			if err != nil {
				return fmt.Errorf("failed to enable sync for app %q: %w", remote, err)
			}

			fmt.Printf("Enable sync of %q at %q\n", remote, tmp)

			<-cmd.Context().Done()
			_ = s.DisableSyncApp(remote)
			return nil
		},
	}

	return syncAppCmd
}
