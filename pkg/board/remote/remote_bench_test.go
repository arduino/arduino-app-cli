// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package remote_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/testtools"
	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/adb"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/local"
	"github.com/arduino/arduino-app-cli/pkg/board/remote/ssh"
)

func BenchmarkRemotePush(b *testing.B) {
	b.Run("Native", func(b *testing.B) {
		runPushBenchmark(b, func(ctx context.Context, conn remote.RemoteConn, src, dst string) error {
			return conn.Push(ctx, src, dst)
		})
	})

	b.Run("Base", func(b *testing.B) {
		runPushBenchmark(b, func(ctx context.Context, conn remote.RemoteConn, src, dst string) error {
			return fsWalkPush(conn, src, dst)
		})
	})
	b.Run("Legacy", func(b *testing.B) {
		runPushBenchmark(b, legacyPush)
	})
}

// runPushBenchmark runs the given backends against the standard payload set.
// Sub-benchmark names (transport/payload) are kept identical across
// BenchmarkRemotePushNative and BenchmarkRemotePushFSWalk so they can be
// compared directly with benchstat.
func runPushBenchmark(b *testing.B, pushFunc func(ctx context.Context, conn remote.RemoteConn, src, dst string) error) {
	name, adbPort, sshPort := testtools.StartAdbDContainer(b)
	b.Cleanup(func() { testtools.StopAdbDContainer(b, name) })

	backends := []struct {
		name     string
		conn     remote.RemoteConn
		basePath string
	}{{
		"adb",
		func() remote.RemoteConn {
			conn, err := adb.FromHost("localhost:"+adbPort, "")
			require.NoError(b, err)
			return conn
		}(),
		"/home/arduino",
	}, {
		"ssh",
		func() remote.RemoteConn {
			conn, err := ssh.FromHost("arduino", "arduino", "127.0.0.1:"+sshPort)
			require.NoError(b, err)
			return conn
		}(),
		"./",
	}, {
		"local",
		func() remote.RemoteConn {
			return &local.LocalConnection{}
		}(),
		b.TempDir(),
	},
	}

	payloads := []struct {
		name  string
		build func(tb testing.TB, dir string)
	}{
		{
			// Many small files: 500 files of 1 KiB each.
			name: "ManySmallFiles",
			build: func(tb testing.TB, dir string) {
				const n = 500
				const size = 1 * 1024
				for i := range n {
					writeRandomFile(tb, filepath.Join(dir, fmt.Sprintf("small_%04d.bin", i)), size)
				}
			},
		},
		{
			// Few big files: 3 files of 8 MiB each.
			name: "FewBigFiles",
			build: func(tb testing.TB, dir string) {
				const n = 3
				const size = 8 * 1024 * 1024
				for i := range n {
					writeRandomFile(tb, filepath.Join(dir, fmt.Sprintf("big_%d.bin", i)), size)
				}
			},
		},
		{
			// Mixed: 100 small (2 KiB), 5 medium (256 KiB), 1 big (4 MiB), nested.
			name: "Mixed",
			build: func(tb testing.TB, dir string) {
				for i := range 100 {
					writeRandomFile(tb, filepath.Join(dir, "small", fmt.Sprintf("s_%03d.bin", i)), 2*1024)
				}
				for i := range 5 {
					writeRandomFile(tb, filepath.Join(dir, "medium", fmt.Sprintf("m_%d.bin", i)), 256*1024)
				}
				writeRandomFile(tb, filepath.Join(dir, "big", "b_0.bin"), 4*1024*1024)
			},
		},
	}

	for _, be := range backends {
		b.Run(be.name, func(b *testing.B) {
			for _, p := range payloads {
				b.Run(p.name, func(b *testing.B) {
					// Build payload once per case (outside the timed loop).
					srcDir := b.TempDir()
					payloadDir := filepath.Join(srcDir, "payload")
					require.NoError(b, os.MkdirAll(payloadDir, 0755))
					p.build(b, payloadDir)

					// Compute total bytes for throughput reporting.
					var totalBytes int64
					err := filepath.Walk(payloadDir, func(_ string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if !info.IsDir() {
							totalBytes += info.Size()
						}
						return nil
					})
					require.NoError(b, err)

					// Prepare remote destination base dir.
					remoteBase := path.Join(be.basePath, "bench_push")
					_ = be.conn.Remove(remoteBase)
					require.NoError(b, be.conn.MkDirAll(remoteBase))
					b.Cleanup(func() { _ = be.conn.Remove(remoteBase) })

					b.SetBytes(totalBytes)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						dst := path.Join(be.basePath, fmt.Sprintf("iter_%d", i))
						if err := pushFunc(context.Background(), be.conn, payloadDir, dst); err != nil {
							b.Fatalf("push failed: %v", err)
						}
					}

				})
			}
		})
	}
}

// writeRandomFile creates a file at p filled with size bytes of random data.
func writeRandomFile(tb testing.TB, p string, size int) {
	tb.Helper()
	require.NoError(tb, os.MkdirAll(filepath.Dir(p), 0755))
	f, err := os.Create(p)
	require.NoError(tb, err)
	defer f.Close()
	if size == 0 {
		return
	}
	buf := make([]byte, 64*1024)
	remaining := size
	for remaining > 0 {
		n := min(len(buf), remaining)
		_, err := rand.Read(buf[:n])
		require.NoError(tb, err)
		_, err = f.Write(buf[:n])
		require.NoError(tb, err)
		remaining -= n
	}
}

// fsWalkPush is a baseline implementation that recursively walks src using
// io/fs.WalkDir and replicates the tree on the remote using only the
// remote.FS interface (MkDirAll + WriteFile), without using Push.
func fsWalkPush(rfs remote.FS, src, dst string) error {
	srcFS := os.DirFS(src)
	return fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Use forward slashes for the remote path; p is already slash-separated.
		target := path.Join(dst, p)
		if d.IsDir() {
			return rfs.MkDirAll(target)
		}
		if err := rfs.MkDirAll(path.Dir(target)); err != nil {
			return err
		}
		in, err := srcFS.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		return rfs.WriteFile(in, target)
	})
}

// legacyPush replicates the old ImportFolderToAppFromPath algorithm:
//  1. walk src locally and collect dirs/files
//  2. create all remote dirs concurrently (MkDirAll is recursive/idempotent)
//  3. upload all files concurrently
//
// Kept here only for benchmarking purposes.
func legacyPush(ctx context.Context, conn remote.RemoteConn, src, dst string) error {
	const maxConcurrentUploads = 8

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type fileEntry struct {
		localPath  string
		remotePath string
	}
	var dirs []string
	var files []fileEntry

	if err := filepath.WalkDir(src, func(currentLocalPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, currentLocalPath)
		if err != nil {
			return err
		}
		remoteEntryPath := path.Join(dst, relPath)
		if d.IsDir() {
			dirs = append(dirs, remoteEntryPath)
		} else {
			files = append(files, fileEntry{localPath: currentLocalPath, remotePath: remoteEntryPath})
		}
		return nil
	}); err != nil {
		return err
	}

	sem := make(chan struct{}, maxConcurrentUploads)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	setErr := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	// Phase 2: concurrent MkDirAll.
	for _, d := range dirs {
		if cancelCtx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(d string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := conn.MkDirAll(d); err != nil {
				setErr(err)
			}
		}(d)
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}

	// Phase 3: concurrent WriteFile.
	for _, fe := range files {
		if cancelCtx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(f fileEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			localFile, err := os.Open(f.localPath)
			if err != nil {
				setErr(fmt.Errorf("failed to open local file %q: %w", f.localPath, err))
				return
			}
			defer localFile.Close()

			for range 10 {
				err = conn.WriteFile(localFile, f.remotePath)
				if err == nil {
					return
				}
			}
			_ = conn.Remove(f.remotePath)
			setErr(fmt.Errorf("failed to write remote file %q: %w", f.remotePath, err))
		}(fe)
	}
	wg.Wait()

	return firstErr
}
