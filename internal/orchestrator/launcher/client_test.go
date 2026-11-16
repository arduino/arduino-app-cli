// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package launcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testApp = "/app"

// fakeLauncher answers every connection with the reply of handle, recording the requests.
type fakeLauncher struct {
	socket   string
	requests chan request
}

func newFakeLauncher(t *testing.T, handle func(cmd string) string) *fakeLauncher {
	t.Helper()
	// macOS limits socket paths to 104 bytes: t.TempDir() is often longer
	dir, err := os.MkdirTemp("", "l")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, SocketName)

	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	fake := &fakeLauncher{socket: socket, requests: make(chan request, 16)}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var req request
				if json.Unmarshal(line, &req) != nil {
					return
				}
				fake.requests <- req
				if reply := handle(req.Cmd); reply != "" {
					_, _ = conn.Write([]byte(reply + "\n"))
				}
			}()
		}
	}()
	return fake
}

func TestClientRequests(t *testing.T) {
	fake := newFakeLauncher(t, func(cmd string) string {
		switch cmd {
		case cmdStart:
			return `{"ok":true,"pid":42,"started_at":"2026-10-13T10:00:00.5+00:00","runner":"python","timings":{"stop_ms":12.5,"fork_ms":3}}`
		case cmdStop:
			return `{"ok":true,"stopped":[{"app":"/app","pid":42,"exit_code":null,"signal":15}]}`
		case cmdStatus:
			return `{"ok":true,"child":{"app":"/app","pid":42,"started_at":"2026-10-13T10:00:00+00:00","runner":"python","stopping":false},` +
				`"last":{"app":"/app","pid":41,"exit_code":1,"signal":null,"ended_at":"2026-10-13T09:59:00+00:00"},"stale":false,` +
				`"preload":{"modules":["numpy"]},"version":"1.0","uptime_s":3.5}`
		default:
			return `{"ok":true,"stale":true}`
		}
	})
	client := New(fake.socket, time.Second)
	ctx := context.Background()

	t.Run("start with the app environment", func(t *testing.T) {
		resp, err := client.Start(ctx, testApp, map[string]string{"FOO": "bar"})
		require.NoError(t, err)
		assert.Equal(t, request{Cmd: cmdStart, App: testApp, Env: map[string]string{"FOO": "bar"}}, <-fake.requests)
		assert.Equal(t, 42, resp.PID)
		assert.Equal(t, "python", resp.Runner)
		assert.Equal(t, Timings{StopMs: 12.5, ForkMs: 3}, resp.Timings)
		assert.Equal(t, time.Date(2026, 10, 13, 10, 0, 0, 5e8, time.UTC), resp.StartedAt.UTC())
	})

	t.Run("start without environment sends none", func(t *testing.T) {
		_, err := client.Start(ctx, testApp, nil)
		require.NoError(t, err)
		assert.Equal(t, request{Cmd: cmdStart, App: testApp}, <-fake.requests)
	})

	t.Run("stop", func(t *testing.T) {
		resp, err := client.Stop(ctx)
		require.NoError(t, err)
		assert.Equal(t, request{Cmd: cmdStop}, <-fake.requests)
		require.Len(t, resp.Stopped, 1)
		assert.Nil(t, resp.Stopped[0].ExitCode)
		assert.Equal(t, 15, *resp.Stopped[0].Signal)
	})

	t.Run("status", func(t *testing.T) {
		resp, err := client.Status(ctx)
		require.NoError(t, err)
		<-fake.requests
		require.NotNil(t, resp.Child)
		assert.Equal(t, Child{App: testApp, PID: 42, StartedAt: resp.Child.StartedAt, Runner: "python"}, *resp.Child)
		require.NotNil(t, resp.Last)
		assert.Equal(t, 41, resp.Last.PID)
		assert.Equal(t, 1, *resp.Last.ExitCode)
		assert.Nil(t, resp.Last.Signal)
		assert.Equal(t, "1.0", resp.Version)
		assert.InDelta(t, 3.5, resp.UptimeS, 0)
	})
}

func TestClientStatusWithoutChild(t *testing.T) {
	fake := newFakeLauncher(t, func(string) string { return `{"ok":true,"child":null,"last":null,"version":"1.0"}` })
	resp, err := New(fake.socket, time.Second).Status(context.Background())
	require.NoError(t, err)
	assert.Nil(t, resp.Child)
	assert.Nil(t, resp.Last)
}

func TestClientErrors(t *testing.T) {
	t.Run("a refusal carries its code", func(t *testing.T) {
		fake := newFakeLauncher(t, func(string) string {
			return `{"ok":false,"error":{"code":"busy","message":"the launcher is shutting down"}}`
		})
		_, err := New(fake.socket, time.Second).Start(context.Background(), testApp, nil)
		var launcherErr *Error
		require.ErrorAs(t, err, &launcherErr)
		assert.Equal(t, CodeBusy, launcherErr.Code)
		assert.Equal(t, "the launcher is shutting down", launcherErr.Message)
		assert.True(t, IsCode(err, CodeBusy))
	})

	t.Run("a stale container", func(t *testing.T) {
		fake := newFakeLauncher(t, func(string) string {
			return `{"ok":false,"error":{"code":"stale","message":"the dependencies of /app changed"}}`
		})
		_, err := New(fake.socket, time.Second).Start(context.Background(), testApp, nil)
		assert.True(t, IsCode(err, CodeStale))
	})

	t.Run("no socket", func(t *testing.T) {
		_, err := New(filepath.Join(t.TempDir(), "missing.sock"), time.Second).Status(context.Background())
		require.ErrorContains(t, err, "cannot reach the app launcher")
		assert.False(t, IsCode(err, CodeInternal))
	})

	t.Run("no answer within the timeout", func(t *testing.T) {
		fake := newFakeLauncher(t, func(string) string {
			time.Sleep(time.Second)
			return ""
		})
		started := time.Now()
		_, err := New(fake.socket, 100*time.Millisecond).Status(context.Background())
		require.ErrorContains(t, err, "no answer from the app launcher")
		assert.Less(t, time.Since(started), 500*time.Millisecond)
	})

	t.Run("invalid answer", func(t *testing.T) {
		fake := newFakeLauncher(t, func(string) string { return "not json" })
		_, err := New(fake.socket, time.Second).Status(context.Background())
		require.ErrorContains(t, err, "invalid answer")
	})
}

func TestClientAcceptsOnlyTheApprovedPeer(t *testing.T) {
	fake := newFakeLauncher(t, func(string) string { return `{"ok":true}` })
	previous := peerPID
	peerPID = func(net.Conn) (int, error) { return 4242, nil }
	t.Cleanup(func() { peerPID = previous })

	var seen int
	approve := func(pid int) error { seen = pid; return nil }
	_, err := New(fake.socket, time.Second).WithPeer(approve).Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4242, seen, "the check sees the peer pid")
	<-fake.requests

	refuse := func(int) error { return errors.New("not the launcher") }
	_, err = New(fake.socket, time.Second).WithPeer(refuse).WithTimeout(time.Second).Status(context.Background())
	require.ErrorIs(t, err, ErrPeer)
	assert.Equal(t, 0, len(fake.requests), "a refused peer is sent nothing")
}

// eventsLauncher answers an events request with what serve writes on the connection, which
// stays open until serve returns.
func eventsLauncher(t *testing.T, serve func(conn net.Conn)) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "l")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, SocketName)
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if line, err := bufio.NewReader(conn).ReadString('\n'); err == nil && line == `{"cmd":"events"}`+"\n" {
					serve(conn)
				}
			}()
		}
	}()
	return socket
}

const (
	idleState    = `{"ok":true,"child":null,"last":null}` + "\n"
	runningState = `{"ok":true,"child":{"app":"/app","pid":42,"started_at":"2026-10-13T10:00:00+00:00","runner":"python","stopping":false},"last":null}` + "\n"
	endedState   = `{"ok":true,"child":null,"last":{"app":"/app","pid":42,"exit_code":3,"signal":null,"ended_at":"2026-10-13T10:01:00+00:00"}}` + "\n"
)

func TestClientEventsTellTheStateOfTheAppNowAndAtEachChange(t *testing.T) {
	socket := eventsLauncher(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte(idleState))
		// Later than the timeout of the client, which bounds the first state only
		time.Sleep(150 * time.Millisecond)
		_, _ = conn.Write([]byte(runningState + endedState))
	})

	var states []AppState
	err := New(socket, 100*time.Millisecond).Events(context.Background(), func(state AppState) { states = append(states, state) })
	require.ErrorContains(t, err, "connection closed", "the events end when the launcher goes")
	require.Len(t, states, 3)
	assert.Equal(t, AppState{}, states[0])
	require.NotNil(t, states[1].Child)
	assert.Equal(t, 42, states[1].Child.PID)
	assert.Nil(t, states[1].Last)
	assert.Nil(t, states[2].Child)
	require.NotNil(t, states[2].Last)
	assert.Equal(t, 3, *states[2].Last.ExitCode)
}

func TestClientEventsEndWithTheContext(t *testing.T) {
	gone := make(chan struct{})
	socket := eventsLauncher(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte(runningState))
		// The launcher sees its subscriber leave
		_, _ = conn.Read(make([]byte, 1))
		close(gone)
	})

	ctx, cancel := context.WithCancel(context.Background())
	err := New(socket, time.Second).Events(ctx, func(AppState) { cancel() })
	require.ErrorIs(t, err, context.Canceled)
	select {
	case <-gone:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the connection stayed open")
	}
}

func TestClientEventsRefusedByALauncherWithoutThem(t *testing.T) {
	fake := newFakeLauncher(t, func(string) string {
		return `{"ok":false,"error":{"code":"bad_request","message":"unknown cmd: 'events'"}}`
	})
	err := New(fake.socket, time.Second).Events(context.Background(), func(AppState) { require.FailNow(t, "no state was told") })
	assert.True(t, IsCode(err, CodeBadRequest))
	assert.Equal(t, request{Cmd: cmdEvents}, <-fake.requests)
}
