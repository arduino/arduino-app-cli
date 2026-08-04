// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type LocalConnection struct{}

// Ensures LocalConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*LocalConnection)(nil)

func (a *LocalConnection) Forward(ctx context.Context, localPort int, remotePort int) error {
	// Locally we don't need to forward ports.
	return nil
}

func (a *LocalConnection) ForwardKillAll(ctx context.Context) error {
	return nil
}

func (a *LocalConnection) List(path string) ([]remote.FileInfo, error) {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", path, err)
	}

	return f.Map(dirs, func(d fs.DirEntry) remote.FileInfo {
		return remote.FileInfo{
			Name:      d.Name(),
			IsDir:     d.IsDir(),
			IsSymlink: d.Type()&fs.ModeSymlink != 0,
		}
	}), nil
}

func (a *LocalConnection) Stats(path string) (remote.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return remote.FileInfo{}, fmt.Errorf("failed to get stats for path %q: %w", path, err)
	}

	return remote.FileInfo{
		Name:  info.Name(),
		IsDir: info.IsDir(),
	}, nil
}

func (a *LocalConnection) ReadFile(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (a *LocalConnection) WriteFile(r io.Reader, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("failed to write to file %q: %w", path, err)
	}

	return nil
}

func (a *LocalConnection) MkDirAll(path string) error {
	return os.MkdirAll(path, 0700)
}

func (a *LocalConnection) Remove(path string) error {
	return os.RemoveAll(path)
}

type LocalCommand struct {
	cmd *paths.Process
	err error
}

func (a *LocalConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	cmds := make([]string, 0, 1+len(args))
	cmds = append(cmds, cmd)
	cmds = append(cmds, args...)

	command, err := paths.NewProcess(nil, cmds...)
	return &LocalCommand{cmd: command, err: err}
}

func (a *LocalCommand) Run(ctx context.Context) error {
	if a.err != nil {
		return fmt.Errorf("failed to create command: %w", a.err)
	}
	return a.cmd.RunWithinContext(ctx)
}

func (a *LocalCommand) Output(ctx context.Context) ([]byte, error) {
	if a.err != nil {
		return nil, fmt.Errorf("failed to create command: %w", a.err)
	}
	return a.cmd.RunAndCaptureCombinedOutput(ctx)
}

func (a *LocalCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	if a.err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create command: %w", a.err)
	}

	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := a.cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return stdin, stdout, stderr, func() error {
		if err := stdout.Close(); err != nil {
			return fmt.Errorf("failed to close stdout pipe: %w", err)
		}
		if err := stderr.Close(); err != nil {
			return fmt.Errorf("failed to close stderr pipe: %w", err)
		}
		if err := a.cmd.Wait(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		return nil
	}, nil
}

func (a *LocalConnection) Push(ctx context.Context, src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source %q: %w", src, err)
	}

	if !info.IsDir() {
		perm := info.Mode().Perm() | 0600 // Ensure files are readable and writable.
		return copyFile(ctx, src, dst, perm)
	}

	return fs.WalkDir(os.DirFS(src), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		target := filepath.Join(dst, path)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get info for file %q: %w", path, err)
		}

		if d.IsDir() {
			perm := info.Mode().Perm() | 0700 // Ensure directories are executable.
			if err := os.MkdirAll(target, perm); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", target, err)
			}
			return nil
		}

		return copyFile(ctx, filepath.Join(src, path), target, info.Mode().Perm())
	})
}

func copyFile(ctx context.Context, src string, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dst, err)
	}
	defer out.Close()

	stop := context.AfterFunc(ctx, func() { _ = in.Close() })
	defer stop()
	if _, err := io.Copy(out, in); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("copying file %q to %q was canceled: %w", src, dst, ctx.Err())
		}
		return fmt.Errorf("failed to copy %q to %q: %w", src, dst, err)
	}
	return nil
}
