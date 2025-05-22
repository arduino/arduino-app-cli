package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleAppPropertyChanges(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
	if id == "" {
		render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
		return
	}

	type patchRequest struct {
		Default *bool `json:"default"`
	}
	var req patchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.EncodeResponse(w, http.StatusBadRequest, "unable to decode request")
		return
	}
	if req == (patchRequest{}) {
		render.EncodeResponse(w, http.StatusBadRequest, "request body must not be empty")
		return
	}

	appPath, err := id.ToPath()
	if err != nil {
		render.EncodeResponse(w, http.StatusPreconditionFailed, "invalid id")
		return
	}
	app, err := parser.Load(appPath.String())
	if err != nil {
		render.EncodeResponse(w, http.StatusInternalServerError, "unable to parse the app")
		return
	}

	if *req.Default {
		if err := orchestrator.SetDefaultApp(&app); err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to set the default app")
			return
		}
	} else {
		if err := orchestrator.SetDefaultApp(nil); err != nil {
			render.EncodeResponse(w, http.StatusInternalServerError, "unable to set the default app")
			return
		}
	}
	render.EncodeResponse(w, http.StatusOK, nil)
}
