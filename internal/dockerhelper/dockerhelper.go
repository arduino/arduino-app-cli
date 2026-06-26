// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dockerhandler provides a thin Docker API wrapper for running a container
// to completion and streaming its output.
package dockerhelper

import (
	"cmp"
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
	"golang.org/x/sync/errgroup"
)

type RunOptions struct {
	Image  string
	Cmd    []string
	Binds  []string
	Env    map[string]string
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

	launchStart := time.Now()

	if err := ensureImage(ctx, cli, opts.Image); err != nil {
		return err
	}

	env := make([]string, 0, len(opts.Env))
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: opts.Image,
			Cmd:   opts.Cmd,
			Env:   env,
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

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	slog.Debug("container launched", "id", resp.ID, "image", opts.Image, "launch_s", time.Since(launchStart).Seconds())

	// Stop the container on ctx cancel so the daemon EOFs the attach stream,
	// which unblocks StdCopy and fires errCh/statusCh.
	stopOnCancel := context.AfterFunc(ctx, func() {
		if err := cli.ContainerStop(context.Background(), resp.ID, container.StopOptions{}); err != nil {
			slog.Debug("container stop on cancel failed", "id", resp.ID, "err", err)
		}
	})
	defer stopOnCancel()

	execStart := time.Now()
	defer func() {
		slog.Debug("container finished", "id", resp.ID, "image", opts.Image, "exec_s", time.Since(execStart).Seconds())
	}()

	// Read output in a goroutine so it doesn't block waiting for the container.
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err := stdcopy.StdCopy(opts.Stdout, opts.Stderr, attachResp.Reader)
		return err
	})

	var runErr error
	select {
	case err := <-errCh:
		if err != nil {
			runErr = fmt.Errorf("container wait: %w", err)
		}
	case status := <-statusCh:
		if status.Error != nil {
			runErr = fmt.Errorf("container exit error: %s", status.Error.Message)
		} else if status.StatusCode != 0 {
			runErr = fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	// Wait for StdCopy to finish draining output.
	_ = g.Wait()

	return cmp.Or(ctx.Err(), runErr)
}

func ensureImage(ctx context.Context, cli client.APIClient, img string) error {
	if _, err := cli.ImageInspect(ctx, img); err == nil {
		return nil
	}
	slog.Debug("image not found locally, pulling", "image", img)
	// TODO: we should stream the pull progress to the caller.
	pullResp, err := cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}
	defer pullResp.Close()
	if _, err := io.Copy(io.Discard, pullResp); err != nil {
		return fmt.Errorf("image pull read: %w", err)
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
