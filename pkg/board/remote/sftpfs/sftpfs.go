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
		return c, nil
	}
	fmt.Printf("DEBUG: SftpFS crete client\n")
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if c := s.client.Load(); c != nil {
		return c, nil
	}
	return s.genLocked()
}

func (s *SftpFS) genLocked() (*sftp.Client, error) {
	c, extra, err := s.dial()
	if err != nil {
		return nil, err
	}
	s.client.Store(c)
	s.extra = extra
	return c, nil
}

// Close tears down the current client (if any). Subsequent calls will re-dial.
func (s *SftpFS) Close() error {
	s.initMu.Lock()
	defer s.initMu.Unlock()
	return s.closeLocked()
}

func (s *SftpFS) closeLocked() error {
	fmt.Printf("DEBUG: SftpFS close client\n")
	var err error
	if c := s.client.Load(); c != nil {
		err = errors.Join(err, c.Close())
	}
	for _, f := range s.extra {
		if err1 := f(); err1 != nil {
			err = errors.Join(err, err1)
		}
	}
	s.client.Store(nil)
	s.extra = nil
	return err
}

// onErr drops the cached client if the connection was lost.
func (s *SftpFS) onErr(err error) {
	fmt.Printf("DEBUG: SftpFS error: %v\n", err)
	if errors.Is(err, sftp.ErrSSHFxConnectionLost) {
		if old := s.client.Swap(nil); old != nil {
			_ = s.Close()
		}
	}
}

func (s *SftpFS) List(path string) ([]remote.FileInfo, error) {
	fmt.Printf("DEBUG: SftpFS list %q\n", path)
	c, err := s.get()
	if err != nil {
		return nil, err
	}
	entries, err := c.ReadDir(path)
	if err != nil {
		s.onErr(err)
		return nil, fmt.Errorf("failed to list %q: %w", path, err)
	}
	out := make([]remote.FileInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, remote.FileInfo{Name: e.Name(), IsDir: e.IsDir()})
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
		s.onErr(err)
		return remote.FileInfo{}, fmt.Errorf("failed to stat %q: %w", p, err)
	}
	return remote.FileInfo{Name: filepath.Base(p), IsDir: info.IsDir()}, nil
}

func (s *SftpFS) ReadFile(path string) (io.ReadCloser, error) {
	fmt.Printf("DEBUG: SftpFS read file %q\n", path)
	c, err := s.get()
	if err != nil {
		return nil, err
	}
	f, err := c.Open(path)
	if err != nil {
		s.onErr(err)
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
		s.onErr(err)
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write file %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file %q: %w", path, err)
	}
	return nil
}

func (s *SftpFS) MkDirAll(path string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	if err := c.MkdirAll(path); err != nil {
		s.onErr(err)
		return fmt.Errorf("failed to create directory %q: %w", path, err)
	}
	return nil
}

func (s *SftpFS) Remove(path string) error {
	c, err := s.get()
	if err != nil {
		return err
	}
	if err := removeRec(c, path); err != nil {
		s.onErr(err)
		return fmt.Errorf("failed to remove path %q: %w", path, err)
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
