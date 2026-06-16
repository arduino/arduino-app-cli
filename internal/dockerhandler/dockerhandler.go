// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dockerhandler provides a thin Docker API wrapper for running a container
// to completion and streaming its output.
package dockerhandler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

// Run creates, starts, and waits for a container to exit, streaming stdout and
// stderr to the provided writers. The container is always removed on return.
func Run(ctx context.Context, cli client.APIClient, opts RunOptions) error {
	if opts.LineCallback != nil {
		pr, pw := io.Pipe()
		opts.Stdout = pw
		var wg sync.WaitGroup
		wg.Go(func() {
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				if line := scanner.Text(); line != "" {
					opts.LineCallback(line)
				}
			}
		})
		defer func() { pw.Close(); wg.Wait() }()
	} else if opts.Stdout == nil {
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
