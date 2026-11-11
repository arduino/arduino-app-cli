// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/appbook"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/arduino-app-cli/internal/render"
)

// Hooks run before the CRUD: they read and stop what runs the apps.
// A nil status map means the status is unknown.
type Hooks struct {
	AppsStatus   func(ctx context.Context) (map[string]appbook.Status, error)
	BeforeEdit   func(ctx context.Context, req appbook.AppEditRequest, a app.ArduinoApp) error
	BeforeDelete func(ctx context.Context, a app.ArduinoApp) error
}

// NoHooks do nothing: the apps have no status.
var NoHooks = Hooks{
	AppsStatus:   func(context.Context) (map[string]appbook.Status, error) { return nil, nil },
	BeforeEdit:   func(context.Context, appbook.AppEditRequest, app.ArduinoApp) error { return nil },
	BeforeDelete: func(context.Context, app.ArduinoApp) error { return nil },
}

type AppListResponse struct {
	Apps       []appbook.AppInfo       `json:"apps" description:"List of applications"`
	BrokenApps []appbook.BrokenAppInfo `json:"broken_apps,omitempty" description:"List of applications that are broken and couldn't be parsed"`
}

type EditRequest struct {
	Name        *string `json:"name" example:"My Awesome App" description:"application name"`
	Icon        *string `json:"icon" example:"💻" description:"application icon"`
	Description *string `json:"description" example:"This is my awesome app" description:"application description"`
	Default     *bool   `json:"default"`
}

func HandleAppList(
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
	platform platform.Platform,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := appbook.ListApps(ListAppRequest(r), idProvider, bricksIndex, cfg, platform)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, AppListResponse{Apps: res.Apps, BrokenApps: res.BrokenApps})
	}
}

func HandleAppDetails(
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
	appsStatus func(context.Context) (map[string]appbook.Status, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, a, ok := loadApp(w, r, idProvider)
		if !ok {
			return
		}
		res, err := appbook.AppDetails(r.Context(), a, appsStatus, bricksIndex, idProvider, cfg)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppDetailsEdits(
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
	hooks Hooks,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, appToEdit, ok := loadApp(w, r, idProvider)
		if !ok {
			return
		}

		var editRequest EditRequest
		if err := json.NewDecoder(r.Body).Decode(&editRequest); err != nil {
			slog.Error("Unable to decode the request body", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "invalid request"})
			return
		}
		onlyDefault := editRequest.Description == nil && editRequest.Icon == nil && editRequest.Name == nil
		switch {
		case id.IsRelease() && !onlyDefault:
			render.EncodeResponse(w, http.StatusForbidden, render.ErrorResponse{Details: "you can patch just the default field for a release"})
			return
		case id.IsExample() && !onlyDefault:
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "you can patch just the default field for example apps"})
			return
		}
		req := appbook.AppEditRequest{
			Default:     editRequest.Default,
			Name:        editRequest.Name,
			Icon:        editRequest.Icon,
			Description: editRequest.Description,
		}

		err := func() error {
			if err := hooks.BeforeEdit(r.Context(), req, appToEdit); err != nil {
				return err
			}
			return appbook.EditApp(req, &appToEdit, cfg)
		}()
		if err != nil {
			switch {
			case errors.Is(err, app.ErrInvalidApp):
				slog.Error("Unable to edit the app 1", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: err.Error()})
			case errors.Is(err, appbook.ErrAppRunning):
				render.EncodeResponse(w, http.StatusConflict, render.ErrorResponse{Details: "stop the app before renaming it"})
			case errors.Is(err, appbook.ErrAppAlreadyExists):
				slog.Error("The name is already in use.", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{
					Details: fmt.Sprintf("the name %q is already in use", *editRequest.Name),
				})
			default:
				slog.Error("Unable to edit the app ", slog.String("error", err.Error()))
				render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to edit the app"})
			}
			return
		}

		res, err := appbook.AppDetails(r.Context(), appToEdit, hooks.AppsStatus, bricksIndex, idProvider, cfg)
		if err != nil {
			slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to find the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, res)
	}
}

func HandleAppDelete(idProvider *appid.Provider, beforeDelete func(context.Context, app.ArduinoApp) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := idProvider.IDFromBase64(r.PathValue("appID"))
		if err != nil {
			render.EncodeResponse(w, http.StatusPreconditionFailed, render.ErrorResponse{Details: "invalid id"})
			return
		}
		if id.IsExample() {
			render.EncodeResponse(w, http.StatusBadRequest, render.ErrorResponse{Details: "cannot delete example"})
			return
		}
		_, a, ok := loadApp(w, r, idProvider)
		if !ok {
			return
		}

		if err := beforeDelete(r.Context(), a); err != nil {
			slog.Error("Unable to delete the app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to delete the app"})
			return
		}
		if err := appbook.DeleteApp(a); err != nil {
			slog.Error("Unable to delete the app", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to delete the app"})
			return
		}
		render.EncodeResponse(w, http.StatusOK, nil)
	}
}

// ListAppRequest is the catalog the filter query asks for: everything if it is missing.
func ListAppRequest(r *http.Request) appbook.ListAppRequest {
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		return appbook.ListAppRequest{ShowExamples: true, ShowApps: true, ShowReleases: true}
	}
	filters := strings.Split(strings.TrimSpace(filter), ",")
	return appbook.ListAppRequest{
		ShowExamples:    slices.Contains(filters, "examples"),
		ShowOnlyDefault: slices.Contains(filters, "default"),
		ShowApps:        slices.Contains(filters, "apps"),
		ShowReleases:    slices.Contains(filters, "releases"),
	}
}

// loadApp loads the app of the appID path value, or answers the error and returns false.
func loadApp(w http.ResponseWriter, r *http.Request, idProvider *appid.Provider) (appid.ID, app.ArduinoApp, bool) {
	id, err := idProvider.IDFromBase64(r.PathValue("appID"))
	if err != nil {
		render.EncodeResponse(w, http.StatusPreconditionFailed, render.ErrorResponse{Details: "invalid id"})
		return appid.ID{}, app.ArduinoApp{}, false
	}
	a, err := app.Load(id.ToPath())
	if err != nil {
		slog.Error("Unable to parse the app.yaml", slog.String("error", err.Error()), slog.String("path", id.String()))
		render.EncodeResponse(w, http.StatusInternalServerError, render.ErrorResponse{Details: "unable to find the app"})
		return appid.ID{}, app.ArduinoApp{}, false
	}
	return id, a, true
}
