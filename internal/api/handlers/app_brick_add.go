// This file implements POST /v1/apps/{appID}/bricks for adding a new brick instance to an app.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickAddRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AppLocalBrickAddResponse struct {
	ID string `json:"id"`
}

func HandleAppLocalBrickAdd(idProvider *app.IDProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		appPath := appId.ToPath()

		appLocal, err := app.Load(appPath)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		var req AppLocalBrickAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "name is required"})
			return
		}
		// Generate brick ID: lowercase, underscores, no spaces
		id := generateBrickID(req.Name)

		err = generator.GenerateAppLocalBrick(appPath, id, req.Name, req.Description)
		if err != nil {
			slog.Error("Failed to generate local brick", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to generate local brick"})
			return
		}

		appLocal.Descriptor.Bricks = append(appLocal.Descriptor.Bricks, app.Brick{ID: id})
		err = appLocal.Save()
		if err != nil {
			slog.Error("Failed to save app descriptor with new brick", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to update app descriptor"})
			return
		}
		render.EncodeResponse(w, http.StatusCreated, AppLocalBrickAddResponse{ID: id})
	}
}

func generateBrickID(name string) string {
	id := strings.ToLower(name)
	id = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	return id
}
