// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	properties "github.com/arduino/arduino-app-cli/internal/orchestrator/system_properties"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandlePropertyKeys(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		propertyList, err := properties.ReadPropertyKeys(cfg.DataDir().Join("properties.msgpack").String())
		if err != nil {
			slog.Error("Unable to retrieve list", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "unable to find the list"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, models.PropertyKeysResponse{Keys: propertyList})
	}
}

func HandlePropertyGet(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")

		property, found, err := properties.GetProperty(cfg.DataDir().Join("properties.msgpack").String(), key)
		if err != nil {
			if errors.Is(err, properties.ErrInvalidKey) {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
				return
			}
			slog.Error("Unable to retrieve property", "key", key, "error", err.Error())
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to retrieve property"})
			return
		}

		if !found {
			render.EncodeResponse(w, http.StatusNotFound, nil)
			return
		}

		render.EncodeByteResponse(w, http.StatusOK, property)
	}
}

func HandlePropertyUpsert(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Warn("Failed to read request body", "error", err.Error())
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "invalid body"})
			return
		}
		defer r.Body.Close()
		if len(reqBody) == 0 {
			render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: "body cannot be empty"})
			return
		}

		err = properties.UpsertProperty(cfg.DataDir().Join("properties.msgpack").String(), key, reqBody)
		if err != nil {
			if errors.Is(err, properties.ErrInvalidKey) {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
				return
			}
			slog.Error("Failed to upsert property", "key", key, "error", err.Error())
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to update property"})
			return
		}
		render.EncodeByteResponse(w, http.StatusOK, reqBody)
	}
}

func HandlePropertyDelete(cfg config.Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		found, err := properties.DeleteProperty(cfg.DataDir().Join("properties.msgpack").String(), key)
		if err != nil {
			if errors.Is(err, properties.ErrInvalidKey) {
				render.EncodeResponse(w, http.StatusBadRequest, models.ErrorResponse{Details: err.Error()})
				return
			}
			slog.Error("Failed to delete property", "key", key, "error", err.Error())
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "failed to delete property"})
			return
		}
		if !found {
			render.EncodeResponse(w, http.StatusNotFound, nil)
			return
		}
		render.EncodeResponse(w, http.StatusNoContent, nil)
	}
}
