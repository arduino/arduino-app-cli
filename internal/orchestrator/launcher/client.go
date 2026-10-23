// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package launcher is the client of the control socket of the app launcher that `run.sh serve`
// runs in an app container: one JSON request per connection, answered by one JSON line.
package launcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// SocketName is the control socket file name inside the runtime directory of an app.
const SocketName = "app-launcher.sock"

const maxResponseBytes = 1024 * 1024

// Error codes of the launcher protocol.
const (
	CodeBadRequest = "bad_request"
	CodeNotFound   = "not_found"
	CodeBusy       = "busy"
	CodeInternal   = "internal"
	// CodeStale is a start of an app whose dependencies changed since its container prepared
	// them: nothing was done, the app needs a fresh container.
	CodeStale = "stale"
)

// Error is a request the launcher refused or failed.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("app launcher: %s: %s", e.Code, e.Message)
}

type Timings struct {
	StopMs float64 `json:"stop_ms"`
	ForkMs float64 `json:"fork_ms"`
}

type StartResponse struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Runner    string    `json:"runner"`
	Timings   Timings   `json:"timings"`
}

// Ended is how an app process ended: by a signal or with an exit code.
type Ended struct {
	App      string `json:"app"`
	PID      int    `json:"pid"`
	ExitCode *int   `json:"exit_code"`
	Signal   *int   `json:"signal"`
}

type StopResponse struct {
	Stopped []Ended `json:"stopped"`
}

type Child struct {
	App       string    `json:"app"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Runner    string    `json:"runner"`
	Stopping  bool      `json:"stopping"`
}

type PingResponse struct {
	// Stale tells the dependencies of the app changed since the container prepared them.
	Stale bool `json:"stale"`
}

type StatusResponse struct {
	// Child is the app the launcher runs, nil when none.
	Child *Child `json:"child"`
	// Last is how the previous app process ended, nil when none ran.
	Last    *Ended         `json:"last"`
	Stale   bool           `json:"stale"`
	Preload map[string]any `json:"preload"`
	Version string         `json:"version"`
	UptimeS float64        `json:"uptime_s"`
}

// PeerCheck tells whether the process answering the socket, given by its pid as the caller
// sees it, is the launcher expected there: the app shares the socket folder with its
// launcher, so the path proves nothing.
type PeerCheck func(pid int) error

// ErrPeer is returned when the socket is answered by a process other than the app launcher.
var ErrPeer = errors.New("the socket is not answered by the app launcher")

// Client talks to the launcher on a Unix socket. Each call bounds the whole exchange by
// timeout, or by the context deadline when it is earlier.
type Client struct {
	socket  string
	timeout time.Duration
	peer    PeerCheck
}

func New(socket string, timeout time.Duration) *Client {
	return &Client{socket: socket, timeout: timeout}
}

// WithTimeout returns a client on the same socket with another timeout.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	return &Client{socket: c.socket, timeout: timeout, peer: c.peer}
}

// WithPeer returns a client that accepts an answer only from a process the check approves.
func (c *Client) WithPeer(check PeerCheck) *Client {
	return &Client{socket: c.socket, timeout: c.timeout, peer: check}
}

func (c *Client) Socket() string {
	return c.socket
}

// Commands of the launcher protocol.
const (
	cmdPing   = "ping"
	cmdStart  = "start"
	cmdStop   = "stop"
	cmdStatus = "status"
)

type request struct {
	Cmd string `json:"cmd"`
	App string `json:"app,omitempty"`
	// Env is the app environment, which the child runs with over the container one.
	Env map[string]string `json:"env,omitempty"`
}

func (c *Client) Ping(ctx context.Context) (PingResponse, error) {
	var resp PingResponse
	err := c.do(ctx, request{Cmd: cmdPing}, &resp)
	return resp, err
}

// Start runs the app with its environment in place of the running one.
func (c *Client) Start(ctx context.Context, app string, env map[string]string) (StartResponse, error) {
	var resp StartResponse
	err := c.do(ctx, request{Cmd: cmdStart, App: app, Env: env}, &resp)
	return resp, err
}

// Stop stops the running app. No running app is not an error.
func (c *Client) Stop(ctx context.Context) (StopResponse, error) {
	var resp StopResponse
	err := c.do(ctx, request{Cmd: cmdStop}, &resp)
	return resp, err
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	var resp StatusResponse
	err := c.do(ctx, request{Cmd: cmdStatus}, &resp)
	return resp, err
}

func (c *Client) do(ctx context.Context, req request, resp any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.socket)
	if err != nil {
		return fmt.Errorf("cannot reach the app launcher: %w", err)
	}
	defer conn.Close()
	if c.peer != nil {
		pid, err := peerPID(conn)
		if err != nil {
			return fmt.Errorf("cannot tell who answers the app launcher socket: %w", err)
		}
		// An unknown pid is a platform without peer credentials, development only
		if pid != 0 {
			if err := c.peer(pid); err != nil {
				return fmt.Errorf("%w: %v", ErrPeer, err)
			}
		}
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	line, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("cannot send %s to the app launcher: %w", req.Cmd, err)
	}

	reply, err := readLine(conn)
	if err != nil {
		return fmt.Errorf("no answer from the app launcher to %s: %w", req.Cmd, err)
	}
	var envelope struct {
		OK    bool   `json:"ok"`
		Error *Error `json:"error"`
	}
	if err := json.Unmarshal(reply, &envelope); err != nil {
		return fmt.Errorf("invalid answer from the app launcher: %w", err)
	}
	if !envelope.OK {
		if envelope.Error == nil {
			return &Error{Code: CodeInternal, Message: "request refused without a reason"}
		}
		return envelope.Error
	}
	if resp == nil {
		return nil
	}
	if err := json.Unmarshal(reply, resp); err != nil {
		return fmt.Errorf("invalid answer from the app launcher: %w", err)
	}
	return nil
}

// peerPID gives the pid of the process on the other end of a connected unix socket, 0 when the platform cannot tell.
var peerPID = socketPeerPID

func readLine(conn net.Conn) ([]byte, error) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), maxResponseBytes)
	if scanner.Scan() {
		return scanner.Bytes(), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("connection closed")
}

// IsCode tells whether err is a launcher error with that code.
func IsCode(err error, code string) bool {
	var launcherErr *Error
	return errors.As(err, &launcherErr) && launcherErr.Code == code
}
