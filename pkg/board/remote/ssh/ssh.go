// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path"
	"slices"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/x/ports"
)

var ErrAuthFailed = errors.New("ssh authentication failed")

type SSHConnection struct {
	client   *ssh.Client
	sessions chan struct{}
	wg       sync.WaitGroup

	mu             sync.Mutex
	ForwardedPorts []ForwardedPort
}

type ForwardedPort struct {
	Listener   net.Listener
	LocalPort  int
	RemotePort int
}

// Ensures SSHConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*SSHConnection)(nil)

// Option configures a connection returned by FromHost.
type Option func(*SSHConnection)

// WithMaxSessions bounds the sessions open at once. sshd never reports its own
// MaxSessions, so this cap only lowers the chance of a refusal; 10 is its default.
func WithMaxSessions(n int) Option {
	return func(a *SSHConnection) {
		a.sessions = make(chan struct{}, max(n, 1))
	}
}

func FromHost(user, password, address string, opts ...Option) (*SSHConnection, error) {
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		// TODO: audit the security of this setting
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint:gosec
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unable to authenticate") ||
			strings.Contains(msg, "no supported methods remain") ||
			strings.Contains(msg, "permission denied") {
			return nil, ErrAuthFailed
		}
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	conn := &SSHConnection{client: client}
	for _, opt := range append([]Option{WithMaxSessions(10)}, opts...) {
		opt(conn)
	}
	return conn, nil
}

// newSession opens a session and takes a slot of the session budget. The
// returned release gives the slot back: it must run when the session ends, not
// when the call that opened it returns.
func (a *SSHConnection) newSession() (*ssh.Session, func(), error) {
	a.sessions <- struct{}{}
	session, err := a.client.NewSession()
	if err != nil {
		<-a.sessions
		return nil, nil, err
	}
	return session, sync.OnceFunc(func() { <-a.sessions }), nil
}

func (a *SSHConnection) Forward(ctx context.Context, localPort int, remotePort int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if slices.ContainsFunc(a.ForwardedPorts, func(fp ForwardedPort) bool {
		return fp.LocalPort == localPort && fp.RemotePort == remotePort
	}) {
		return nil // Port already forwarded as requested
	}

	if !ports.IsAvailable(localPort) {
		return remote.ErrPortAvailable
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", localPort))
	if err != nil {
		return err
	}

	a.ForwardedPorts = append(a.ForwardedPorts, ForwardedPort{
		Listener:   listener,
		LocalPort:  localPort,
		RemotePort: remotePort,
	})

	a.wg.Add(1)
	go func() {
		defer listener.Close()
		defer a.wg.Done()

		for {
			localConn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					slog.Warn("failed to accept local connection:", slog.Any("error", err))
				}
				return
			}

			go func(localConn net.Conn, remotePort int) {
				defer localConn.Close()

				// TODO: the kill operation should forcefully terminate the connection that was already estabish

				// Open remote connection through SSH
				remoteConn, err := a.client.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
				if err != nil {
					slog.Warn("failed to dial remote host:", slog.Any("error", err))
					return
				}
				defer remoteConn.Close()

				// Bidirectional copy
				var wg sync.WaitGroup
				wg.Go(func() { copyAndLog(remoteConn, localConn) })
				wg.Go(func() { copyAndLog(localConn, remoteConn) })
				wg.Wait()
			}(localConn, remotePort)
		}
	}()

	return nil
}

func copyAndLog(dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	if err != nil {
		slog.Warn("failed to copy connection", slog.Any("error", err))
	}
}

func (a *SSHConnection) ForwardKillAll(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, fp := range a.ForwardedPorts {
		if err := fp.Listener.Close(); err != nil {
			return err
		}
	}
	a.wg.Wait()
	a.ForwardedPorts = make([]ForwardedPort, 0)
	return nil
}

func (a *SSHConnection) List(path string) ([]remote.FileInfo, error) {
	session, release, err := a.newSession()
	if err != nil {
		return nil, err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("ls -laQ %s", remote.ShellQuote(path))
	output, err := session.Output(cmd)
	if err != nil {
		return nil, err
	}

	return remote.ParseLsOutput(bytes.NewReader(output))
}

func (a *SSHConnection) MkDirAll(path string) error {
	session, release, err := a.newSession()
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("mkdir -p %s", remote.ShellQuote(path))
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (a *SSHConnection) WriteFile(r io.Reader, path string) error {
	session, release, err := a.newSession()
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("cat > %s", remote.ShellQuote(path))
	session.Stdin = r

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (a *SSHConnection) ReadFile(path string) (io.ReadCloser, error) {
	session, release, err := a.newSession()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	// LC_ALL=C: the parser reads the failure of cat in English.
	cmd := fmt.Sprintf("LC_ALL=C cat %s", remote.ShellQuote(path))
	output, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		release()
		return nil, err
	}

	if err := session.Start(cmd); err != nil {
		_ = session.Close()
		release()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Wait is not idempotent, and the parser can ask for the end twice.
	exit := sync.OnceValues(func() ([]byte, error) {
		// Wait first: it ends the copy of stderr.
		err := session.Wait()
		return stderr.Bytes(), err
	})
	r, err := remote.ParseReadOutput(output, exit)
	if err != nil {
		_ = session.Close()
		release()
		return nil, err
	}

	return remote.WithCloser{Reader: r, CloseFun: func() error {
		defer release()
		// The read reports the command failure, and an ended session is closed.
		if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	}}, nil
}

func (a *SSHConnection) Remove(path string) error {
	session, release, err := a.newSession()
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("rm -rf %s", remote.ShellQuote(path))
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}

func (a *SSHConnection) Move(src string, dst string) error {
	session, release, err := a.newSession()
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("mv -T -- %s %s", remote.ShellQuote(src), remote.ShellQuote(dst))
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to move %q to %q: %w", src, dst, err)
	}

	return nil
}

func (a *SSHConnection) Stats(p string) (remote.FileInfo, error) {
	session, release, err := a.newSession()
	if err != nil {
		return remote.FileInfo{}, err
	}
	defer release()
	defer session.Close()

	cmd := fmt.Sprintf("file -L %s", remote.ShellQuote(p))
	output, err := session.Output(cmd)
	if err != nil {
		return remote.FileInfo{}, err
	}

	line := bytes.TrimSpace(output)
	parts := bytes.Split(line, []byte(":"))
	if len(parts) < 2 {
		return remote.FileInfo{}, fmt.Errorf("unexpected file command output: %s", line)
	}

	name := string(bytes.TrimSpace(parts[0]))
	other := string(bytes.TrimSpace(parts[1]))

	if strings.Contains(other, "cannot open") {
		return remote.FileInfo{}, fs.ErrNotExist
	}

	return remote.FileInfo{
		Name:  path.Base(name),
		IsDir: other == "directory",
	}, nil
}

type SSHCommand struct {
	session *ssh.Session
	release func()
	cmd     string
	err     error
}

func (a *SSHConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	session, release, err := a.newSession()
	if err != nil {
		return &SSHCommand{
			err: fmt.Errorf("failed to create SSH session: %w", err),
		}
	}

	parts := make([]string, 0, 1+len(args))
	parts = append(parts, remote.ShellQuote(cmd))
	for _, arg := range args {
		parts = append(parts, remote.ShellQuote(arg))
	}

	return &SSHCommand{
		session: session,
		release: release,
		cmd:     strings.Join(parts, " "),
	}
}

func (c SSHCommand) Run(ctx context.Context) error {
	if c.err != nil {
		return c.err
	}

	defer c.release()
	defer c.session.Close()
	return c.session.Run(c.cmd)
}

func (c *SSHCommand) Output(ctx context.Context) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}

	defer c.release()
	defer c.session.Close()
	return c.session.CombinedOutput(c.cmd)
}

func (c *SSHCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	if c.err != nil {
		return nil, nil, nil, nil, c.err
	}

	// A failed start keeps no session, so the slot goes back here.
	fail := func(err error) (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
		_ = c.session.Close()
		c.release()
		return nil, nil, nil, nil, err
	}

	c.session.Stderr = c.session.Stdout // Redirect stderr to stdout
	stdin, err := c.session.StdinPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stdin pipe: %w", err))
	}
	stdout, err := c.session.StdoutPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stdout pipe: %w", err))
	}
	stderr, err := c.session.StderrPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stderr pipe: %w", err))
	}

	if err := c.session.Start(c.cmd); err != nil {
		return fail(fmt.Errorf("failed to start command: %w", err))
	}

	// The slot is held for the life of the remote process, not of this call.
	return stdin, stdout, stderr, func() error {
		defer func() {
			_ = c.session.Close()
			c.release()
		}()
		if err := c.session.Wait(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		return nil
	}, nil
}

func (a *SSHConnection) Push(ctx context.Context, local, remote string) error {
	isDirLocal := func() bool {
		if info, err := os.Stat(local); err == nil {
			return info.IsDir()
		}
		return false
	}()
	isDirRemote := func() bool {
		if info, err := a.Stats(remote); err == nil {
			return info.IsDir
		}
		return false
	}()

	scpClient := NewScpClient(a)

	if !isDirLocal {
		if isDirRemote {
			return fmt.Errorf("cannot push file %q to directory %q", local, remote)
		}
		return scpClient.PushFile(ctx, local, remote)
	} else {
		name := path.Base(remote)
		if isDirRemote {
			// force folder override by pushing into parent folder.
			remote = path.Dir(remote)
		}
		return scpClient.PushDir(ctx, os.DirFS(local), name, remote)
	}
}
