// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arduino/go-paths-helper"
	"github.com/docker/cli/cli/command"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/servicesindex"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleAppStart(
	dockerCli command.Cli,
	provisioner *orchestrator.Provision,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	servicesIndex *servicesindex.ServicesIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
	platform platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}

		var verbose bool
		if r.URL.Query().Has("verbose") || r.URL.Query().Get("verbose") == queryTrue {
			verbose = true
		}

		app, err := app.Load(id.ToPath())
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		// The whole body is read before the SSE stream opens: net/http may close the
		// connection when a handler writes its response before it has read the request.
		var prebuiltSketch *paths.Path
		if ok, _ := strconv.ParseBool(r.URL.Query().Get("prebuilt_sketch")); ok {
			fwFile, err := savePrebuiltSketch(r)
			if err != nil {
				slog.Error("unable to read the prebuilt sketch", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "missing or unreadable file parameter"})
				return
			}
			defer func() { _ = fwFile.Remove() }()
			prebuiltSketch = fwFile
		}

		sseStream, err := render.NewSSEStream(r.Context(), w)
		if err != nil {
			slog.Error("Unable to create SSE stream", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to create SSE stream"})
			return
		}
		defer sseStream.Close()

		type progress struct {
			Name     string  `json:"name"`
			Progress float32 `json:"progress"`
		}
		type log struct {
			Message string `json:"message"`
		}
		// An editable app is prepared by its start, downloading what it is missing; a
		// release is not, so a start of one that is not prepared fails as not prepared.
		prepare := !app.IsRelease()
		if err := orchestrator.StartApp(r.Context(), dockerCli, provisioner, modelsIndex, bricksIndex, servicesIndex, app, cfg, platform, verbose, prepare, prebuiltSketch, func(item orchestrator.StreamMessage) {
			switch item.GetType() {
			case orchestrator.ProgressType:
				sseStream.Send(render.SSEEvent{Type: render.ProgressEvent, Data: progress(*item.GetProgress())})
			case orchestrator.InfoType:
				sseStream.Send(render.SSEEvent{Type: render.MessageEvent, Data: log{Message: item.GetData()}})
			}
		}); err != nil {
			code := render.InternalServiceErr
			switch {
			case errors.Is(err, orchestrator.ErrNotPrepared):
				code = render.NotPreparedErr
			case errors.Is(err, orchestrator.ErrBadRequest):
				code = render.BadRequestErr
			}
			sseStream.SendError(render.SSEErrorData{
				Code:    code,
				Message: err.Error(),
			})
		}
	}
}

// savePrebuiltSketch writes the "file" form field of r to a temp file.
func savePrebuiltSketch(r *http.Request) (*paths.Path, error) {
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tempFile, err := paths.MkTempFile(nil, "sketch-prebuilt-*.fw")
	if err != nil {
		return nil, err
	}
	tempFilePath := paths.NewFromFile(tempFile)
	_, err = io.Copy(tempFile, file)
	if closeErr := tempFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = tempFilePath.Remove()
		return nil, err
	}
	return tempFilePath, nil
}
