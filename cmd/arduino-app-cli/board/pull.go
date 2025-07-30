package board

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/appsync"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
)

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
