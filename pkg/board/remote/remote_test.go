package remote_test

import (
	"context"
	"fmt"

	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/testtools"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
	"github.com/arduino/arduino-app-cli/pkg/x/ports"
)

func TestRemoteFS(t *testing.T) {
	name, adbPort, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	tests := []struct {
		name string
		conn remote.FS
	}{{
		"adb",
		func() remote.FS {
			conn, err := adb.FromHost("localhost:"+adbPort, "")
			require.NoError(t, err)
			return conn
		}(),
	}, {
		"ssh",
		func() remote.FS {
			conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
			require.NoError(t, err)
			return conn
		}(),
	}, {
		"local",
		func() remote.FS {
			return &local.LocalConnection{}
		}(),
	},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Mkdir", func(t *testing.T) {
				err := tc.conn.MkDirAll("./testdir")
				require.NoError(t, err)
				info, err := tc.conn.Stats("./testdir")
				require.NoError(t, err)
				assert.Equal(t, remote.FileInfo{
					Name:  "testdir",
					IsDir: true,
				}, info)
			})

			t.Run("WriteFile/ReadFile", func(t *testing.T) {
				err := tc.conn.WriteFile(strings.NewReader("Hello, World!"), "./testdir/testfile.txt")
				require.NoError(t, err)
				info, err := tc.conn.Stats("./testdir/testfile.txt")
				require.NoError(t, err)
				assert.Equal(t, remote.FileInfo{
					Name:  "testfile.txt",
					IsDir: false,
				}, info)

				r, err := tc.conn.ReadFile("./testdir/testfile.txt")
				require.NoError(t, err)
				data, err := io.ReadAll(r)
				require.NoError(t, err)
				require.Equal(t, "Hello, World!", string(data))
			})

			t.Run("List", func(t *testing.T) {
				files, err := tc.conn.List("./")
				require.NoError(t, err)
				assert.NotEmpty(t, files)
				assert.Contains(t, files, remote.FileInfo{Name: "testdir", IsDir: true})

				files, err = tc.conn.List("./testdir")
				require.NoError(t, err)
				assert.Len(t, files, 1)
				assert.Equal(t, remote.FileInfo{Name: "testfile.txt", IsDir: false}, files[0])
			})

			t.Run("Remove", func(t *testing.T) {
				err := tc.conn.Remove("./testdir/testfile.txt")
				require.NoError(t, err)
				_, err = tc.conn.Stats("./testdir/testfile.txt")
				assert.Error(t, err)

				err = tc.conn.Remove("./testdir")
				require.NoError(t, err)
				_, err = tc.conn.Stats("./testdir")
				assert.Error(t, err)
			})
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
		func() remote.RemoteShell {
			return &local.LocalConnection{}
		}(),
	}

	for _, conn := range remotes {
		tests := []func(string, ...string) remote.Cmder{
			func(cmd string, args ...string) remote.Cmder {
				return conn.GetCmd(cmd, args...)
			},
		}

		for _, cmder := range tests {
			t.Run("Run", func(t *testing.T) {
				cmd := cmder("echo", "Hello, World!")
				err := cmd.Run(t.Context())
				require.NoError(t, err)
			})

			t.Run("Output", func(t *testing.T) {
				cmd := cmder("echo", "Hello, World!")
				output, err := cmd.Output(t.Context())
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(string(output), "Hello, World!"))
			})

			t.Run("Interactive", func(t *testing.T) {
				cmd := cmder("cat")
				stdin, stdout, stderr, closer, err := cmd.Interactive()
				require.NoError(t, err)

				_, err = stdin.Write([]byte("Hello, Interactive World!\n"))
				require.NoError(t, err)
				stdin.Close() // Close stdin to signal EOF

				output, err := io.ReadAll(stdout)
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(string(output), "Hello, Interactive World!"))
				stderrOutput, err := io.ReadAll(stderr)
				require.NoError(t, err)
				require.Empty(t, stderrOutput)

				require.NoError(t, closer())
			})
		}
	}

}

func TestSSHForwarder(t *testing.T) {
	name, _, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	conn, err := ssh.FromHost("arduino", "arduino", fmt.Sprintf("%s:%s", "localhost", sshPort))
	require.NoError(t, err)

	t.Run("Forward ADB", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		forwardPort, err := ports.GetAvailable()
		require.NoError(t, err)

		err = conn.Forward(ctx, forwardPort, 5555)
		if err != nil {
			t.Errorf("Forward failed: %v", err)
		}
		if forwardPort <= 0 || forwardPort > 65535 {
			t.Fatalf("invalid port: %d", forwardPort)
		}
		adb_forwarded_endpoint := fmt.Sprintf("localhost:%s", strconv.Itoa(forwardPort))

		out, err := exec.Command("adb", "connect", adb_forwarded_endpoint).CombinedOutput()
		require.NoError(t, err, "adb connect output: %q", out)

		cmd := exec.Command("adb", "-s", adb_forwarded_endpoint, "shell", "echo", "Hello, World!")
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "command output: %q", out)
		feedback.Printf("Command output:\n%s\n", string(out))
		require.NotNil(t, string(out))
	})
}

func TestSSHKillForwarder(t *testing.T) {
	name, _, sshPort := testtools.StartAdbDContainer(t)
	t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })

	conn, err := ssh.FromHost("arduino", "arduino", fmt.Sprintf("%s:%s", "localhost", sshPort))
	require.NoError(t, err)

	t.Run("KillAllForwards", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		forwardPort, err := ports.GetAvailable()
		require.NoError(t, err)

		err = conn.Forward(ctx, forwardPort, 5555)
		if err != nil {
			t.Errorf("Forward failed: %v", err)
		}
		if forwardPort <= 0 || forwardPort > 65535 {
			t.Fatalf("invalid port: %d", forwardPort)
		}
		adb_forwarded_endpoint := fmt.Sprintf("localhost:%s", strconv.Itoa(forwardPort))

		out, err := exec.Command("adb", "connect", adb_forwarded_endpoint).CombinedOutput()
		require.NoError(t, err, "adb connect output: %q", out)

		cmd := exec.Command("adb", "-s", adb_forwarded_endpoint, "shell", "echo", "Hello, World!")
		out, err = cmd.CombinedOutput()
		require.NoError(t, err, "command output: %q", out)
		feedback.Printf("Command output:\n%s\n", string(out))
		require.NotNil(t, string(out))

		err = conn.ForwardKillAll(t.Context())
		require.NoError(t, err)
		out, err = exec.Command("adb", "disconnect", adb_forwarded_endpoint).CombinedOutput()
		require.NoError(t, err, "adb disconnect output: %q", out)

		out, err = exec.Command("adb", "connect", adb_forwarded_endpoint).CombinedOutput()
		require.NoError(t, err, "adb connect output: %q", out)

		cmd = exec.Command("adb", "-s", adb_forwarded_endpoint, "shell", "echo", "Hello, World!")
		out, err = cmd.CombinedOutput()
		require.Error(t, err, "command output: %q", out)
		feedback.Printf("Command output:\n%s\n", string(out))
	})
}
