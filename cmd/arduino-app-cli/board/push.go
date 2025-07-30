package board

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/pkg/appsync"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
)

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
