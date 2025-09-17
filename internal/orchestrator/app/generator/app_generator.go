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

type Opts int

const (
	None       Opts = 0
	SkipSketch Opts = 1 << iota
	SkipPython
)

//go:embed app_template
var fsApp embed.FS

func GenerateApp(basePath *paths.Path, app app.AppDescriptor, options Opts) error {
	if err := basePath.MkdirAll(); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}
	isSkipSketchSet := options&SkipSketch != 0
	isSkipPythonSet := options&SkipPython != 0
	if !isSkipSketchSet {
		if err := generateSketch(basePath); err != nil {
			return fmt.Errorf("failed to create sketch: %w", err)
		}
	}
	if !isSkipPythonSet {
		if err := generatePython(basePath); err != nil {
			return fmt.Errorf("failed to create python: %w", err)
		}
	}
	if err := generateReadme(basePath, app); err != nil {
		slog.Warn("error generating readme for app %q: %w", app.Name, err)
	}
	if err := generateAppYaml(basePath, app); err != nil {
		return fmt.Errorf("failed to create app content: %w", err)
	}

	return nil
}

func generateAppYaml(basePath *paths.Path, app app.AppDescriptor) error {
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

func generateReadme(basePath *paths.Path, app app.AppDescriptor) error {
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
