// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package handlers

import (
	"net/http"
	"strconv"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleSketchAddLibrary(idProvider *app.IDProvider) http.HandlerFunc {
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
		app, err := app.Load(id.ToPath().String())

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
			render.EncodeResponse(w, http.StatusCreated, SketchAddLibraryResponse{
				AddedLibraries: addedLibs,
			})
			return
		}
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchAddLibraryResponse struct {
	AddedLibraries []orchestrator.LibraryReleaseID `json:"libraries"`
}

func HandleSketchRemoveLibrary(idProvider *app.IDProvider) http.HandlerFunc {
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
		app, err := app.Load(id.ToPath().String())
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		libRef, err := orchestrator.ParseLibraryReleaseID(r.PathValue("libRef"))
		if err != nil {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "unable to parse library reference"})
			return
		}

		if removedLib, err := orchestrator.RemoveSketchLibrary(r.Context(), app, libRef); err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to remove sketch library"})
			return
		} else {
			render.EncodeResponse(w, http.StatusOK, SketchRemoveLibraryResponse{
				RemovedLibraries: []orchestrator.LibraryReleaseID{removedLib},
			})
			return
		}
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchRemoveLibraryResponse struct {
	RemovedLibraries []orchestrator.LibraryReleaseID `json:"libraries"`
}

func HandleSketchListLibraries(idProvider *app.IDProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid id"})
			return
		}
		app, err := app.Load(id.ToPath().String())
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		libraries, err := orchestrator.ListSketchLibraries(r.Context(), app)
		if err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to clone app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, SketchListLibraryResponse{
			Libraries: libraries,
		})
	}
}

// NOTE: this is only to generate the openapi docs.
type SketchListLibraryResponse struct {
	Libraries []orchestrator.LibraryReleaseID `json:"libraries"`
}
