// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/fatomic"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

const maxDescriptionLength = 150

// ArduinoApp holds all the files composing an app
type ArduinoApp struct {
	Name           string
	MainPythonFile *paths.Path
	mainSketchPath *paths.Path
	FullPath       *paths.Path // FullPath is the path to the App folder
	LocalBricks    []bricksindex.Brick
	Descriptor     AppDescriptor
}

// Load creates an App instance by reading all the files composing an app and grouping them
// by file type.
func Load(appPath *paths.Path) (ArduinoApp, error) {
	if appPath == nil {
		return ArduinoApp{}, errors.New("empty app path")
	}

	exist, err := appPath.IsDirCheck()
	if err != nil {
		return ArduinoApp{}, fmt.Errorf("app path is not valid: %w", err)
	}
	if !exist {
		return ArduinoApp{}, fmt.Errorf("app path must be a directory: %s", appPath)
	}
	appPath, err = appPath.Abs()
	if err != nil {
		return ArduinoApp{}, fmt.Errorf("cannot get absolute path for app: %w", err)
	}

	if !IsValidFolderName(appPath.Base()) {
		return ArduinoApp{}, fmt.Errorf("app folder name %q is not valid: use only alphanumeric, underscores, dashes and spaces", appPath.Base())
	}

	descriptorFile := appPath.Join("app.yaml")
	if !descriptorFile.Exist() {
		return ArduinoApp{}, errors.New("descriptor app.yaml file missing from app")
	}

	app := ArduinoApp{
		FullPath:   appPath,
		Descriptor: AppDescriptor{},
	}

	desc, err := ParseDescriptorFile(app.GetDescriptorPath())
	if err != nil {
		return app, err
	}
	app.Descriptor = desc
	app.Name = desc.Name

	if app.Descriptor.Description == "" {
		description, err := app.getAppDescriptionFromReadme()
		if err != nil {
			slog.Warn("cannot extract app description from README.md", "error", err)
		} else {
			app.Descriptor.Description = description
		}
	}

	app.MainPythonFile = appPath.Join("python", "main.py")
	if !app.MainPythonFile.Exist() {
		return app, errors.New("main python file missing from app")
	}

	sketchPath := appPath.Join("sketch")
	if sketchPath.IsDir() {
		sketchIno := sketchPath.Join("sketch.ino")
		sketchYaml := sketchPath.Join("sketch.yaml")

		if sketchIno.Exist() || sketchYaml.Exist() {
			if !sketchIno.Exist() || !sketchYaml.Exist() {
				return app, fmt.Errorf("sketch folder is incomplete: both sketch.ino and sketch.yaml are required")
			}
		}
		app.mainSketchPath = sketchPath
	}

	if appPath.Join("bricks").Exist() {
		app.LocalBricks = loadBricksFromFolder(appPath.Join("bricks"))
	}

	return app, nil
}

func IsValidFolderName(s string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_ -]*$`, s)
	return matched
}

func (a *ArduinoApp) GetSketchPath() (*paths.Path, bool) {
	if a == nil || a.mainSketchPath == nil {
		return nil, false
	}
	return a.mainSketchPath, true
}

// GetDescriptorPath returns the path to the app descriptor file (app.yaml)
func (a *ArduinoApp) GetDescriptorPath() *paths.Path {
	descriptorFile := a.FullPath.Join("app.yaml")
	return descriptorFile
}

var ErrInvalidApp = fmt.Errorf("invalid app")

func (a *ArduinoApp) Save() error {
	if err := a.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidApp, err)
	}
	if err := a.writeApp(); err != nil {
		return err
	}
	return nil
}

func (a *ArduinoApp) writeApp() error {
	descriptorPath := a.GetDescriptorPath()
	if descriptorPath == nil {
		return errors.New("app descriptor file path is not set")
	}

	out, err := yaml.Marshal(a.Descriptor)
	if err != nil {
		return fmt.Errorf("cannot marshal app descriptor: %w", err)
	}

	if err := fatomic.WriteFile(descriptorPath.String(), out, os.FileMode(0644)); err != nil {
		return fmt.Errorf("cannot write app descriptor file: %w", err)
	}
	return nil
}

func (a *ArduinoApp) SketchBuildPath() *paths.Path {
	return a.FullPath.Join(".cache", "sketch")
}

func (a *ArduinoApp) GetBricksPath() *paths.Path {
	return a.FullPath.Join("bricks")
}

func (a *ArduinoApp) ProvisioningStateDir() *paths.Path {
	return a.FullPath.Join(".cache")
}

func (a *ArduinoApp) AppComposeFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join("app-compose.yaml")
}

func (a *ArduinoApp) AppComposeOverrideFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join("app-compose-overrides.yaml")
}

func (a *ArduinoApp) getAppDescriptionFromReadme() (string, error) {
	readmePath := a.FullPath.Join("README.md")
	if !readmePath.Exist() {
		return "", fmt.Errorf("README.md not found in app directory")
	}

	f, err := readmePath.Open()
	if err != nil {
		return "", fmt.Errorf("error reading README.md: %w", err)
	}
	defer f.Close()
	description := extractFirstParagraph(f)
	return truncateDescription(description, maxDescriptionLength), nil
}

func extractFirstParagraph(source io.Reader) string {
	scanner := bufio.NewScanner(source)
	var lines []string
	inFence := false

	for scanner.Scan() {
		line := scanner.Text()
		if reFence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		if reSpaces.MatchString(line) ||
			reHeader.MatchString(line) ||
			reSetext.MatchString(line) ||
			reList.MatchString(line) ||
			reQuote.MatchString(line) ||
			reIndent.MatchString(line) {
			if len(lines) > 0 {
				break
			}
			continue
		}

		clean := cleanInlineMarkdown(line)
		if clean == "" {
			continue
		}

		lines = append(lines, clean)
	}

	return strings.Join(lines, " ")
}

var (
	// Block-level regex
	reHeader = regexp.MustCompile(`^#{1,6}\s+`)           // Matches ATX-style headings (lines starting with 1-6 # characters)
	reSetext = regexp.MustCompile(`^\s*(=+|-+)\s*$`)      // Matches Setext-style headings (underlines with === or ---)
	reList   = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s+`) // Matches unordered (-, *, +) or ordered (1., 2., etc.) list items
	reQuote  = regexp.MustCompile(`^>\s+`)                // Matches blockquotes starting with >
	reFence  = regexp.MustCompile("^```")                 // Matches fenced code block start/end (```)
	reIndent = regexp.MustCompile(`^\s{4,}`)              // Matches indented code blocks (4+ spaces)

	// Inline-level regex
	reBold        = regexp.MustCompile(`\*\*(.*?)\*\*`)               // Matches bold text (**text**)
	reItalic      = regexp.MustCompile(`\*(.*?)\*`)                   // Matches italic text (*text*)
	reCode        = regexp.MustCompile("`([^`]*)`")                   // Matches inline code (`code`)
	reLink        = regexp.MustCompile(`\[(.*?)\]\(.*?\)`)            // Matches links [text](url), keeps only the text
	reLinkedImage = regexp.MustCompile(`\[\!\[.*?\]\(.*?\)\]\(.*?\)`) // Matches linked images [![alt](img)](url)
	reImage       = regexp.MustCompile(`!\[.*?\]\(.*?\)`)             // Matches images ![alt](img)
	reMultiSpace  = regexp.MustCompile(`\s+`)                         // Matches multiple spaces/newlines to normalize
	reSpaces      = regexp.MustCompile(`^\s*$`)
)

func cleanInlineMarkdown(s string) string {
	s = reLinkedImage.ReplaceAllString(s, "")
	s = reImage.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reBold.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1")
	s = reCode.ReplaceAllString(s, "$1")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func truncateDescription(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	if i := strings.LastIndex(s, " "); i > 0 {
		return s[:i]
	}
	return s
}

func loadBricksFromFolder(dir *paths.Path) []bricksindex.Brick {
	if dir == nil || !dir.Exist() {
		slog.Debug("App does not contain a bricks folder, skipping loading app bricks", "path", dir)
		return nil
	}
	pathsList, err := dir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		return file.Join("brick_config.yaml").NotExist()
	}, paths.FilterDirectories())
	if err != nil {
		slog.Warn("error reading app bricks folder, skipping loading bricks", "err", err, "path", dir)
		return nil
	}
	bricks := []bricksindex.Brick{}
	for _, path := range pathsList {
		brick, err := load(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		if err := isValid(brick); err != nil {
			slog.Warn("Invalid local app brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return bricks
}

func isValid(brick bricksindex.Brick) error {
	if brick.ID == "" {
		return errors.New("brick ID is required")
	}
	// TODO: add other validation
	return nil
}

func load(brickPath *paths.Path) (b bricksindex.Brick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return bricksindex.Brick{}, fmt.Errorf("brick_config.yaml does not exist: %v", brickConfigPath)
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return bricksindex.Brick{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	brick := bricksindex.Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &brick); err != nil {
		return bricksindex.Brick{}, fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}
	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}
	brick.Source = "App"
	brick.FullPath = brickPath
	brick.ComposeFile = composeFile
	brick.ReadmeFile = brickPath.Join("README.md")
	brick.ExamplesPath = brickPath.Join("examples")
	brick.DocsAPIPath = brickPath.Join("docs/API.md")
	return brick, nil
}
