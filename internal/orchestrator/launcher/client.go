// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package launcher is the client of the control socket of the app launcher that `run.sh serve`
// runs in an app container: one JSON request per connection, answered by one JSON line or,
// for the events, by one line each time the app starts or ends.
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

// AppState is the app of a launcher: the one it runs and how the previous one ended.
type AppState struct {
	// Child is the app the launcher runs, nil when none.
	Child *Child `json:"child"`
	// Last is how the previous app process ended, nil when none ran.
	Last *Ended `json:"last"`
}

type StatusResponse struct {
	AppState
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
	cmdStart  = "start"
	cmdStop   = "stop"
	cmdStatus = "status"
	cmdEvents = "events"
)

type request struct {
	Cmd string `json:"cmd"`
	App string `json:"app,omitempty"`
	// Env is the app environment, which the child runs with over the container one.
	Env map[string]string `json:"env,omitempty"`
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

// Events calls onState with the state of the app as it is now, then each time the app starts
// or ends, on one connection kept open. It returns when the launcher refuses the request or
// goes, or with the error of the context once it is done. The timeout bounds the exchange up
// to the first state only.
func (c *Client) Events(ctx context.Context, onState func(AppState)) error {
	openCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, err := c.send(openCtx, request{Cmd: cmdEvents})
	if err != nil {
		return err
	}
	defer conn.Close()
	// A done context closes the connection, which ends the read
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()

	answers := newAnswers(conn)
	for first := true; ; first = false {
		var state AppState
		if err := answers.read(cmdEvents, &state); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if first {
			// The next states come when the app changes, whenever that is
			if err := conn.SetDeadline(time.Time{}); err != nil {
				return err
			}
		}
		onState(state)
	}
}

func (c *Client) do(ctx context.Context, req request, resp any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, err := c.send(ctx, req)
	if err != nil {
		return err
	}
	defer conn.Close()
	return newAnswers(conn).read(req.Cmd, resp)
}

// send opens a connection to the launcher, checks who answers and sends the request, all
// within the deadline of the context, which the connection keeps for its answers.
func (c *Client) send(ctx context.Context, req request) (_ net.Conn, err error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.socket)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the app launcher: %w", err)
	}
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	if c.peer != nil {
		pid, err := peerPID(conn)
		if err != nil {
			return nil, fmt.Errorf("cannot tell who answers the app launcher socket: %w", err)
		}
		// An unknown pid is a platform without peer credentials, development only
		if pid != 0 {
			if err := c.peer(pid); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrPeer, err)
			}
		}
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}

	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("cannot send %s to the app launcher: %w", req.Cmd, err)
	}
	return conn, nil
}

// answers reads the lines the launcher answers on one connection.
type answers struct {
	lines *bufio.Scanner
}

func newAnswers(conn net.Conn) answers {
	lines := bufio.NewScanner(conn)
	lines.Buffer(make([]byte, 0, 4096), maxResponseBytes)
	return answers{lines: lines}
}

// read decodes the next answer to cmd into resp, or returns the refusal it carries.
func (a answers) read(cmd string, resp any) error {
	if !a.lines.Scan() {
		err := a.lines.Err()
		if err == nil {
			err = errors.New("connection closed")
		}
		return fmt.Errorf("no answer from the app launcher to %s: %w", cmd, err)
	}
	reply := a.lines.Bytes()
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

// IsCode tells whether err is a launcher error with that code.
func IsCode(err error, code string) bool {
	var launcherErr *Error
	return errors.As(err, &launcherErr) && launcherErr.Code == code
}
