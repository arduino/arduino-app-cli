package handlers

import (
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/pkg/parser"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

func HandleAppSetDefault(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
	if id == "" {
		render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
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

	if err := orchestrator.SetDefaultApp(&app); err != nil {
		render.EncodeResponse(w, http.StatusInternalServerError, "unable to set the default app")
		return
	}
	render.EncodeResponse(w, http.StatusOK, nil)
}

func HandleDeleteDefault(w http.ResponseWriter, r *http.Request, id orchestrator.ID) {
	if id == "" {
		render.EncodeResponse(w, http.StatusPreconditionFailed, "id must be set")
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

	defaultApp, err := orchestrator.GetDefaultApp()
	if err != nil {
		render.EncodeResponse(w, http.StatusInternalServerError, "unable to get the default app")
		return
	}
	if defaultApp == nil || *defaultApp.FullPath != *app.FullPath {
		render.EncodeResponse(w, http.StatusBadRequest, "the app is not the default app")
		return
	}

	if err := orchestrator.SetDefaultApp(nil); err != nil {
		render.EncodeResponse(w, http.StatusInternalServerError, "unable to set the default app")
		return
	}
	render.EncodeResponse(w, http.StatusOK, nil)
}
