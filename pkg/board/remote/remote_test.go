package remote_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
	"github.com/arduino/arduino-app-cli/pkg/x/testtools"
)

func TestRemoteFS(t *testing.T) {
	name, adbPort, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	remotes := []remote.RemoteFs{
		func() remote.RemoteFs {
			conn, err := adb.FromHost("localhost:"+adbPort, "")
			require.NoError(t, err)
			return conn
		}(),
		func() remote.RemoteFs {
			conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
			require.NoError(t, err)
			return conn
		}(),
	}

	for _, conn := range remotes {
		t.Run("Mkdir", func(t *testing.T) {
			err := conn.MkDirAll("./testdir")
			require.NoError(t, err)
			info, err := conn.Stats("./testdir")
			require.NoError(t, err)
			assert.Equal(t, info, remote.FileInfo{
				Name:  "./testdir",
				IsDir: true,
			})
		})

		t.Run("WriteFile/ReadFile", func(t *testing.T) {
			err := conn.WriteFile(strings.NewReader("Hello, World!"), "./testdir/testfile.txt")
			require.NoError(t, err)
			info, err := conn.Stats("./testdir/testfile.txt")
			require.NoError(t, err)
			assert.Equal(t, info, remote.FileInfo{
				Name:  "./testdir/testfile.txt",
				IsDir: false,
			})

			r, err := conn.ReadFile("./testdir/testfile.txt")
			require.NoError(t, err)
			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "Hello, World!", string(data))
		})

		t.Run("List", func(t *testing.T) {
			files, err := conn.List("./")
			require.NoError(t, err)
			assert.NotEmpty(t, files)
			assert.Contains(t, files, remote.FileInfo{Name: "testdir", IsDir: true})

			files, err = conn.List("./testdir")
			require.NoError(t, err)
			assert.Len(t, files, 1)
			assert.Equal(t, remote.FileInfo{Name: "testfile.txt", IsDir: false}, files[0])
		})

		t.Run("Remove", func(t *testing.T) {
			err := conn.Remove("./testdir/testfile.txt")
			require.NoError(t, err)
			_, err = conn.Stats("./testdir/testfile.txt")
			assert.Error(t, err)

			err = conn.Remove("./testdir")
			require.NoError(t, err)
			_, err = conn.Stats("./testdir")
			assert.Error(t, err)
		})
	}
}

func TestSSHShell(t *testing.T) {
	name, adbPort, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	remotes := []remote.RemoteShell{
		func() remote.RemoteShell {
			conn, err := adb.FromHost("localhost:"+adbPort, "")
			require.NoError(t, err)
			return conn
		}(),
		func() remote.RemoteShell {
			conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
			require.NoError(t, err)
			return conn
		}(),
	}

	for _, conn := range remotes {
		tests := []func(string, ...string) remote.Cmder{
			func(cmd string, args ...string) remote.Cmder {
				return conn.GetCmd(t.Context(), cmd, args...)
			},
		}

		for _, cmder := range tests {
			t.Run("Run", func(t *testing.T) {
				cmd := cmder("echo", "Hello, World!")
				err := cmd.Run()
				require.NoError(t, err)
			})

			t.Run("Output", func(t *testing.T) {
				cmd := cmder("echo", "Hello, World!")
				output, err := cmd.Output()
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(string(output), "Hello, World!"))
			})

			t.Run("Interactive", func(t *testing.T) {
				cmd := cmder("cat")
				stdin, stdout, closer, err := cmd.Interactive()
				require.NoError(t, err)

				_, err = stdin.Write([]byte("Hello, Interactive World!\n"))
				require.NoError(t, err)
				stdin.Close() // Close stdin to signal EOF

				output, err := io.ReadAll(stdout)
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(string(output), "Hello, Interactive World!"))

				require.NoError(t, closer())
			})
		}
	}
}
