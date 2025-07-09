package appsync

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
	"github.com/arduino/arduino-app-cli/pkg/board/remotefs"
	"github.com/arduino/arduino-app-cli/pkg/x/testtools"
)

func TestEnableSyncApp(t *testing.T) {
	t.Parallel()
	remotes := []struct {
		name string
		conn remote.RemoteConn
	}{
		{
			name: "adb",
			conn: func() remote.RemoteConn {
				name, adbPort, _ := testtools.StartAdbDContainer(t)
				t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })
				conn, err := adb.FromHost("localhost:"+adbPort, "")
				require.NoError(t, err)
				return conn
			}(),
		},
		{
			name: "ssh",
			conn: func() remote.RemoteConn {
				name, _, sshPort := testtools.StartAdbDContainer(t)
				t.Cleanup(func() { testtools.StopAdbDContainer(t, name) })
				conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
				require.NoError(t, err)
				return conn
			}(),
		},
	}

	// Init the file system on the remote connections.
	for _, remote := range remotes {
		out, err := remote.conn.GetCmd(t.Context(), "mkdir", "-p", "apps/test/python").Output()
		require.NoError(t, err, "output: %q", out)
		out, err = remote.conn.GetCmd(t.Context(), "touch", "apps/test/python/main.py").Output()
		require.NoError(t, err, "output: %q", out)
		out, err = remote.conn.GetCmd(t.Context(), "mkdir", "-p", "apps/test/sketch/").Output()
		require.NoError(t, err, "output: %q", out)
		out, err = remote.conn.GetCmd(t.Context(), "touch", "apps/test/sketch/sketch.ino").Output()
		require.NoError(t, err, "output: %q", out)
		out, err = remote.conn.GetCmd(t.Context(), "touch", "apps/test/app.yml").Output()
		require.NoError(t, err, "output: %q", out)
	}

	for _, remote := range remotes {
		getFiles := func(f fs.FS) []string {
			var files []string
			err := fs.WalkDir(f, ".", func(p string, info fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if p == "." {
					return nil
				}
				files = append(files, p)
				return nil
			})
			if err != nil {
				t.Fatalf("failed to walk dir: %v", err)
			}
			slices.Sort(files)
			return files
		}

		t.Run(remote.name, func(t *testing.T) {
			t.Parallel()

			sync, err := New(remote.conn, "apps")
			require.NoError(t, err)
			tmp, err := sync.EnableSyncApp("test")
			require.NoError(t, err)
			files := getFiles(os.DirFS(tmp))
			require.Equal(t, []string{
				"app.yml",
				"python",
				"python/main.py",
				"sketch",
				"sketch/sketch.ino",
			}, files)

			t.Run("test new file in root folder", func(t *testing.T) {
				err = os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("test"), 0600)
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the file is created
				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"python",
					"python/main.py",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)
			})

			t.Run("test new file in subdir", func(t *testing.T) {
				err = os.WriteFile(filepath.Join(tmp, "python", "test.txt"), []byte("test"), 0600)
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the file is created
				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"python",
					"python/main.py",
					"python/test.txt",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)
			})

			t.Run("test new dir", func(t *testing.T) {
				err = os.MkdirAll(filepath.Join(tmp, "python", "test"), 0700)
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)
				// check if the dir is created

				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"python",
					"python/main.py",
					"python/test",
					"python/test.txt",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)

				// add file in the new dir
				err = os.WriteFile(filepath.Join(tmp, "python", "test", "test.txt"), []byte("test"), 0600)
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the file is created
				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"python",
					"python/main.py",
					"python/test",
					"python/test.txt",
					"python/test/test.txt",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)
			})

			t.Run("test update file", func(t *testing.T) {
				err = os.WriteFile(filepath.Join(tmp, "python/main.py"), []byte("print('Hello')"), 0600)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the file is updated
				buff, err := remote.conn.ReadFile("apps/test/python/main.py")
				require.NoError(t, err)
				b, err := io.ReadAll(buff)
				require.NoError(t, err)
				require.Equal(t, "print('Hello')", string(b))
			})

			t.Run("test delete file", func(t *testing.T) {
				err = os.Remove(filepath.Join(tmp, "python/test.txt"))
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the file is deleted
				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"python",
					"python/main.py",
					"python/test",
					"python/test/test.txt",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)
			})

			t.Run("test delete dir", func(t *testing.T) {
				err = os.RemoveAll(filepath.Join(tmp, "python"))
				require.NoError(t, err)

				// wait for the event to be triggered
				time.Sleep(1 * time.Second)

				// check if the dir is deleted
				files = getFiles(remotefs.New("apps/test", remote.conn))
				require.Equal(t, []string{
					"app.yml",
					"sketch",
					"sketch/sketch.ino",
					"test.txt",
				}, files)
			})
		})
	}
}
