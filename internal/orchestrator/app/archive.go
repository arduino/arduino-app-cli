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

package app

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

	yaml "github.com/goccy/go-yaml"
)

func ExportAppZip(
	ctx context.Context,
	appTarget ArduinoApp,
	includeData bool,
) ([]byte, string, error) {

	appName := strings.ToLower(strings.ReplaceAll(appTarget.Name, " ", "-"))
	if appName == "" {
		appName = "app-export"
	}
	filename := fmt.Sprintf("%s.zip", appName)
	zipBytes, err := zipAppToBuffer(appTarget.FullPath.String(), includeData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create zip archive: %w", err)
	}
	return zipBytes, filename, nil
}

func zipAppToBuffer(sourcePath string, includeData bool) ([]byte, error) {
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

		header.Name = filepath.ToSlash(relPath)
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
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
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

func ReadAppDescriptorFromZip(r *zip.Reader) (AppDescriptor, error) {
	var descriptor AppDescriptor

	for _, f := range r.File {
		if f.Name == "app.yaml" || f.Name == "app.yml" {
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
func ValidateAppZipContent(r *zip.Reader) error {
	hasAppYaml := false
	hasMainPy := false

	hasSketchFolder := false
	hasIno := false
	hasSketchYaml := false

	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)

		if name == "app.yaml" || name == "app.yml" {
			hasAppYaml = true
		}
		if name == "python/main.py" {
			hasMainPy = true
		}

		if strings.HasPrefix(name, "sketch/") {
			hasSketchFolder = true
			if strings.HasSuffix(name, ".ino") {
				hasIno = true
			}
			if strings.HasSuffix(name, ".yaml") {
				hasSketchYaml = true
			}
		}
	}

	if !hasAppYaml {
		return fmt.Errorf(" missing app.yaml")
	}
	if !hasMainPy {
		return fmt.Errorf(" missing python/main.py")
	}

	if hasSketchFolder {
		if !hasIno {
			return fmt.Errorf(" sketch folder present but missing .ino file")
		}
		if !hasSketchYaml {
			return fmt.Errorf("sketch folder present but missing .yaml file")
		}
	}

	return nil
}
