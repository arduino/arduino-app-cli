package remotefs_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
	"github.com/arduino/arduino-app-cli/pkg/x/testtools"
)

func TestRemoteFS(t *testing.T) {
	t.Parallel()

	remotes := []struct {
		name string
		conn remote.FS
	}{
		{
			name: "adb",
			conn: func() remote.FS {
				name, adbPort, _ := testtools.StartAdbDContainer(t)
				t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })
				conn, err := adb.FromHost("localhost:"+adbPort, "")
				require.NoError(t, err)
				return conn
			}(),
		},
		{
			name: "ssh",
			conn: func() remote.FS {
				name, _, sshPort := testtools.StartAdbDContainer(t)
				t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })
				conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
				require.NoError(t, err)
				return conn
			}(),
		},
		{
			name: "local",
			conn: &local.LocalConnection{},
		},
	}

	getRandData := func(t *testing.T, size int) []byte {
		buf := make([]byte, size)
		n, err := rand.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, size, n)
		return buf
	}
	file1 := getRandData(t, 512)   // 512 bytes
	file2 := getRandData(t, 1024)  // 1 KB
	file3 := getRandData(t, 2048)  // 2 KB
	file4 := getRandData(t, 4096)  // 4 KB
	file5 := getRandData(t, 8192)  // 8 KB
	file6 := getRandData(t, 16384) // 16 KB

	// init the remote file systems
	for _, remote := range remotes {
		// Create the base directory
		require.NoError(t, remote.conn.MkDirAll("test"))
		require.NoError(t, remote.conn.MkDirAll("empty"))

		// Create subdir1 and its files
		require.NoError(t, remote.conn.MkDirAll("test/dir/subdir1"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file1), "test/dir/subdir1/file1.txt"))

		// Create subdir2 and its files
		require.NoError(t, remote.conn.MkDirAll("test/dir/subdir2"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file2), "test/dir/subdir2/file2.txt"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file3), "test/dir/subdir2/file3.txt"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file4), "test/dir/subdir2/file4.txt"))

		// Create subdir3/nested1 and its files
		require.NoError(t, remote.conn.MkDirAll("test/dir/subdir3/nested1"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file5), "test/dir/subdir3/nested1/file5.txt"))
		require.NoError(t, remote.conn.WriteFile(bytes.NewReader(file6), "test/dir/subdir3/nested1/file6.txt"))

		// Create subdir4 (empty directory)
		require.NoError(t, remote.conn.MkDirAll("test/dir/subdir4"))
	}

	for _, remote := range remotes {
		t.Run("read/"+remote.name, func(t *testing.T) {
			t.Parallel()
			assertFile := func(path string, expected []byte) {
				r, err := remote.conn.ReadFile(path)
				require.NoError(t, err)
				data, err := io.ReadAll(r)
				require.NoError(t, err)
				assert.Equal(t, expected, data)
			}

			assertFile("test/dir/subdir1/file1.txt", file1)
			assertFile("test/dir/subdir2/file2.txt", file2)
			assertFile("test/dir/subdir2/file3.txt", file3)
			assertFile("test/dir/subdir2/file4.txt", file4)
			assertFile("test/dir/subdir3/nested1/file5.txt", file5)
			assertFile("test/dir/subdir3/nested1/file6.txt", file6)
		})

		t.Run("fstest/"+remote.name, func(t *testing.T) {
			t.Parallel()
			t.Run("not empty", func(t *testing.T) {
				t.Parallel()
				myFS := remotefs.New("test", remote.conn)
				assert.NoError(t, fstest.TestFS(myFS,
					"dir/subdir1/file1.txt",
					"dir/subdir2/file2.txt",
					"dir/subdir2/file3.txt",
					"dir/subdir2/file4.txt",
					"dir/subdir3/nested1/file5.txt",
					"dir/subdir3/nested1/file6.txt",
				))
			})

			t.Run("empty", func(t *testing.T) {
				t.Parallel()
				myFS := remotefs.New("empty", remote.conn)
				assert.NoError(t, fstest.TestFS(myFS))
			})
		})
	}
}
