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
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.bug.st/f"
)

type RunOptions struct {
	Image        string
	Cmd          []string
	Binds        []string
	Env          []string
	Stdout       io.Writer
	Stderr       io.Writer
	LineCallback func(string)
}

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

	for _, bind := range opts.Binds {
		hostPath, _, _ := strings.Cut(bind, ":")
		if err := os.MkdirAll(hostPath, 0755); err != nil {
			slog.Warn("cannot pre-create bind mount directory", "path", hostPath, "err", err)
			continue
		}
		if info, err := os.Stat(filepath.Dir(hostPath)); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if err := os.Chown(hostPath, int(stat.Uid), int(stat.Gid)); err != nil {
					slog.Warn("cannot chown bind mount directory", "path", hostPath, "err", err)
				}
			}
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: opts.Image,
			Cmd:   opts.Cmd,
			Env:   opts.Env,
			User:  getCurrentUser(),
		},
		&container.HostConfig{
			Binds: opts.Binds,
			AutoRemove: true,
		},
		nil, nil, "",
	)
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}
	slog.Debug("creating container", "id", resp.ID, "image", opts.Image, "cmd", opts.Cmd, "env", opts.Env, "binds", opts.Binds)

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}

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

	inspect, err := cli.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		return fmt.Errorf("container inspect: %w", err)
	}
	if inspect.State.ExitCode != 0 {
		return fmt.Errorf("container exited with status %d", inspect.State.ExitCode)
	}

	return nil
}

func getCurrentUser() string {
	userInfo := f.Must(user.Current())
	uid := userInfo.Uid
	gid := userInfo.Gid

	// If exist use arduino group to avoid permission issue on files /var/lib/arduino-app-cli in.
	if gInfo, err := user.LookupGroup("arduino"); err == nil {
		gid = gInfo.Gid
	}

	return uid + ":" + gid
}
