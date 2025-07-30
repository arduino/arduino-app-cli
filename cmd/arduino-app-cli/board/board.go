package board

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
const boardsListKey contextKey = "boardsList"

func NewBoardCmd() *cobra.Command {
	var fqbn, host string
	fsCmd := &cobra.Command{
		Use:   "board",
		Short: "Manage boards",
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
			conn, err := boards[0].GetConnection()
			if err != nil {
				panic(fmt.Errorf("failed to connect to board: %w", err))
			}

			cmd.SetContext(context.WithValue(cmd.Context(), remoteConnKey, conn))
			cmd.SetContext(context.WithValue(cmd.Context(), boardsListKey, boards))
		},
	}
	fsCmd.PersistentFlags().StringVarP(&fqbn, "fqbn", "b", "arduino:zephyr:unoq", "fqbn of the board")
	fsCmd.PersistentFlags().StringVar(&host, "host", "", "ADB host address")

	fsCmd.AddCommand(newPushCmd())
	fsCmd.AddCommand(newPullCmd())
	fsCmd.AddCommand(newSyncAppCmd())
	fsCmd.AddCommand(newBoardListCmd())
	fsCmd.AddCommand(newBoardSetName())

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

func newBoardListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available boards",
		RunE: func(cmd *cobra.Command, args []string) error {
			boards := cmd.Context().Value(boardsListKey).([]board.Board)
			for _, b := range boards {
				var address string
				switch b.Protocol {
				case board.SerialProtocol:
					address = b.Serial
				case board.NetworkProtocol:
					address = b.Address
				default:
					panic("unreachable")
				}
				feedback.Printf("%s (%s) - Connection: %s [%s]\n", b.BoardName, b.CustomName, b.Protocol, address)
			}
			return nil
		},
	}

	return listCmd
}

func newBoardSetName() *cobra.Command {
	setNameCmd := &cobra.Command{
		Use:   "set-name <name>",
		Short: "Set the custom name of the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)
			name := args[0]

			if err := board.SetCustomName(cmd.Context(), conn, name); err != nil {
				return fmt.Errorf("failed to set custom name: %w", err)
			}
			feedback.Printf("Custom name set to %q\n", name)
			return nil
		},
	}

	return setNameCmd
}
