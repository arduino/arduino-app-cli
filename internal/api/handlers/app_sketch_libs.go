// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/render"

	"go.bug.st/f"
)

func HandleSketchAddLibrary(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "cannot alter examples"})
			return
		}
		app, err := app.Load(id.ToPath())

		// Get query param addDeps (default false)
		addDeps, _ := strconv.ParseBool(r.URL.Query().Get("add_deps"))

		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}
		libRef, err := orchestrator.ParseLibraryReleaseID(r.PathValue("libRef"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to parse library reference"})
			return
		}
		if addedLibs, err := orchestrator.AddSketchLibrary(r.Context(), app, libRef, addDeps); err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to add sketch library: " + err.Error()})
			return
		} else {
			render.EncodeResponse(w, http.StatusOK, SketchAddLibraryResponse{
				AddedLibraries: f.Map(addedLibs, (orchestrator.LibraryReleaseID).String),
			})
			return
		}
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchAddLibraryResponse struct {
	AddedLibraries []string `json:"libraries"`
}

func HandleSketchRemoveLibrary(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "cannot alter examples"})
			return
		}
		app, err := app.Load(id.ToPath())
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		libRef, err := orchestrator.ParseLibraryReleaseID(r.PathValue("libRef"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to parse library reference"})
			return
		}

		// Get query param addDeps (default false)
		removeDeps, _ := strconv.ParseBool(r.URL.Query().Get("remove_deps"))
		if removedLibs, err := orchestrator.RemoveSketchLibrary(r.Context(), app, libRef, removeDeps); err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to remove sketch library: " + err.Error()})
			return
		} else {
			render.EncodeResponse(w, http.StatusOK, SketchRemoveLibraryResponse{
				RemovedLibraries: f.Map(removedLibs, (orchestrator.LibraryReleaseID).String),
			})
			return
		}
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchRemoveLibraryResponse struct {
	RemovedLibraries []string `json:"libraries"`
}

func HandleSketchListLibraries(idProvider *appid.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		app, err := app.Load(id.ToPath())
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		allLibraries, err := orchestrator.ListSketchLibraries(r.Context(), app)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to list sketch libraries: " + err.Error()})
			return
		}

		libs := f.Filter(allLibraries, func(l orchestrator.LibraryReleaseID) bool { return !l.IsDependency })
		deps := f.Filter(allLibraries, func(l orchestrator.LibraryReleaseID) bool { return l.IsDependency })
		render.EncodeResponse(w, http.StatusOK, SketchListLibraryResponse{
			Libraries:    f.Map(libs, (orchestrator.LibraryReleaseID).String),
			Dependencies: f.Map(deps, (orchestrator.LibraryReleaseID).String),
		})
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchListLibraryResponse struct {
	Libraries    []string `json:"libraries"`
	Dependencies []string `json:"dependencies"`
}
