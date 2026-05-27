// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package sftpfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/pkg/sftp"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

var _ remote.FS = (*SftpFS)(nil)

// SftpFS is an implementation of the FS interface that uses an SFTP client to perform file operations.
type SftpFS struct {
	dial SftpFSDialer

	initMu sync.Mutex
	client atomic.Pointer[sftp.Client]
	extra  []CloseFunc
}

type SftpFSDialer func() (*sftp.Client, []CloseFunc, error)

type CloseFunc func() error

func New(dial SftpFSDialer) *SftpFS {
	return &SftpFS{dial: dial}
}

func (s *SftpFS) get() (*sftp.Client, error) {
	if c := s.client.Load(); c != nil {
		slog.Debug("sftpfs: reusing existing client")
		return c, nil
	}
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if c := s.client.Load(); c != nil {
		slog.Debug("sftpfs: reusing existing client (after lock)")
		return c, nil
	}
	slog.Info("sftpfs: dialing new SFTP connection")
	c, extra, err := s.dial()
	if err != nil {
		slog.Error("sftpfs: failed to dial SFTP connection", "error", err)
		return nil, err
	}
	slog.Info("sftpfs: SFTP connection established")
	s.client.Store(c)
	s.extra = extra
	go s.watch(c)
	return c, nil
}

// watch blocks until the given sftp client's underlying connection ends,
// then invalidates the cache if this client is still the current one.
func (s *SftpFS) watch(c *sftp.Client) {
	slog.Info("sftpfs: watching client for disconnect")
	_ = c.Wait()
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if s.client.Load() != c {
		slog.Debug("sftpfs: client already replaced or closed, skipping invalidation")
		return
	}
	slog.Warn("sftpfs: client connection ended, invalidating cache")
	if err := s.closeLocked(); err != nil {
		slog.Warn("sftpfs: error while closing stale client", "error", err)
	}
}

// Teardown closes the cached client and runs the extra close funcs.
// After this method is called, the SftpFS instance can be reused and will create a new client on demand.
func (s *SftpFS) Teardown() error {
	slog.Info("sftpfs: teardown requested")
	s.initMu.Lock()
	defer s.initMu.Unlock()
	err := s.closeLocked()
	if err != nil {
		slog.Warn("sftpfs: teardown completed with errors", "error", err)
	} else {
		slog.Info("sftpfs: teardown completed successfully")
	}
	return err
}

// closeLocked closes the cached client and runs the extra close funcs.
// Caller must hold s.initMu.
func (s *SftpFS) closeLocked() error {
	c := s.client.Load()
	if c == nil {
		slog.Debug("sftpfs: closeLocked called but no client to close")
		return nil
	}
	slog.Info("sftpfs: closing SFTP client")
	err := c.Close()
	if err != nil {
		slog.Warn("sftpfs: error closing SFTP client", "error", err)
	}
	for _, f := range s.extra {
		if e := f(); e != nil {
			slog.Warn("sftpfs: error running extra close func", "error", e)
			err = errors.Join(err, e)
		}
	}
	s.client.Store(nil)
	s.extra = nil
	slog.Info("sftpfs: client closed and cache cleared")
	return err
}

func (s *SftpFS) List(path string) ([]remote.FileInfo, error) {
	c, err := s.get()
	if err != nil {
		return nil, err
	}
	entries, err := c.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list %q: %w", path, err)
	}
	out := make([]remote.FileInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, remote.FileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Mode:  uint32(e.Mode().Perm()),
		})
	}
	return out, nil
}

func (s *SftpFS) Stats(p string) (remote.FileInfo, error) {
	c, err := s.get()
	if err != nil {
		return remote.FileInfo{}, err
	}
	info, err := c.Stat(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return remote.FileInfo{}, fs.ErrNotExist
		}
		return remote.FileInfo{}, fmt.Errorf("failed to stat %q: %w", p, err)
	}
	return remote.FileInfo{
		Name:  filepath.Base(p),
		IsDir: info.IsDir(),
		Mode:  uint32(info.Mode().Perm()),
	}, nil
}

func (s *SftpFS) ReadFile(path string) (io.ReadCloser, error) {
	c, err := s.get()
	if err != nil {
		return nil, err
	}
	f, err := c.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", path, err)
	}
	return f, nil
}

func (s *SftpFS) WriteFile(r io.Reader, path string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	f, err := c.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write file %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file %q: %w", path, err)
	}

	if err := c.Chmod(path, 0664); err != nil {
		return fmt.Errorf("failed to set permissions for file %q: %w", path, err)
	}

	return nil
}

func (s *SftpFS) MkDirAll(path string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	if err := c.MkdirAll(path); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", path, err)
	}

	if err := c.Chmod(path, 0775); err != nil {
		return fmt.Errorf("failed to set permissions for directory %q: %w", path, err)
	}

	return nil
}

func (s *SftpFS) Remove(path string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	if err := removeRec(c, path); err != nil {
		return fmt.Errorf("failed to remove path %q: %w", path, err)
	}
	return nil
}

func (s *SftpFS) Rename(oldPath, newPath string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	if err := c.PosixRename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename %q to %q: %w", oldPath, newPath, err)
	}
	return nil
}

func removeRec(client *sftp.Client, path string) error {
	info, err := client.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return client.Remove(path)
	}
	entries, err := client.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := removeRec(client, filepath.ToSlash(filepath.Join(path, e.Name()))); err != nil {
			return err
		}
	}
	return client.RemoveDirectory(path)
}
