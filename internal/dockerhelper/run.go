// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"go.bug.st/f"
	"golang.org/x/sync/errgroup"
)

// RunOptions is what a container is run with. Only Image is required.
type RunOptions struct {
	Image      string
	Entrypoint []string
	Cmd        []string
	Binds      []string
	Env        map[string]string
	Stdout     io.Writer
	Stderr     io.Writer
}

// Run is `docker run --rm`: the container is started, waited for and removed, and what
// it writes reaches the writers. A missing image is downloaded first.
func Run(ctx context.Context, docker client.APIClient, opts RunOptions) error {
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

	if err := ensureImage(ctx, docker, opts.Image); err != nil {
		return err
	}

	env := make([]string, 0, len(opts.Env)+1)
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	if _, set := opts.Env["HOME"]; !set {
		// The image's own HOME belongs to its user, and the container runs as the host's
		// user instead. A writable HOME lets python write its caches whatever the id is.
		env = append(env, "HOME=/tmp")
	}

	resp, err := docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:      opts.Image,
			Entrypoint: opts.Entrypoint,
			Cmd:        opts.Cmd,
			Env:        env,
			User:       getCurrentUser(),
		},
		HostConfig: &container.HostConfig{
			Binds:      opts.Binds,
			LogConfig:  container.LogConfig{Type: "none"},
			AutoRemove: true,
		},
	})
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}

	slog.Debug("creating container", "id", resp.ID, "image", opts.Image, "cmd", opts.Cmd, "env", opts.Env, "binds", opts.Binds)

	// AutoRemove is set, so the status comes with the removal: waiting for the container
	// to stop races the daemon removing it, and the exit code is lost.
	wait := docker.ContainerWait(ctx, resp.ID, client.ContainerWaitOptions{Condition: container.WaitConditionRemoved})
	statusCh, errCh := wait.Result, wait.Error

	attachResp, err := docker.ContainerAttach(ctx, resp.ID, client.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("container attach: %w", err)
	}
	defer attachResp.Close()

	if _, err := docker.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	slog.Debug("container launched", "id", resp.ID, "image", opts.Image, "launch_s", time.Since(launchStart).Seconds())

	// Stop the container on ctx cancel so the daemon EOFs the attach stream,
	// which unblocks StdCopy and fires errCh/statusCh.
	stopOnCancel := context.AfterFunc(ctx, func() {
		if _, err := docker.ContainerStop(context.Background(), resp.ID, client.ContainerStopOptions{}); err != nil {
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
			runErr = &ExitError{Code: status.StatusCode}
		}
	}

	// Wait for StdCopy to finish draining output.
	_ = g.Wait()

	return cmp.Or(ctx.Err(), runErr)
}

// ExitError is what Run returns for a container that exited non-zero.
type ExitError struct {
	Code int64
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("container exited with status %d", e.Code)
}

// IsExitError reports a container that exited non-zero, at any depth of the error.
func IsExitError(err error) bool {
	_, ok := errors.AsType[*ExitError](err)
	return ok
}

// ensureImage downloads what the board has not, with the disk check and the retry that
// every other pull has.
func ensureImage(ctx context.Context, docker client.APIClient, img string) error {
	if _, err := docker.ImageInspect(ctx, img); err == nil {
		return nil
	}
	return PullImages(ctx, docker, []string{img}, nil, nil)
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
