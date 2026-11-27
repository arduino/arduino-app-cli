// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/docker/cli/cli/command"
	"go.bug.st/downloader/v3"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/dockerhelper"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type brickInstalled struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	AlreadyInstalled bool   `json:"already_installed"`
}

// HandleInstallBrick installs a release of a third-party brick from the bricks index: the one of
// the version query parameter or, without it, the latest one. It must support the board.
func HandleInstallBrick(dockerClient command.Cli, cfg config.Configuration, servicesIndex *servicesindex.ServicesIndex, plat platform.Platform, downloadConfig downloader.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("brickID")
		if err := bricksindex.ValidateBrickID(id); err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
			return
		}
		var version *semver.Version
		if query := r.URL.Query(); query.Has("version") {
			v, err := bricksindex.ParseBrickVersion(query.Get("version"))
			if err != nil {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: fmt.Sprintf("invalid version: %v", err)})
				return
			}
			version = v
		}

		// A release that cannot be installed has to be a status, so it is looked up before the stream opens.
		// The warning is sent once the stream opens.
		var warning string
		index, err := bricksindex.LoadOrUpdateRemoteIndex(r.Context(), downloadConfig, cfg.BricksIndexURL, cfg.BricksIndexDir(), nil,
			func(msg string) { warning = msg })
		if err != nil {
			slog.Error("bricks index not available", "error", err)
			render.EncodeResponse(w, http.StatusServiceUnavailable, models.ErrorResponse{Details: fmt.Sprintf("bricks index not available: %v", err)})
			return
		}
		brick, release, err := index.FindBrickRelease(id, version, plat)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, bricksindex.ErrBrickNotFound), errors.Is(err, bricksindex.ErrReleaseNotFound):
				status = http.StatusNotFound
			case errors.Is(err, bricksindex.ErrUnsupportedBoard):
				status = http.StatusBadRequest
			}
			render.EncodeResponse(w, status, models.ErrorResponse{Details: err.Error()})
			return
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		ref := id + "@" + release.Version.String()
		stream := &brickInstallStream{sse: sseStream}
		if warning != "" {
			stream.message(warning)
		}
		installed, err := orchestrator.InstallBrick(r.Context(), dockerClient, downloadConfig, cfg.BricksDir(), servicesIndex, brick, release, plat,
			func(downloaded, total int64) { stream.progress(ref, downloaded, total) }, stream.message, stream.progress)
		if err != nil {
			slog.Error("unable to install brick", "brick", ref, "error", err)
			stream.sendError(err)
			return
		}
		sseStream.Send(render.SSEEvent{Type: render.DoneEvent, Data: brickInstalled{
			ID:               id,
			Version:          release.Version.String(),
			AlreadyInstalled: installed.AlreadyInstalled,
		}})
	}
}

// brickInstallStream sends the events of a brick install as SSE, in the shape of the events of
// the model installs. After the stream opens, a failure is an event, not an HTTP status.
type brickInstallStream struct {
	sse sseSender
}

func (s *brickInstallStream) message(msg string) {
	s.sse.Send(render.SSEEvent{Type: render.MessageEvent, Data: sseLog{Message: msg}})
}

func (s *brickInstallStream) progress(name string, current, total int64) {
	var progress float32
	if total > 0 {
		progress = float32(current) / float32(total) * 100
	}
	s.sse.Send(render.SSEEvent{Type: render.ProgressEvent, Data: sseProgress{
		Name: name, Current: current, Total: total, Progress: progress,
	}})
}

func (s *brickInstallStream) sendError(err error) {
	if errors.Is(err, bricksindex.ErrInsufficientStorage) || errors.Is(err, dockerhelper.ErrOutOfSpace) {
		s.sse.SendError(render.SSEErrorData{Code: "insufficient_storage", Message: err.Error()})
		return
	}
	s.sse.SendError(render.SSEErrorData{Code: render.InternalServiceErr, Message: err.Error()})
}
