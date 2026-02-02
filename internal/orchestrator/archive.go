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

package orchestrator

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"path/filepath"
	"strings"
	"time"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func ExportAppZip(
	ctx context.Context,
	bricksIndex *bricksindex.BricksIndex,
	appTarget app.ArduinoApp,
	includeData bool,
) ([]byte, string, error) {

	appName := strings.ToLower(strings.ReplaceAll(appTarget.Name, " ", "-"))
	if appName == "" {
		appName = "app-export"
	}
	filename := fmt.Sprintf("%s.zip", appName)
	zipBytes, err := zipAppToBuffer(bricksIndex, appTarget.FullPath.String(), appName, includeData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create zip archive: %w", err)
	}
	return zipBytes, filename, nil
}

func zipAppToBuffer(bricksIndex *bricksindex.BricksIndex, sourcePath string, rootFolderName string, includeData bool) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	err := filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Always skip .cache
			if name == ".cache" {
				return filepath.SkipDir
			}
			// Conditionally skip data
			if !includeData && name == "data" {
				return filepath.SkipDir
			}
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(filepath.Join(rootFolderName, relPath))
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if d.Name() == "app.yaml" || d.Name() == "app.yml" { // nolint:goconst
			desc, err := app.ParseDescriptorFile(paths.New(path))
			if err != nil {
				return err
			}
			redactSecrets(bricksIndex, &desc)
			err = yaml.NewEncoder(writer).Encode(desc)
			return err
		} else {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		}
	})
	if err != nil {
		zipWriter.Close()
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func ImportAppFromZip(
	cfg config.Configuration,
	zipPath *paths.Path,
	idProvider *app.IDProvider,
	originalZipName string,
) (app.ID, error) {
	if zipPath == nil {
		return app.ID{}, fmt.Errorf("internal error: zipPath cannot be nil")
	}
	r, err := zip.OpenReader(zipPath.String())
	if err != nil {
		return app.ID{}, fmt.Errorf("unable to open zip archive: %w", err)
	}
	defer r.Close()

	rootPrefix, err := findZipRoot(&r.Reader)
	if err != nil {
		return app.ID{}, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	var rawAppName string
	if rootPrefix != "" {
		rawAppName = rootPrefix
	} else {
		rawAppName = strings.TrimSuffix(originalZipName, filepath.Ext(originalZipName))
	}

	if err := validateAppZipContent(&r.Reader, rootPrefix); err != nil {
		return app.ID{}, fmt.Errorf("%w:%v", ErrBadRequest, err)
	}

	appDescriptor, err := readAppDescriptorFromZip(&r.Reader, rootPrefix)
	if err != nil {
		return app.ID{}, fmt.Errorf("failed to read app.yaml: %w", err)
	}

	if strings.TrimSpace(appDescriptor.Name) == "" {
		return app.ID{}, fmt.Errorf("%w: app name is missing", ErrBadRequest)
	}

	finalDestPath, appExists := findAppPathByName(rawAppName, cfg)
	if appExists {
		suffix := time.Now().Format("-20060102-150405")
		newName := rawAppName + suffix
		finalDestPath, _ = findAppPathByName(newName, cfg)
	}

	tempDestDir, err := app.MkTmpAppDir(finalDestPath.Parent())
	if err != nil {
		return app.ID{}, fmt.Errorf("unable to create temp app directory: %w", err)
	}
	defer func() { _ = tempDestDir.RemoveAll() }()

	if err := extractZip(&r.Reader, tempDestDir.String(), rootPrefix); err != nil {
		return app.ID{}, err
	}

	if finalDestPath.Exist() {
		return app.ID{}, ErrAppAlreadyExists
	}

	if err := tempDestDir.Rename(finalDestPath); err != nil {
		return app.ID{}, fmt.Errorf("failed to finalize app import (swap): %w", err)
	}

	id, err := idProvider.IDFromPath(finalDestPath)
	if err != nil {
		return app.ID{}, err
	}

	return id, nil
}

func extractZip(r *zip.Reader, dest string, rootPrefix string) error {
	dest = filepath.Clean(dest) + string(os.PathSeparator)
	const maxFileSize = 100 * 1024 * 1024 // 100MB limit per file

	rootPrefixClean := filepath.FromSlash(rootPrefix)
	if rootPrefixClean == "." {
		rootPrefixClean = ""
	}

	for _, f := range r.File {
		zipName := filepath.Clean(filepath.FromSlash(f.Name))

		if rootPrefixClean != "" {
			if !strings.HasPrefix(zipName, rootPrefixClean) {
				continue
			}
			zipName = strings.TrimPrefix(zipName, rootPrefixClean)
			zipName = strings.TrimPrefix(zipName, string(os.PathSeparator))
		}

		if zipName == "" || zipName == "." {
			continue
		}

		fpath := filepath.Join(dest, zipName)
		if !strings.HasPrefix(fpath, dest) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", fpath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("create parent directory: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("create file %s: %w", fpath, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("unable to open entry %s: %w", f.Name, err)
		}

		lr := io.LimitReader(rc, maxFileSize+1)
		written, err := io.Copy(outFile, lr)

		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("write file %s: %w", fpath, err)
		}
		if written > maxFileSize {
			return fmt.Errorf("file %s too large", f.Name)
		}
	}

	return nil
}

func readAppDescriptorFromZip(r *zip.Reader, rootPrefix string) (app.AppDescriptor, error) {
	var descriptor app.AppDescriptor

	targetAppYaml := paths.New(rootPrefix, "app.yaml")
	targetAppYml := paths.New(rootPrefix, "app.yml")

	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)

		if name == targetAppYaml.String() || name == targetAppYml.String() {
			rc, err := f.Open()
			if err != nil {
				return descriptor, err
			}
			defer rc.Close()

			if err := yaml.NewDecoder(rc).Decode(&descriptor); err != nil {
				if errors.Is(err, io.EOF) {
					return descriptor, fmt.Errorf("app.yaml is empty")
				}
				return descriptor, err
			}
			return descriptor, nil
		}
	}
	return descriptor, fmt.Errorf("app.yaml not found in archive")
}

// TODO implement centralized app validator to use everywhere is needed
// validateAppZipContent checks for mandatory files respecting the rootPrefix
func validateAppZipContent(r *zip.Reader, rootPrefix string) error {
	hasAppYaml := false
	hasMainPy := false

	hasSketchFolder := false
	hasSketchIno := false
	hasSketchYaml := false

	targetAppYaml := paths.New(rootPrefix, "app.yaml")
	targetAppYml := paths.New(rootPrefix, "app.yml")
	targetMainPy := paths.New(rootPrefix, "python/main.py")

	targetSketchPrefix := paths.New(rootPrefix, "sketch").String() + "/"
	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)

		if name == targetAppYaml.String() || name == targetAppYml.String() {
			hasAppYaml = true
		}
		if name == targetMainPy.String() {
			hasMainPy = true
		}

		if strings.HasPrefix(name, targetSketchPrefix) {
			hasSketchFolder = true
			if name == paths.New(rootPrefix, "sketch/sketch.ino").String() {
				hasSketchIno = true
			}

			if name == paths.New(rootPrefix, "sketch/sketch.yaml").String() {
				hasSketchYaml = true
			}
		}
	}

	if !hasAppYaml {
		return errors.New("missing app.yaml")
	}
	if !hasMainPy {
		return errors.New("missing python/main.py")
	}

	if hasSketchFolder {
		if !hasSketchIno {
			return errors.New("sketch folder present but missing .ino file")
		}
		if !hasSketchYaml {
			return errors.New("sketch folder present but missing .yaml file")
		}
	}

	return nil
}

func redactSecrets(bricksindex *bricksindex.BricksIndex, desc *app.AppDescriptor) {
	for i := range desc.Bricks {
		brick := &desc.Bricks[i]

		brickDef, found := bricksindex.FindBrickByID(brick.ID)
		if !found {
			// Brick definition not found; skip secret redaction
			continue
		}

		for k, v := range brick.Variables {
			if v == "" {
				continue // Only redact if variable is set
			}
			vDef, ok := brickDef.GetVariable(k)
			if ok && vDef.Secret {
				brick.Variables[k] = ""
			}
		}
	}
}

func findZipRoot(r *zip.Reader) (string, error) {
	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)
		if filepath.Base(name) != "app.yaml" && filepath.Base(name) != "app.yml" {
			continue
		}
		slashCount := strings.Count(name, "/")

		if slashCount == 0 {
			return "", nil
		}

		if slashCount == 1 {
			return paths.New(name).Parent().String(), nil
		}

		// If slashCount > 1, file is too deeply nested
	}

	return "", fmt.Errorf("invalid archive structure: missing or misplaced app.yaml. Supported paths: archive.zip/app.yaml or archive.zip/<root_dir>/app.yaml")
}
