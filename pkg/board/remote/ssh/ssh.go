package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/arduino/arduino-app-cli/pkg/board/remote"
)

type SSHConnection struct {
	client *ssh.Client
}

// Ensures SSHConnection implements the RemoteConn interface at compile time.
var _ remote.RemoteConn = (*SSHConnection)(nil)

func FromHost(user, password, address string) (*SSHConnection, error) {
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		// TODO: audit the security of this setting
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint:gosec
	})
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}

	return &SSHConnection{
		client: client,
	}, nil
}

func (a *SSHConnection) Forward(ctx context.Context, remotePort int) (int, error) {
	panic("`Forward` is not implemented for SSHConnection")
}

func (a *SSHConnection) ForwardKillAll(ctx context.Context) error {
	panic("`ForwardKillAll` is not implemented for SSHConnection")
}

func (a *SSHConnection) List(path string) ([]remote.FileInfo, error) {
	session, err := a.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// Run the `ls -la` command on the remote host
	cmd := fmt.Sprintf("ls -la %s", path)
	output, err := session.Output(cmd)
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(output, []byte("\n"))
	if len(lines) > 0 {
		lines = lines[1:] // Skip the first line (header)
	}

	files := make([]remote.FileInfo, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		parts := bytes.Fields(line)
		if len(parts) < 9 {
			continue
		}
		name := string(parts[len(parts)-1])
		if name == "." || name == ".." {
			continue
		}
		files = append(files, remote.FileInfo{
			Name:  name,
			IsDir: line[0] == 'd',
		})
	}

	return files, nil
}

func (a *SSHConnection) MkDirAll(path string) error {
	session, err := a.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	cmd := fmt.Sprintf("mkdir -p %s", path)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (a *SSHConnection) WriteFile(r io.Reader, path string) error {
	session, err := a.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	cmd := fmt.Sprintf("cat > %s", path)
	session.Stdin = r

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

type WithCloser struct {
	io.Reader
	CloseFun func() error
}

func (w WithCloser) Close() error {
	if w.CloseFun != nil {
		return w.CloseFun()
	}
	return nil
}

func (a *SSHConnection) ReadFile(path string) (io.ReadCloser, error) {
	session, err := a.client.NewSession()
	if err != nil {
		return nil, err
	}

	cmd := fmt.Sprintf("cat %s", path)
	output, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := session.Start(cmd); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return WithCloser{
		Reader:   output,
		CloseFun: session.Close,
	}, nil
}

func (a *SSHConnection) Remove(path string) error {
	session, err := a.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	cmd := fmt.Sprintf("rm -rf %s", path)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}

func (a *SSHConnection) Stats(p string) (remote.FileInfo, error) {
	session, err := a.client.NewSession()
	if err != nil {
		return remote.FileInfo{}, err
	}
	defer session.Close()

	cmd := fmt.Sprintf("file %s", p)
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
	cmd     string
	err     error
}

func (a *SSHConnection) GetCmd(cmd string, args ...string) remote.Cmder {
	session, err := a.client.NewSession()
	if err != nil {
		return &SSHCommand{
			err: fmt.Errorf("failed to create SSH session: %w", err),
		}
	}

	// TODO: fix for command injection vulnerability
	cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))

	return &SSHCommand{
		session: session,
		cmd:     cmd,
	}
}

func (c SSHCommand) Run(ctx context.Context) error {
	if c.err != nil {
		return c.err
	}

	defer c.session.Close()
	return c.session.Run(c.cmd)
}

func (c *SSHCommand) Output(ctx context.Context) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}

	defer c.session.Close()
	return c.session.CombinedOutput(c.cmd)
}

func (c *SSHCommand) Interactive() (io.WriteCloser, io.Reader, io.Reader, remote.Closer, error) {
	if c.err != nil {
		return nil, nil, nil, nil, c.err
	}

	c.session.Stderr = c.session.Stdout // Redirect stderr to stdout
	stdin, err := c.session.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := c.session.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := c.session.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := c.session.Start(c.cmd); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return stdin, stdout, stderr, func() error {
		if err := c.session.Wait(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		_ = c.session.Close()
		return nil
	}, nil
}
