package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"
	"github.com/arduino/arduino-app-cli/internal/render"
)

type AppLocalBrickCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AppLocalBrickCreateResponse struct {
	ID string `json:"id"`
}

func HandleAppLocalBrickCreate(idProvider *app.IDProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appId, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}

		a, err := app.Load(appId.ToPath())
		if err != nil {
			slog.Error("Unable to load the app", slog.String("error", err.Error()), slog.String("path", appId.String()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the app"})
			return
		}

		var req AppLocalBrickCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Failed to decode request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid request body"})
			return
		}
		if req.Name == "" {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "name is required"})
			return
		}

		id := generateBrickID(req.Name)

		err = generator.GenerateLocalBrick(a, id, req.Name, req.Description)
		if err != nil {
			if errors.Is(err, generator.ErrBrickAlreadyExists) {
				render.EncodeResponse(w, http.StatusConflict, models.ErrorResponse{Details: fmt.Sprintf("a brick with the same id '%s' already exists", id)})
				return
			}
			slog.Error("Failed to generate local brick", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to generate local brick"})
			return
		}

		idx := slices.IndexFunc(a.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == id })
		if idx == -1 {
			a.Descriptor.Bricks = append(a.Descriptor.Bricks, app.Brick{ID: id})
			err = a.Save()
			if err != nil {
				slog.Error("Failed to save app descriptor with new brick", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to update app descriptor"})
				return
			}
		}

		render.EncodeResponse(w, http.StatusCreated, AppLocalBrickCreateResponse{ID: id})
	}
}

func generateBrickID(name string) string {
	id := strings.ToLower(name)
	id = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	return id
}
