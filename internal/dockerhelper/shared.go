// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

// What the rest of the project uses with or without docker: the errors it tells apart,
// the options of a run and the images of a project. Built for js/wasm too.

import (
	"errors"
	"fmt"
	"io"

	"github.com/compose-spec/compose-go/v2/types"
)

// ErrNoDocker is what an action that needs docker answers where there is none, as in a
// browser. What only reads the state of the apps answers as if no container existed.
var ErrNoDocker = errors.New("this needs docker, which runs on a board: there is none here")

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

// ErrOutOfSpace is what a download the board has no room for answers.
var ErrOutOfSpace = errors.New("not enough disk space to pull the docker image")

// ComposeImages is every image the project names, which is what a pull is given.
func ComposeImages(prj *types.Project) []string {
	images := make([]string, 0, len(prj.Services))
	for _, service := range prj.Services {
		if service.Image != "" {
			images = append(images, service.Image)
		}
	}
	return images
}

// ErrNetwork states that the network of the app was not created. A pool with no
// subnet left is one of the reasons, worded by the daemon.
var ErrNetwork = errors.New("failed to create the network of the app")
