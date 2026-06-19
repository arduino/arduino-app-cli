// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dockerhandler provides a thin Docker API wrapper for running a container
// to completion and streaming its output.
package dockerhelper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.bug.st/f"
)

type RunOptions struct {
	Image  string
	Cmd    []string
	Binds  []string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

// Run creates, starts, and waits for a container to exit, streaming stdout and
// stderr to the provided writers. The container is always removed on return.
func Run(ctx context.Context, cli client.APIClient, opts RunOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	for _, bind := range opts.Binds {
		hostPath, _, _ := strings.Cut(bind, ":")
		if err := os.MkdirAll(hostPath, 0775); err != nil {
			slog.Warn("cannot pre-create bind mount directory", "path", hostPath, "err", err)
			continue
		}
	}

	pullResp, err := cli.ImagePull(ctx, opts.Image, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}
	if _, err := io.Copy(io.Discard, pullResp); err != nil {
		return fmt.Errorf("image pull read: %w", err)
	}
	pullResp.Close()

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: opts.Image,
			Cmd:   opts.Cmd,
			Env:   opts.Env,
			User:  getCurrentUser(),
		},
		&container.HostConfig{
			Binds:      opts.Binds,
			LogConfig:  container.LogConfig{Type: "none"},
			AutoRemove: true,
		},
		nil, nil, "",
	)
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}
	slog.Debug("creating container", "id", resp.ID, "image", opts.Image, "cmd", opts.Cmd, "env", opts.Env, "binds", opts.Binds)

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	attachResp, err := cli.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("container attach: %w", err)
	}
	defer attachResp.Close()
	now := time.Now()
	defer func() {
		slog.Warn("container finished", "id", resp.ID, "image", opts.Image, "cmd", opts.Cmd, "env", opts.Env, "binds", opts.Binds, "duration", time.Since(now))
	}()
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	if _, err := stdcopy.StdCopy(opts.Stdout, opts.Stderr, attachResp.Reader); err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("container wait: %w", err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			return fmt.Errorf("container exit error: %s", status.Error.Message)
		}
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
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
