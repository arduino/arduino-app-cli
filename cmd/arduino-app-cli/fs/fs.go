package fs

import (
	"context"
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/pkg/appsync"
	"github.com/arduino/arduino-app-cli/pkg/board"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
)

const boardHomePath = "/home/arduino"

type contextKey string

const remoteConnKey contextKey = "remoteConn"

func NewFSCmd() *cobra.Command {
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
				feedback.Printf(" ⬆️ Pulled app %q to folder %q", name, path)
			}
			s.OnPush = func(name string) {
				feedback.Printf(" ⬇️ Pushed app %q to the board", name)
			}

			tmp, err := s.EnableSyncApp(remote)
			if err != nil {
				return fmt.Errorf("failed to enable sync for app %q: %w", remote, err)
			}

			feedback.Printf("Enable sync of %q at %q", remote, tmp)

			<-cmd.Context().Done()
			_ = s.DisableSyncApp(remote)
			return nil
		},
	}

	return syncAppCmd
}
