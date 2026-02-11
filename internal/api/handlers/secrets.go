package handlers

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/secrets"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleSecretsList(cfg config.Configuration, idProvider *app.IDProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}

		secrets, err := secrets.ListSecrets(cfg, appID)
		if err != nil {
			slog.Error("Unable to list secrets", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to list secret"})
			return
		}

		render.EncodeResponse(w, http.StatusOK, models.SecretListResponse{Secrets: secrets})
	})
}

func HandleSecretsUpdate(cfg config.Configuration, idProvider *app.IDProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		name := r.PathValue("secretName")

		value, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Warn("Failed to read request body", "error", err.Error())
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid body"})
			return
		}

		err = secrets.UpdateSecret(cfg, appID, name, value)
		if err != nil {
			slog.Error("Unable to update secret", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to update secret"})
			return
		}

		render.EncodeResponse(w, http.StatusNoContent, nil)
	})
}

func HandleSecretsDelete(cfg config.Configuration, idProvider *app.IDProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, models.ErrorResponse{Details: "invalid app id"})
			return
		}
		name := r.PathValue("secretName")

		err = secrets.RemoveSecret(cfg, appID, name)
		if err != nil {
			slog.Error("Unable to remove secret", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to remove secret"})
			return
		}

		render.EncodeResponse(w, http.StatusNoContent, nil)
	})
}
