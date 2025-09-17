package board

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if host != "" {
				conn, err := adb.FromHost(host, "")
				if err != nil {
					panic(fmt.Errorf("failed to connect to ADB host %s: %w", host, err))
				}
				cmd.SetContext(context.WithValue(cmd.Context(), remoteConnKey, conn))
				return nil
			}

			boards, err := board.FromFQBN(cmd.Context(), fqbn)
			if err != nil {
				return fmt.Errorf("failed to get boards for FQBN %s: %w", fqbn, err)
			}
			if len(boards) == 0 {
				return fmt.Errorf("no boards found for FQBN %s", fqbn)
			}
			conn, err := boards[0].GetConnection()
			if err != nil {
				return fmt.Errorf("failed to connect to board %s: %w", boards[0].BoardName, err)
			}

			cmd.SetContext(context.WithValue(cmd.Context(), remoteConnKey, conn))
			cmd.SetContext(context.WithValue(cmd.Context(), boardsListKey, boards))
			return nil
		},
	}
	fsCmd.PersistentFlags().StringVarP(&fqbn, "fqbn", "b", "arduino:zephyr:unoq", "fqbn of the board")
	fsCmd.PersistentFlags().StringVar(&host, "host", "", "ADB host address")

	fsCmd.AddCommand(newPushCmd())
	fsCmd.AddCommand(newPullCmd())
	fsCmd.AddCommand(newSyncAppCmd())
	fsCmd.AddCommand(newBoardListCmd())
	fsCmd.AddCommand(newBoardSetName())
	fsCmd.AddCommand(newSetPasswordCmd())
	fsCmd.AddCommand(newEnableNetworkModeCmd())
	fsCmd.AddCommand(newDisableNetworkModeCmd())
	fsCmd.AddCommand(newNetworkModeStatusCmd())

	fsCmd.AddCommand(listKeyboardLayouts())
	fsCmd.AddCommand(getKeyboardLayout())
	fsCmd.AddCommand(setKeyboardLayout())

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

				var address, configured string
				switch b.Protocol {
				case board.SerialProtocol, board.LocalProtocol:
					address = b.Serial

					if conn, err := b.GetConnection(); err != nil {
						return fmt.Errorf("failed to connect to board %s: %w", b.BoardName, err)
					} else {
						if s, err := board.IsUserPasswordSet(conn); err != nil {
							return fmt.Errorf("failed to check if user password is set: %w", err)
						} else {
							configured = "- Configured: " + strconv.FormatBool(s)
						}
					}
				case board.NetworkProtocol:
					address = b.Address
				default:
					panic("unreachable")
				}

				feedback.Printf("%s (%s) - Connection: %s [%s] %s\n", b.BoardName, b.CustomName, b.Protocol, address, configured)
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

func newSetPasswordCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-password",
		Short: "Set the user password of the board",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			feedback.Print("Enter new password: ")
			// TODO: fix for not interactive terminal
			password, err := term.ReadPassword(int(os.Stdin.Fd())) // nolint:forbidigo
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			if err := board.SetUserPassword(cmd.Context(), conn, string(password)); err != nil {
				return fmt.Errorf("failed to set user password: %w", err)
			}

			feedback.Printf("User password set\n")
			return nil
		},
	}
}

func newEnableNetworkModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable-ssh",
		Short: "Enable and start the SSH service on the board",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			if err := board.EnableNetworkMode(cmd.Context(), conn); err != nil {
				return fmt.Errorf("failed to enable SSH: %w", err)
			}

			feedback.Printf("SSH service enabled and started\n")
			return nil
		},
	}
}

func newDisableNetworkModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable-ssh",
		Short: "Disable and stop the SSH service on the board",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			if err := board.DisableNetworkMode(cmd.Context(), conn); err != nil {
				return fmt.Errorf("failed to disable SSH: %w", err)
			}

			feedback.Printf("SSH service disabled and stopped\n")
			return nil
		},
	}
}

func newNetworkModeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status-ssh",
		Short: "Check the status of the network mode on the board",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			isEnabled, err := board.NetworkModeStatus(cmd.Context(), conn)
			if err != nil {
				return fmt.Errorf("failed to check network mode status: %w", err)
			}

			feedback.Printf("Network mode is %s\n", map[bool]string{true: "enabled", false: "disabled"}[isEnabled])
			return nil
		},
	}
}

func getKeyboardLayout() *cobra.Command {
	return &cobra.Command{
		Use:   "get-keyboard-layout",
		Short: "Returns the current system keyboard layout code",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			layoutCode, err := board.GetKeyboardLayout(cmd.Context(), conn)
			if err != nil {
				return fmt.Errorf("failed: %w", err)
			}
			feedback.Printf("Layout: %s", layoutCode)

			return nil
		},
	}
}

func setKeyboardLayout() *cobra.Command {
	return &cobra.Command{
		Use:   "set-keyboard-layout <layout>",
		Short: "Saves and applies the current system keyboard layout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)
			layoutCode := args[0]

			err := validateKeyboardLayoutCode(conn, layoutCode)
			if err != nil {
				return fmt.Errorf("failed: %w", err)
			}

			err = board.SetKeyboardLayout(cmd.Context(), conn, layoutCode)
			if err != nil {
				return fmt.Errorf("failed: %w", err)
			}

			feedback.Printf("New layout applied: %s", layoutCode)
			return nil
		},
	}
}

func validateKeyboardLayoutCode(conn remote.RemoteConn, layoutCode string) error {
	// Make sure the input layout code is in the list of valid ones
	layouts, err := board.ListKeyboardLayouts(conn)
	if err != nil {
		return fmt.Errorf("failed to fetch valid layouts: %w", err)
	}

	for _, layout := range layouts {
		if layout.LayoutId == layoutCode {
			return nil
		}
	}

	return fmt.Errorf("invalid layout code: %s", layoutCode)
}

func listKeyboardLayouts() *cobra.Command {
	return &cobra.Command{
		Use:   "list-keyboard-layouts",
		Short: "Returns the list of valid keyboard layouts, with a description",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := cmd.Context().Value(remoteConnKey).(remote.RemoteConn)

			layouts, err := board.ListKeyboardLayouts(conn)
			if err != nil {
				return fmt.Errorf("failed: %w", err)
			}

			for _, layout := range layouts {
				feedback.Printf("%s, %s", layout.LayoutId, layout.Description)
			}

			return nil
		},
	}
}
