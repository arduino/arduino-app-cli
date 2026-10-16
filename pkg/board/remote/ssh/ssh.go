// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package ssh

import (
	"bytes"
	"cmp"
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
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
	"github.com/arduino/arduino-app-cli/pkg/x/ports"
)

var ErrAuthFailed = errors.New("ssh authentication failed")

// ErrTooManySessions reports a session budget that stayed full: the caller is
// saturated, the board did not refuse anything.
var ErrTooManySessions = errors.New("too many concurrent ssh sessions")

type SSHConnection struct {
	client      *ssh.Client
	sessions    chan struct{}
	sessionWait time.Duration
	wg          sync.WaitGroup

	mu             sync.Mutex
	ForwardedPorts []ForwardedPort

	ctx    context.Context
	cancel context.CancelFunc
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

// WithSessionWait bounds the wait for a free session slot when the caller has
// no deadline of its own. It never bounds the session itself. Zero does not
// wait: a full budget fails at once.
func WithSessionWait(d time.Duration) Option {
	return func(a *SSHConnection) {
		a.sessionWait = d
	}
}

func FromHost(user, password, address string, opts ...Option) (*SSHConnection, error) {
	ctx, cancel := context.WithCancel(context.Background())
	board := slog.String("board", address)

	client, err := func() (*ssh.Client, error) {
		tcp, err := net.Dial("tcp", address)
		if err != nil {
			return nil, err
		}
		const silence = 5 * time.Second
		kick := &kickConn{Conn: tcp, timeout: silence, timer: time.AfterFunc(silence, func() {
			if ctx.Err() == nil {
				slog.Warn("the board sent nothing", board, slog.Duration("silence", silence))
			}
			cancel()
		})}
		// The timer runs only after the handshake.
		kick.timer.Stop()
		c, chans, reqs, err := ssh.NewClientConn(kick, address, &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			// TODO: audit the security of this setting
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint:gosec
		})
		if err != nil {
			return nil, err
		}
		kick.armed.Store(true)
		kick.timer.Reset(silence)
		return ssh.NewClient(c, chans, reqs), nil
	}()
	if err != nil {
		cancel()
		msg := err.Error()
		if strings.Contains(msg, "unable to authenticate") ||
			strings.Contains(msg, "no supported methods remain") ||
			strings.Contains(msg, "permission denied") {
			return nil, ErrAuthFailed
		}
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	conn := &SSHConnection{client: client, ctx: ctx, cancel: cancel}
	for _, opt := range append([]Option{WithMaxSessions(10), WithSessionWait(60 * time.Second)}, opts...) {
		opt(conn)
	}
	go conn.watchClose(board)
	go conn.keepalive()
	return conn, nil
}

// Done closes when the board closes the connection, or goes silent.
func (a *SSHConnection) Done() <-chan struct{} {
	return a.ctx.Done()
}

// Close stops the forwards and closes the ssh connection.
func (a *SSHConnection) Close() error {
	a.cancel()
	return errors.Join(a.ForwardKillAll(context.Background()), a.client.Close())
}

// newSession opens a session and takes a slot of the session budget. The
// returned release gives the slot back: it must run when the session ends, not
// when the call that opened it returns.
func (a *SSHConnection) newSession(ctx context.Context, what string) (*ssh.Session, func(), error) {
	if a.sessionWait <= 0 {
		select {
		case a.sessions <- struct{}{}:
		default:
			return nil, nil, fmt.Errorf("%w: %s", ErrTooManySessions, what)
		}
	} else {
		select {
		case a.sessions <- struct{}{}:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		// The bound of a caller with no deadline: a slot can stay out for ever.
		case <-time.After(a.sessionWait):
			return nil, nil, fmt.Errorf("%w: %s", ErrTooManySessions, what)
		}
	}

	// The board frees a session a moment after we do, so a refusal right after a
	// release is normal: back off and open again.
	// 50ms, 100ms, 200ms, 400ms, then give up.
	const (
		minBackoff = 50 * time.Millisecond
		maxBackoff = 400 * time.Millisecond
	)
	for backoff := minBackoff; ; backoff *= 2 {
		session, err := a.client.NewSession()
		if err == nil {
			return session, sync.OnceFunc(func() { <-a.sessions }), nil
		}

		// Only the server refusing the channel is worth another try.
		_, refused := errors.AsType[*ssh.OpenChannelError](err)
		if !refused || backoff > maxBackoff {
			<-a.sessions
			return nil, nil, err
		}

		select {
		case <-ctx.Done():
			<-a.sessions
			return nil, nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
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
	cmd := fmt.Sprintf("ls -laQ %s", remote.ShellQuote(path))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return nil, err
	}
	defer release()
	defer session.Close()

	output, err := session.Output(cmd)
	if err != nil {
		return nil, err
	}

	return remote.ParseLsOutput(bytes.NewReader(output))
}

func (a *SSHConnection) MkDirAll(path string) error {
	cmd := fmt.Sprintf("mkdir -p %s", remote.ShellQuote(path))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (a *SSHConnection) WriteFile(r io.Reader, path string) error {
	cmd := fmt.Sprintf("cat > %s", remote.ShellQuote(path))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	session.Stdin = r

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (a *SSHConnection) ReadFile(path string) (io.ReadCloser, error) {
	// LC_ALL=C: the parser reads the failure of cat in English.
	cmd := fmt.Sprintf("LC_ALL=C cat %s", remote.ShellQuote(path))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

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
	cmd := fmt.Sprintf("rm -rf %s", remote.ShellQuote(path))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}

func (a *SSHConnection) Move(src string, dst string) error {
	cmd := fmt.Sprintf("mv -T -- %s %s", remote.ShellQuote(src), remote.ShellQuote(dst))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return err
	}
	defer release()
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to move %q to %q: %w", src, dst, err)
	}

	return nil
}

func (a *SSHConnection) Stats(p string) (remote.FileInfo, error) {
	cmd := fmt.Sprintf("file -L %s", remote.ShellQuote(p))
	session, release, err := a.newSession(context.Background(), cmd)
	if err != nil {
		return remote.FileInfo{}, err
	}
	defer release()
	defer session.Close()

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
	conn *SSHConnection
	cmd  string
}

func (a *SSHConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, remote.ShellQuote(cmd))
	for _, arg := range args {
		parts = append(parts, remote.ShellQuote(arg))
	}

	// The session is taken when the command runs, so an unused Cmder costs none.
	return &SSHCommand{conn: a, cmd: strings.Join(parts, " ")}
}

func (c *SSHCommand) Run(ctx context.Context) error {
	session, release, err := c.conn.newSession(ctx, c.cmd)
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer release()
	defer session.Close()

	// Closing the session unblocks Wait: sshd commonly ignores a signal request.
	defer context.AfterFunc(ctx, func() { _ = session.Close() })()
	err = session.Run(c.cmd)
	return cmp.Or(ctx.Err(), err)
}

func (c *SSHCommand) Output(ctx context.Context) ([]byte, error) {
	session, release, err := c.conn.newSession(ctx, c.cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer release()
	defer session.Close()

	// Closing the session unblocks Wait: sshd commonly ignores a signal request.
	defer context.AfterFunc(ctx, func() { _ = session.Close() })()
	// A canceled command still reports what it wrote before the end.
	out, err := session.CombinedOutput(c.cmd)
	return out, cmp.Or(ctx.Err(), err)
}

func (c *SSHCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	// No ctx here: the Closer the caller gets is the end of the session.
	session, release, err := c.conn.newSession(context.Background(), c.cmd)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create SSH session: %w", err)
	}

	// A failed start keeps no session, so the slot goes back here.
	fail := func(err error) (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
		_ = session.Close()
		release()
		return nil, nil, nil, nil, err
	}

	session.Stderr = session.Stdout // Redirect stderr to stdout
	stdin, err := session.StdinPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stdin pipe: %w", err))
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stdout pipe: %w", err))
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fail(fmt.Errorf("failed to get stderr pipe: %w", err))
	}

	if err := session.Start(c.cmd); err != nil {
		return fail(fmt.Errorf("failed to start command: %w", err))
	}

	// The slot is held for the life of the remote process, not of this call.
	return stdin, stdout, stderr, func() error {
		defer func() {
			_ = session.Close()
			release()
		}()
		if err := session.Wait(); err != nil {
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

// watchClose cancels the ctx when the board closes the connection.
func (a *SSHConnection) watchClose(board slog.Attr) {
	err := a.client.Wait()
	if a.ctx.Err() == nil {
		slog.Warn("the board closed the ssh connection", board, slog.Any("error", err))
	}
	a.cancel()
}

// keepalive sends the board a global request every second.
func (a *SSHConnection) keepalive() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(time.Second):
		}
		_, _, _ = a.client.SendRequest("keepalive@openssh.com", true, nil)
	}
}

// kickConn is the kicker of go.bug.st/downloader on a net.Conn: once armed, every byte
// read resets the timer.
type kickConn struct {
	net.Conn
	timer   *time.Timer
	timeout time.Duration
	armed   atomic.Bool
}

func (c *kickConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 && c.armed.Load() {
		c.timer.Reset(c.timeout)
	}
	return n, err
}

func (c *kickConn) Close() error {
	c.timer.Stop()
	return c.Conn.Close()
}
