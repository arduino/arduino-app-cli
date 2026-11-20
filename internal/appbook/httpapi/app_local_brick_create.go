// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickCreateRequest struct {
	Name string `json:"name"`
}

type AppLocalBrickCreateResponse struct {
	ID string `json:"id"`
}

func HandleAppLocalBrickCreate(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, render.ErrorResponse{Details: "invalid app id"})
			return
		}
		a, ok := LoadEditableApp(w, appId)
		if !ok {
			return
		}

		var req AppLocalBrickCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "name is required"})
			return
		}

		id, err := generateBrickID(req.Name)
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: err.Error()})
			return
		}

		err = generator.GenerateLocalBrick(a.GetBricksPath(), id, req.Name)
		if err != nil {
			if errors.Is(err, generator.ErrBrickAlreadyExists) {
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: fmt.Sprintf("a brick with the same id '%s' already exists", id)})
				return
			}
			slog.Error("Failed to generate local brick", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "failed to generate local brick"})
			return
		}

		render.EncodeResponse(w, http.StatusCreated, AppLocalBrickCreateResponse{ID: id})
	}
}

var brickIDRegexp = regexp.MustCompile(`[^a-z0-9]+`)

func generateBrickID(name string) (string, error) {
	if strings.Contains(name, ".") {
		return "", errors.New("brick name cannot contain '.' character")
	}
	if strings.Contains(name, ":") {
		return "", errors.New("brick name cannot contain ':' character")
	}

	id := strings.ToLower(name)
	id = brickIDRegexp.ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	if id == "" {
		return "", errors.New("brick name must contain at least one alphanumeric character")
	}
	return id, nil
}
