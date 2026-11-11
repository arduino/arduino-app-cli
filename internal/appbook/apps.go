// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package appbook is the app CRUD on disk.
package appbook

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	appgenerator "github.com/arduino/arduino-app-cli/internal/orchestrator/app/generator"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
	"github.com/arduino/go-paths-helper"
	"github.com/gosimple/slug"
	"go.bug.st/f"
	"gopkg.in/yaml.v2"
)

var (
	ErrAppAlreadyExists = fmt.Errorf("app already exists")
	ErrAppDoesntExists  = fmt.Errorf("app doesn't exist")
	ErrBadRequest       = fmt.Errorf("bad request")
	ErrAppRunning       = fmt.Errorf("app is running")
)

type ListAppResult struct {
	Apps       []AppInfo       `json:"apps"`
	BrokenApps []BrokenAppInfo `json:"broken_apps"`
}

type AppInfo struct {
	ID          appid.ID `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Status      Status   `json:"status,omitempty"`
	Example     bool     `json:"example"`
	Default     bool     `json:"default"`
	// Release tells an app installed from a release, which is frozen, from an app,
	// which is edited. ReleaseID is the release it comes from.
	Release      bool       `json:"release"`
	ReleaseID    string     `json:"release_id,omitempty"`
	ReleaseLabel string     `json:"release_label,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
}

type BrokenAppInfo struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type ListAppRequest struct {
	ShowExamples    bool
	ShowOnlyDefault bool // List only the default app (runs at startup)
	ShowApps        bool
	ShowReleases    bool // List the apps installed from a release, which are read-only
}

// ListApps lists the apps on disk, with no status: it is the catalog.
func ListApps(
	req ListAppRequest,
	idProvider *appid.Provider,
	bricksIndex *bricksindex.BricksIndex,
	cfg config.Configuration,
	platform platform.Platform,
) (ListAppResult, error) {
	// Get the default app to mark it in the list
	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	}

	// Retrieve all apps from the filesystem
	var pathsToExplore paths.PathList
	if req.ShowExamples || req.ShowOnlyDefault {
		pathsToExplore.AddAll(cfg.ExamplesDirs(platform))
		pathsToExplore.AddAll(cfg.ExamplesAdditionalDirs())
	}
	if req.ShowApps || req.ShowOnlyDefault {
		pathsToExplore.Add(cfg.AppsDir())
	}
	// The releases are apps, installed apart because they are read-only.
	if req.ShowReleases || req.ShowOnlyDefault {
		pathsToExplore.Add(cfg.ReleasesDir())
	}

	appPaths, err := app.FindAppsInFolders(pathsToExplore)
	if err != nil {
		slog.Error("unable to list apps", slog.String("error", err.Error()))
		return ListAppResult{}, err
	}

	// Go through all the known app paths, and create a result item for each.
	result := ListAppResult{Apps: []AppInfo{}, BrokenApps: []BrokenAppInfo{}}
	for _, file := range appPaths {
		app, err := app.Load(file)
		if err != nil {
			result.BrokenApps = append(result.BrokenApps, BrokenAppInfo{
				Name:  file.Base(),
				Error: fmt.Sprintf("unable to parse the app.yaml: %s", err.Error()),
			})
			continue
		}

		// Apply default-apps-only filter if requested
		isDefault := defaultApp != nil && defaultApp.FullPath.EqualsTo(app.FullPath)
		if req.ShowOnlyDefault && !isDefault {
			continue
		}

		// Get the app ID
		id, err := idProvider.IDFromPath(app.FullPath)
		if err != nil {
			return ListAppResult{}, fmt.Errorf("failed to get app ID from path %s: %w", file.String(), err)
		}

		// Filter example apps that use bricks incompatible with the board
		if bricksIndex != nil && id.IsExample() && !exampleCompatibleWithBricksIndex(app, bricksIndex) {
			continue
		}

		release, isRelease := app.GetRelease()

		result.Apps = append(result.Apps,
			AppInfo{
				ID:           id,
				Name:         app.Name,
				Description:  app.Descriptor.Description,
				Icon:         app.Descriptor.Icon,
				Example:      id.IsExample(),
				Default:      isDefault,
				Release:      isRelease,
				ReleaseID:    release.ID,
				ReleaseLabel: release.ReleaseLabel,
				CreatedAt:    release.CreatedAt,
			},
		)
	}

	return result, nil
}

type AppDetailedInfo struct {
	ID           appid.ID           `json:"id" required:"true" `
	Name         string             `json:"name" required:"true"`
	Path         string             `json:"path"`
	Description  string             `json:"description"`
	Icon         string             `json:"icon"`
	Status       Status             `json:"status,omitempty"`
	Example      bool               `json:"example"`
	Default      bool               `json:"default"`
	Release      bool               `json:"release"`
	ReleaseID    string             `json:"release_id,omitempty"`
	ReleaseLabel string             `json:"release_label,omitempty"`
	CreatedAt    *time.Time         `json:"created_at,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	Bricks       []AppDetailedBrick `json:"bricks,omitempty"`
}

type AppDetailedBrick struct {
	ID           string `json:"id" required:"true"`
	Name         string `json:"name" required:"true"`
	Category     string `json:"category,omitempty"`
	RequireModel bool   `json:"require_model"`
}

// AppDetails shows an app on disk. appsStatus maps an app path to its status, and returns nil when the status is unknown.
func AppDetails(
	ctx context.Context,
	userApp app.ArduinoApp,
	appsStatus func(context.Context) (map[string]Status, error),
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
) (AppDetailedInfo, error) {
	bricksIndex = bricksIndex.WithAppBricks(userApp.LocalBricks)
	var defaultAppPath string
	if defaultApp, err := GetDefaultApp(cfg); err != nil {
		slog.Warn("unable to get default app", slog.String("error", err.Error()))
	} else if defaultApp != nil {
		defaultAppPath = defaultApp.FullPath.String()
	}

	id, err := idProvider.IDFromPath(userApp.FullPath)
	if err != nil {
		return AppDetailedInfo{}, err
	}

	release, isRelease := userApp.GetRelease()

	// Retrieve the app status if available: an app that never ran is uninitialized
	appsStatusByPath, err := appsStatus(ctx)
	if err != nil {
		slog.Error("unable to get running app", slog.String("error", err.Error()))
	}
	var status Status
	if appsStatusByPath != nil {
		status = StatusUninitialized
		if s, ok := appsStatusByPath[userApp.FullPath.String()]; ok {
			status = s
		}
	}

	return AppDetailedInfo{
		ID:           id,
		Name:         userApp.Name,
		Path:         userApp.FullPath.String(),
		Description:  userApp.Descriptor.Description,
		Icon:         userApp.Descriptor.Icon,
		Status:       status,
		Example:      id.IsExample(),
		Default:      defaultAppPath == userApp.FullPath.String(),
		Release:      isRelease,
		ReleaseID:    release.ID,
		ReleaseLabel: release.ReleaseLabel,
		CreatedAt:    release.CreatedAt,
		Notes:        release.Notes,
		Bricks: f.Map(userApp.Descriptor.Bricks, func(b app.Brick) AppDetailedBrick {
			res := AppDetailedBrick{ID: b.ID}
			bi, found := bricksIndex.FindBrickByID(b.ID)
			if !found {
				slog.Warn("brick not found in bricks index", slog.String("id", b.ID), slog.String("app", userApp.FullPath.String()))
				return res
			}
			res.Name = bi.Name
			res.Category = bi.Category
			res.RequireModel = bi.RequireModel
			return res
		}),
	}, nil
}

type CreateAppRequest struct {
	Name        string
	Icon        string
	Description string
	Bricks      []string
	SkipSketch  bool
}

type CreateAppResponse struct {
	ID appid.ID `json:"id"`
}

func CreateApp(
	req CreateAppRequest,
	bricksIndex *bricksindex.BricksIndex,
	idProvider *appid.Provider,
	cfg config.Configuration,
) (CreateAppResponse, error) {
	if req.Name == "" {
		return CreateAppResponse{}, fmt.Errorf("app name cannot be empty")
	}

	appBricks := make([]app.Brick, 0, len(req.Bricks))
	for _, id := range req.Bricks {
		if _, found := bricksIndex.FindBrickByID(id); !found {
			return CreateAppResponse{}, fmt.Errorf("%w: brick %q not found", ErrBadRequest, id)
		}
		appBricks = append(appBricks, app.Brick{ID: id})
	}

	basePath, appExists := findAppPathByName(req.Name, cfg)
	if appExists {
		existingID, err := idProvider.IDFromPath(basePath)
		if err != nil {
			return CreateAppResponse{}, ErrAppAlreadyExists
		}
		return CreateAppResponse{}, fmt.Errorf("%w with id: %q", ErrAppAlreadyExists, existingID)
	}
	appName := req.Name
	newApp := app.AppDescriptor{
		Name:        appName,
		Description: req.Description,
		Ports:       []int{},
		Bricks:      appBricks,
		Icon:        req.Icon,
	}
	if err := newApp.IsValid(); err != nil {
		return CreateAppResponse{}, fmt.Errorf("%w: %v", app.ErrInvalidApp, err)
	}

	if err := appgenerator.GenerateApp(basePath, newApp, req.SkipSketch); err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to create app: %w", err)
	}
	fatomic.Sync()
	id, err := idProvider.IDFromPath(basePath)
	if err != nil {
		return CreateAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CreateAppResponse{ID: id}, nil
}

type CloneAppRequest struct {
	FromID appid.ID

	Name *string
	Icon *string
}

type CloneAppResponse struct {
	ID appid.ID `json:"id"`
}

func CloneApp(
	req CloneAppRequest,
	idProvider *appid.Provider,
	cfg config.Configuration,
) (response CloneAppResponse, cloneErr error) {
	originPath := req.FromID.ToPath()
	if !originPath.Exist() {
		return CloneAppResponse{}, ErrAppDoesntExists
	}
	if !originPath.Join("app.yaml").Exist() {
		return CloneAppResponse{}, app.ErrInvalidApp
	}

	var dstPath *paths.Path
	if req.Name != nil && *req.Name != "" {
		dstPath = cfg.AppsDir().Join(slug.Make(*req.Name))
		if dstPath.Exist() {
			return CloneAppResponse{}, ErrAppAlreadyExists
		}
	} else {
		for i := range 100 { // In case of name collision, we try up to 100 times.
			dstName := fmt.Sprintf("%s-copy%d", originPath.Base(), i)
			dstPath = cfg.AppsDir().Join(dstName)
			if !dstPath.Exist() {
				break
			}
		}
	}
	if err := dstPath.MkdirAll(); err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to create app directory: %w", err)
	}

	// In case something during the clone operation fails we remove the dst path
	defer func() {
		if cloneErr != nil {
			_ = dstPath.RemoveAll()
		}
	}()

	// The manifest stays behind: a copy of a release is an app, and is edited.
	list, err := originPath.ReadDir(paths.FilterOutNames(".cache", "data", app.ReleaseManifestFileName))
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to read app directory: %w", err)
	}
	for _, file := range list {
		if file.IsDir() {
			if err := file.CopyDirTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy directory: %w", err)
			}
		} else {
			if err := file.CopyTo(dstPath.Join(file.Base())); err != nil {
				return CloneAppResponse{}, fmt.Errorf("failed to copy file: %w", err)
			}
		}
	}

	if (req.Name != nil && *req.Name != "") || (req.Icon != nil && *req.Icon != "") {

		appYamlPath := dstPath.Join("app.yaml")
		if !appYamlPath.Exist() {
			return CloneAppResponse{}, fmt.Errorf("app.yaml not found in cloned app")
		}
		descriptor, err := app.ParseDescriptorFile(appYamlPath)
		if err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to parse app.yaml file: %w", err)
		}
		if req.Name != nil && *req.Name != "" {
			descriptor.Name = *req.Name
		}
		if req.Icon != nil && *req.Icon != "" {
			descriptor.Icon = *req.Icon
		}

		// TODO: implement MarshalYaml directly in the descriptor.
		newDescriptor, err := yaml.Marshal(descriptor)
		if err != nil {
			// TODO: should we consider this a fatal error, or we prefer to silently ignore the error?
			// Worst case, the optional fields will be the same as the source app.
			return CloneAppResponse{}, fmt.Errorf("failed to marshal app.yaml file: %w", err)
		}
		if err := appYamlPath.WriteFile(newDescriptor); err != nil {
			return CloneAppResponse{}, fmt.Errorf("failed to write app.yaml file: %w", err)
		}
	}
	fatomic.Sync()

	id, err := idProvider.IDFromPath(dstPath)
	if err != nil {
		return CloneAppResponse{}, fmt.Errorf("failed to get app id: %w", err)
	}
	return CloneAppResponse{ID: id}, nil
}

// DeleteApp removes the app folder.
func DeleteApp(app app.ArduinoApp) error {
	if err := app.FullPath.RemoveAll(); err != nil {
		return err
	}
	fatomic.Sync()
	return nil
}

type AppEditRequest struct {
	Name        *string
	Icon        *string
	Description *string
	Default     *bool
}

// EditApp edits the descriptor of an app, its folder name and the default app.
func EditApp(req AppEditRequest, editApp *app.ArduinoApp, cfg config.Configuration) error {
	if req.Name != nil && slug.Make(*req.Name) == "" {
		return fmt.Errorf("%w: invalid app name %q", app.ErrInvalidApp, *req.Name)
	}

	if req.Name != nil || req.Icon != nil || req.Description != nil {
		if err := editAppFolder(req, editApp, cfg); err != nil {
			return err
		}
	}

	if req.Default != nil {
		if err := editAppDefaults(editApp, *req.Default, cfg); err != nil {
			return fmt.Errorf("failed to edit app defaults: %w", err)
		}
	}
	return nil
}

const defaultAppFileName = "default.app"

func SetDefaultApp(app *app.ArduinoApp, cfg config.Configuration) error {
	defaultAppPath := cfg.DataDir().Join(defaultAppFileName)

	// Remove the default app file if the app is nil.
	if app == nil {
		err := defaultAppPath.Remove()
		if err != nil {
			slog.Warn("failed to remove default app file", slog.String("path", defaultAppPath.String()), slog.String("error", err.Error()))
			return nil
		}
		fatomic.Sync()
		return nil
	}

	return fatomic.WriteFile(defaultAppPath.String(), []byte(app.FullPath.String()), os.FileMode(0644))
}

func GetDefaultApp(cfg config.Configuration) (*app.ArduinoApp, error) {
	defaultAppFilePath := cfg.DataDir().Join(defaultAppFileName)
	if !defaultAppFilePath.Exist() {
		return nil, nil
	}

	defaultAppPath, err := defaultAppFilePath.ReadFile()
	if err != nil {
		return nil, err
	}
	defaultAppPath = bytes.TrimSpace(defaultAppPath)
	if len(defaultAppPath) == 0 {
		// If the file is empty, we remove it
		slog.Warn("default app file is empty", slog.String("path", string(defaultAppPath)))
		_ = defaultAppFilePath.Remove()
		return nil, nil
	}

	app, err := app.Load(paths.New(string(defaultAppPath)))
	if err != nil {
		// If the app is not valid, we remove the file
		slog.Warn("default app is not valid", slog.String("path", string(defaultAppPath)), slog.String("error", err.Error()))
		_ = defaultAppFilePath.Remove()
		return nil, err
	}

	return &app, nil
}

type ConfigResponse struct {
	Directories  ConfigDirectories `json:"directories"`
	PythonRunner string            `json:"python_runner"`
}

type ConfigDirectories struct {
	Data     string `json:"data"`
	Apps     string `json:"apps"`
	Examples string `json:"examples"`
}

func GetConfig(cfg config.Configuration) ConfigResponse {
	return ConfigResponse{
		Directories: ConfigDirectories{
			Data: cfg.DataDir().String(),
			Apps: cfg.AppsDir().String(),
			// FIXME: ExamplesDir() is not the right directory, we should return the examples by board directory.
			// Examples: cfg.ExamplesDir().String(),
		},
		PythonRunner: cfg.RunnerVersion,
	}
}

// exampleCompatibleWithBricksIndex returns true if all built-in bricks referenced by the app
// are present in the given bricks index. Local bricks bundled with the app are always treated
// as compatible and are skipped in the check.
func exampleCompatibleWithBricksIndex(a app.ArduinoApp, idx *bricksindex.BricksIndex) bool {
	localBrickIDs := make(map[string]struct{}, len(a.LocalBricks))
	for _, lb := range a.LocalBricks {
		localBrickIDs[lb.ID] = struct{}{}
	}
	for _, brick := range a.Descriptor.Bricks {
		if _, isLocal := localBrickIDs[brick.ID]; isLocal {
			continue
		}
		if _, ok := idx.FindBrickByID(brick.ID); !ok {
			return false
		}
	}
	return true
}

func editAppFolder(req AppEditRequest, editApp *app.ArduinoApp, cfg config.Configuration) error {
	editable, err := editApp.GetAsEditable()
	if err != nil {
		return err
	}

	if req.Name != nil {
		editable.Descriptor.Name = *req.Name
	}
	if req.Icon != nil {
		editable.Descriptor.Icon = *req.Icon
	}
	if req.Description != nil {
		editable.Descriptor.Description = *req.Description
	}

	if err := editable.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrInvalidApp, err)
	}

	if req.Name != nil {
		newPath, err := findRenamePath(editable.FullPath, slug.Make(*req.Name))
		if err != nil {
			return err
		}
		if !newPath.EqualsTo(editable.FullPath) {
			oldPath := editable.FullPath
			if err := editable.FullPath.Rename(newPath); err != nil {
				return fmt.Errorf("failed to rename app path: %w", err)
			}
			fatomic.Sync()
			editable.FullPath = newPath
			// The folder is already renamed: the edit must complete, so the client gets the new ID.
			if err := moveDefaultApp(oldPath, editApp, cfg); err != nil {
				slog.Warn("failed to update default app after renaming", slog.String("path", newPath.String()), slog.String("error", err.Error()))
			}
		}
		editable.Name = editable.Descriptor.Name
	}

	return editable.Save()
}

func findRenamePath(appPath *paths.Path, folderName string) (*paths.Path, error) {
	candidate := appPath.Parent().Join(folderName)
	for i := 1; i <= 100; i++ {
		if candidate.EqualsTo(appPath) || candidate.NotExist() {
			return candidate, nil
		}
		candidate = appPath.Parent().Join(fmt.Sprintf("%s-%d", folderName, i))
	}
	return nil, ErrAppAlreadyExists
}

func moveDefaultApp(oldPath *paths.Path, movedApp *app.ArduinoApp, cfg config.Configuration) error {
	defaultAppFilePath := cfg.DataDir().Join(defaultAppFileName)
	if defaultAppFilePath.NotExist() {
		return nil
	}
	defaultAppPath, err := defaultAppFilePath.ReadFile()
	if err != nil {
		return err
	}
	if string(bytes.TrimSpace(defaultAppPath)) != oldPath.String() {
		return nil
	}
	return SetDefaultApp(movedApp, cfg)
}

func editAppDefaults(userApp *app.ArduinoApp, isDefault bool, cfg config.Configuration) error {
	if isDefault {
		if err := SetDefaultApp(userApp, cfg); err != nil {
			return fmt.Errorf("failed to set default app: %w", err)
		}
		return nil
	}

	defaultApp, err := GetDefaultApp(cfg)
	if err != nil {
		return fmt.Errorf("failed to get default app: %w", err)
	}

	// No default app set, nothing to unset.
	if defaultApp == nil {
		return nil
	}

	// Unset only if the current default is the same as the app being edited.
	if defaultApp.FullPath.String() == userApp.FullPath.String() {
		if err := SetDefaultApp(nil, cfg); err != nil {
			return fmt.Errorf("failed to unset default app: %w", err)
		}
	}
	return nil
}

func findAppPathByName(name string, cfg config.Configuration) (*paths.Path, bool) {
	appFolderName := slug.Make(name)
	basePath := cfg.AppsDir().Join(appFolderName)
	return basePath, basePath.Exist()
}
