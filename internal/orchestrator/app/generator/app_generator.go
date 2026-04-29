// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package generator

import (
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

const templateRoot = "app_template"

//go:embed all:app_template
var fsApp embed.FS

func GenerateApp(basePath *paths.Path, app app.AppDescriptor, skipSketch bool) error {
	if err := basePath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}
	if !skipSketch {
		if err := generateSketch(basePath); err != nil {
			return fmt.Errorf("failed to create sketch: %w", err)
		}
	}

	if err := generatePython(basePath); err != nil {
		return fmt.Errorf("failed to create python: %w", err)
	}
	if err := generateApp(basePath, app); err != nil {
		return fmt.Errorf("failed to create app.yaml: %w", err)
	}

	return nil
}

func generateApp(basePath *paths.Path, appDesc app.AppDescriptor) error {
	generateAppYaml := func(basePath *paths.Path, app app.AppDescriptor) error {
		appYamlTmpl := template.Must(
			template.New("app.yaml").
				Funcs(template.FuncMap{"joinInts": formatPorts}).
				ParseFS(fsApp, path.Join(templateRoot, "app.yaml.template")),
		)

		outputPath := basePath.Join("app.yaml")
		file, err := os.Create(outputPath.String())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath.String(), err)
		}
		defer file.Close()

		return appYamlTmpl.ExecuteTemplate(file, "app.yaml.template", app)
	}

	generateReadme := func(basePath *paths.Path, app app.AppDescriptor) error {
		readmeTmpl := template.Must(template.ParseFS(fsApp, path.Join(templateRoot, "README.md.template")))
		data := struct {
			Title       string
			Icon        string
			Description string
			Ports       string
		}{
			Title:       app.Name,
			Icon:        app.Icon,
			Description: app.Description,
		}

		if len(app.Ports) > 0 {
			data.Ports = "Available application ports: " + formatPorts(app.Ports)
		}

		outputPath := basePath.Join("README.md")
		file, err := os.Create(outputPath.String())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath.String(), err)
		}
		defer file.Close()

		return readmeTmpl.Execute(file, data)
	}

	copyRootFiles := func() error {
		fileList, err := fsApp.ReadDir(templateRoot)
		if err != nil {
			return fmt.Errorf("read template directory: %w", err)
		}
		for _, filePath := range fileList {
			if filePath.IsDir() {
				continue
			}
			if path.Ext(filePath.Name()) == ".template" {
				continue
			}

			srcPath := path.Join(templateRoot, filePath.Name())
			destPath := basePath.Join(filePath.Name())

			if err := func() error {
				srcFile, err := fsApp.Open(srcPath)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				destFile, err := destPath.Create()
				if err != nil {
					return fmt.Errorf("create %q file: %w", destPath, err)
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				return err
			}(); err != nil {
				return fmt.Errorf("copy file %s: %w", filePath.Name(), err)
			}
		}
		return nil
	}

	if err := copyRootFiles(); err != nil {
		slog.Warn("error copying root files for app %q: %w", appDesc.Name, err)
	}
	if err := generateReadme(basePath, appDesc); err != nil {
		slog.Warn("error generating readme for app %q: %w", appDesc.Name, err)
	}

	if err := generateAppYaml(basePath, appDesc); err != nil {
		return fmt.Errorf("generate app.yaml: %w", err)
	}

	return nil
}

func generatePython(basePath *paths.Path) error {
	templatePath := path.Join(templateRoot, "python", "main.py")
	sourceFile, err := fsApp.Open(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open python template: %w", err)
	}
	defer sourceFile.Close()

	pythonDir := basePath.Join("python")
	if err := pythonDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create python directory: %w", err)
	}

	destPath := pythonDir.Join("main.py")
	destFile, err := os.Create(destPath.String())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy template to %s: %w", destPath, err)
	}

	return nil
}

func generateSketch(basePath *paths.Path) error {
	baseSketchPath := basePath.Join("sketch")
	if err := baseSketchPath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create sketch directory: %w", err)
	}

	files, err := fsApp.ReadDir(path.Join(templateRoot, "sketch"))
	if err != nil {
		return fmt.Errorf("failed to read sketch template directory: %w", err)
	}

	for _, file := range files {
		sourcePath := path.Join(templateRoot, "sketch", file.Name())
		destPath := baseSketchPath.Join(file.Name())

		sourceFile, err := fsApp.Open(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to open template %s: %w", sourcePath, err)
		}
		defer sourceFile.Close()

		destFile, err := os.Create(destPath.String())
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, sourceFile); err != nil {
			return fmt.Errorf("failed to copy template to %s: %w", destPath, err)
		}
	}
	return nil
}

func formatPorts(ports []int) string {
	s := make([]string, len(ports))
	for i, v := range ports {
		s[i] = strconv.Itoa(v)
	}
	return strings.Join(s, ", ")
}

const brickTemplateRoot = "brick_template"

//go:embed all:brick_template
var fsBrick embed.FS

var ErrBrickAlreadyExists = fmt.Errorf("brick already exists")

func GenerateLocalBrick(basePath *paths.Path, id string, name string) error {
	brickDir := basePath.Join(id)
	if brickDir.Exist() {
		return fmt.Errorf("%w: %q", ErrBrickAlreadyExists, id)
	}

	if err := brickDir.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create bricks directory: %w", err)
	}

	type brickData struct {
		ID          string
		Name        string
		Description string
	}

	data := brickData{
		ID:   id,
		Name: name,
	}

	generateBrickConfig := func(brickDir *paths.Path, data brickData) error {
		configTmpl := template.Must(template.ParseFS(fsBrick, path.Join(brickTemplateRoot, "brick_config.yaml.template")))
		outputPath := brickDir.Join("brick_config.yaml")
		file, err := os.Create(outputPath.String())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath.String(), err)
		}
		defer file.Close()

		return configTmpl.Execute(file, data)
	}

	generateBrickReadme := func(brickDir *paths.Path, data brickData) error {
		readmeTmpl := template.Must(template.ParseFS(fsBrick, path.Join(brickTemplateRoot, "README.md.template")))

		outputPath := brickDir.Join("README.md")
		file, err := os.Create(outputPath.String())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath.String(), err)
		}
		defer file.Close()

		return readmeTmpl.Execute(file, data)
	}

	copyBrickStaticFiles := func(brickDir *paths.Path) error {
		fileList, err := fsBrick.ReadDir(brickTemplateRoot)
		if err != nil {
			return fmt.Errorf("read brick template directory: %w", err)
		}

		for _, filePath := range fileList {
			if filePath.IsDir() {
				continue
			}
			// Skip template files
			if strings.HasSuffix(filePath.Name(), ".template") {
				continue
			}

			srcPath := path.Join(brickTemplateRoot, filePath.Name())
			destPath := brickDir.Join(filePath.Name())

			if err := func() error {
				srcFile, err := fsBrick.Open(srcPath)
				if err != nil {
					return err
				}
				defer srcFile.Close()

				destFile, err := destPath.Create()
				if err != nil {
					return fmt.Errorf("create %q file: %w", destPath, err)
				}
				defer destFile.Close()

				_, err = io.Copy(destFile, srcFile)
				return err
			}(); err != nil {
				return fmt.Errorf("copy file %s: %w", filePath.Name(), err)
			}
		}
		return nil
	}

	if err := generateBrickConfig(brickDir, data); err != nil {
		return fmt.Errorf("failed to generate brick_config.yaml: %w", err)
	}

	if err := generateBrickReadme(brickDir, data); err != nil {
		return fmt.Errorf("failed to generate README.md: %w", err)
	}

	if err := copyBrickStaticFiles(brickDir); err != nil {
		slog.Warn("error copying static brick files", slog.String("brick", id), slog.Any("error", err))
	}

	return nil
}
