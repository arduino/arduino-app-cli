// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dockerhandler provides a thin Docker API wrapper for running a container
// to completion and streaming its output.
package dockerhandler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// RunOptions configures a one-shot container run.
type RunOptions struct {
	Image        string
	Cmd          []string
	Binds        []string     // "host-path:container-path" volume mounts
	Env          []string     // "KEY=VALUE" pairs
	Stdout       io.Writer    // nil defaults to io.Discard; ignored when LineCallback is set
	Stderr       io.Writer    // nil defaults to io.Discard
	LineCallback func(string) // if set, called once per trimmed non-empty stdout line
}

// lineWriter buffers container stdout and fires a callback for every complete line.
type lineWriter struct {
	buf      []byte
	callback func(string)
}

func (w *lineWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx == -1 {
			break
		}
		line := string(bytes.TrimSpace(w.buf[:idx]))
		w.buf = w.buf[idx+1:]
		if line != "" {
			w.callback(line)
		}
	}
	return len(b), nil
}

// Run creates, starts, and waits for a container to exit, streaming stdout and
// stderr to the provided writers. The container is always removed on return.
//
// ContainerLogs with Follow:true is used instead of ContainerAttach because
// the logging driver fully buffers output before streaming it — fast-exiting
// containers can close the raw attach connection mid-frame, producing spurious
// "short write" errors.
func Run(ctx context.Context, cli client.APIClient, opts RunOptions) error {
	switch {
	case opts.LineCallback != nil:
		opts.Stdout = &lineWriter{callback: opts.LineCallback}
	case opts.Stdout == nil:
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: opts.Image,
			Cmd:   opts.Cmd,
			Env:   opts.Env,
		},
		&container.HostConfig{
			Binds: opts.Binds,
		},
		nil, nil, "",
	)

	slog.Debug("creating container", "id", resp.ID, "image", opts.Image, "cmd", opts.Cmd, "env", opts.Env, "binds", opts.Binds)

	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}
	defer cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}) //nolint:errcheck

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}

	// Follow:true keeps the stream open until the container exits, then closes
	// it cleanly — stdcopy.StdCopy returns nil on EOF.
	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("container logs: %w", err)
	}
	defer out.Close()

	if _, err := stdcopy.StdCopy(opts.Stdout, opts.Stderr, out); err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	// Stream is closed, container has exited — inspect is safe to call.
	inspect, err := cli.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		return fmt.Errorf("container inspect: %w", err)
	}
	if inspect.State.ExitCode != 0 {
		return fmt.Errorf("container exited with status %d", inspect.State.ExitCode)
	}

	return nil
}
