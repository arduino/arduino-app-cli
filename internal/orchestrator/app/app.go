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
	"github.com/arduino/arduino-app-cli/internal/platform"
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

var (
	ErrInvalidApp = fmt.Errorf("invalid app")
	// ErrReleaseReadOnly is what every change of an installed release gets: it runs
	// what a build froze, and changing it would make it something else.
	ErrReleaseReadOnly = errors.New("the app is installed from a release and cannot be changed")
)

func (a *ArduinoApp) Save() error {
	if _, isRelease := a.GetRelease(); isRelease {
		return ErrReleaseReadOnly
	}
	if err := a.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidApp, err)
	}
	if err := a.writeApp(); err != nil {
		return err
	}
	return nil
}

// SaveSecrets writes the descriptor back when only a secret changed. It is the one
// change an app installed from a release takes: its frozen template references the
// secrets, so the board that runs the app is where the values are set.
func (a *ArduinoApp) SaveSecrets() error {
	if err := a.Descriptor.IsValid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidApp, err)
	}
	return a.writeApp()
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

// The templates are exported because the resolve step also writes them outside of an
// app folder, when building a release.
const (
	MainTemplateFileName     = "app-compose.tmpl.yaml"
	OverrideTemplateFileName = "app-compose-overrides.tmpl.yaml"
	// PrebuildDirName is what a release ships beside the app it is built from: the
	// compose files and the python env, which the install copies as the .cache.
	PrebuildDirName = "prebuild"
	// ReleaseManifestFileName is the manifest a release is built with, at the root of
	// the archive and of the app installed from it: an app that holds it runs what a
	// build froze, and what a start would generate is already in .cache.
	ReleaseManifestFileName = "release.yaml"
)

// ReleaseManifestSchema is the layout of the manifest, not the version of the app: an
// older release must stay readable by a newer cli.
const ReleaseManifestSchema = 1

// Release is what the manifest says of the release an app comes from.
type Release struct {
	Schema  int    `yaml:"schema"`
	Version string `yaml:"version"`
	// Target is the board the release is built for, gated on at install and at start.
	Target string `yaml:"target"`
	// ID is the release folder name, so it is read from the path and never written.
	ID string `yaml:"-"`
}

// GetRelease reads the manifest: an app that holds one runs what a release froze.
func (a *ArduinoApp) GetRelease() (Release, bool) {
	manifest := a.FullPath.Join(ReleaseManifestFileName)
	content, err := manifest.ReadFile()
	if err != nil {
		return Release{}, false
	}
	var release Release
	if err := yaml.Unmarshal(content, &release); err != nil {
		slog.Warn("cannot read the release manifest of the app", "path", manifest, "error", err)
	}
	release.ID = a.FullPath.Base()
	return release, true
}

// ReleaseBricks is the brick definitions a release ships in its .cache, which are the
// ones the app was built with. The index of the board is not the one that built the
// release, so it may hold neither the brick nor the same definition of it. Only the
// config of a brick is read from here, so no board fact is resolved.
func (a *ArduinoApp) ReleaseBricks() (*bricksindex.BricksIndex, error) {
	return bricksindex.Load(platform.Platform{}, a.ProvisioningStateDir())
}

func (a *ArduinoApp) AppComposeTemplateFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join(MainTemplateFileName)
}

func (a *ArduinoApp) AppComposeOverrideTemplateFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join(OverrideTemplateFileName)
}

func (a *ArduinoApp) AppComposeFilePath() *paths.Path {
	return a.ProvisioningStateDir().Join("app-compose.yaml")
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
