// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package ssh

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

type ScpClient struct {
	Client *ssh.Client
}

func NewScpClient(client *ssh.Client) *ScpClient {
	return &ScpClient{Client: client}
}

const remoteBinary = "scp"

func (c *ScpClient) PushDir(ctx context.Context, fsys fs.FS, name, remote string) error {
	session, err := c.Client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	r, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	defer w.Close()

	cmd := fmt.Sprintf("%s -rt %q", remoteBinary, remote)
	if err := session.Start(cmd); err != nil {
		return err
	}

	rw := &scpSession{r: r, w: w}
	if err := pushDir(ctx, rw, fsys, name); err != nil {
		return err
	}
	_ = rw.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("scp command failed: %w", err)
	}
	return nil
}

func (c *ScpClient) PushFile(ctx context.Context, local, remote string) error {
	session, err := c.Client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	r, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	defer w.Close()

	cmd := fmt.Sprintf("%s -t %q", remoteBinary, remote)
	if err := session.Start(cmd); err != nil {
		return err
	}

	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()

	rw := &scpSession{r: r, w: w}
	if err := pushFile(ctx, rw, f); err != nil {
		return err
	}
	_ = rw.Close()

	return session.Wait()
}

const enableDebug = false

type scpSession struct {
	r io.Reader
	w io.WriteCloser
}

func (s *scpSession) Read(p []byte) (n int, err error) {
	if enableDebug {
		fmt.Printf("Got: %q\n", string(p)) // nolint:forbidigo
	}
	return s.r.Read(p)
}

func (s *scpSession) Write(p []byte) (n int, err error) {
	if enableDebug {
		fmt.Printf("Sent: %q\n", string(p)) // nolint:forbidigo
	}
	return s.w.Write(p)
}

func (s *scpSession) Close() error {
	return s.w.Close()
}

func pushFile(ctx context.Context, rw io.ReadWriteCloser, f fs.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}

	perm := info.Mode().Perm() | 0600 // ensure files are readable and writable by the owner.
	fmt.Fprintf(rw, "C%04o %d %s\n", perm, info.Size(), info.Name())

	if err := checkErr(rw); err != nil {
		return err
	}

	stop := context.AfterFunc(ctx, func() { _ = rw.Close() })
	defer stop()
	if _, err := io.Copy(rw, f); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	fmt.Fprint(rw, "\x00")

	return checkErr(rw)
}

func pushDir(ctx context.Context, rw io.ReadWriteCloser, fsys fs.FS, remoteBase string) error {
	info, err := fs.Stat(fsys, ".")
	if err != nil {
		return err
	}
	return pushDirRec(ctx, rw, fsys, ".", remoteBase, info)
}

func pushDirRec(ctx context.Context, rw io.ReadWriteCloser, fsys fs.FS, name, remote string, info os.FileInfo) error {
	switch info.Mode().Type() {
	case fs.ModeDir:
		perm := info.Mode().Perm() | 0700 // ensure directories are readable, writable and executable by the owner.
		fmt.Fprintf(rw, "D%04o 0 %s\n", perm, remote)
		if err := checkErr(rw); err != nil {
			return err
		}

		dirs, err := fs.ReadDir(fsys, name)
		if err != nil {
			return err
		}
		for _, d1 := range dirs {
			name1 := filepath.Join(name, d1.Name())
			info1, err := d1.Info()
			if err != nil {
				return err
			}
			if err := pushDirRec(ctx, rw, fsys, name1, d1.Name(), info1); err != nil {
				return err
			}
		}
		fmt.Fprint(rw, "E\n")
		if err := checkErr(rw); err != nil {
			return err
		}
	case 0:
		f, err := fsys.Open(name)
		if err != nil {
			return err
		}
		if err := pushFile(ctx, rw, f); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported file type: %s", info.Mode().Type())
	}

	return nil
}

func checkErr(r io.Reader) error {
	buf := make([]byte, 1)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	if buf[0] == 0 {
		return nil
	}

	// On code 1 (warning) or 2 (fatal), the server sends a newline-terminated message.
	var msg []byte
	b := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, b); err != nil {
			break
		}
		if b[0] == '\n' {
			break
		}
		msg = append(msg, b[0])
	}
	return fmt.Errorf("scp error code %d: %s", buf[0], string(msg))
}
