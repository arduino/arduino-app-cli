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
)

const maxDescriptionLength = 150

// ArduinoApp holds all the files composing an app
type ArduinoApp struct {
	Name           string
	MainPythonFile *paths.Path
	mainSketchPath *paths.Path
	FullPath       *paths.Path // FullPath is the path to the App folder
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

	app := ArduinoApp{
		FullPath:   appPath,
		Descriptor: AppDescriptor{},
	}

	if descriptorFile := app.GetDescriptorPath(); descriptorFile.Exist() {
		desc, err := ParseDescriptorFile(descriptorFile)
		if err != nil {
			return ArduinoApp{}, fmt.Errorf("error loading app descriptor file: %w", err)
		}
		app.Descriptor = desc
		app.Name = desc.Name

		if app.Descriptor.Description == "" {
			description, err := app.getAppDescriptionFromReadme()
			if err != nil {
				// Log the error but don't fail the loading process, as the description is optional
				slog.Warn("cannot extract app description from README.md", "error", err)
			} else {
				app.Descriptor.Description = description
			}
		}

	} else {
		return ArduinoApp{}, errors.New("descriptor app.yaml file missing from app")
	}

	if appPath.Join("python", "main.py").Exist() {
		app.MainPythonFile = appPath.Join("python", "main.py")
	}

	if appPath.Join("sketch", "sketch.ino").Exist() {
		// TODO: check sketch casing?
		app.mainSketchPath = appPath.Join("sketch")
	}

	if app.MainPythonFile == nil && app.mainSketchPath == nil {
		return ArduinoApp{}, errors.New("main python file and sketch file missing from app")
	}

	return app, nil
}

func (a *ArduinoApp) GetSketchPath() (*paths.Path, bool) {
	if a == nil || a.mainSketchPath == nil {
		return nil, false
	}
	return a.mainSketchPath, true
}

// GetDescriptorPath returns the path to the app descriptor file (app.yaml or app.yml)
func (a *ArduinoApp) GetDescriptorPath() *paths.Path {
	descriptorFile := a.FullPath.Join("app.yaml")
	if !descriptorFile.Exist() {
		alternateDescriptorFile := a.FullPath.Join("app.yml")
		if alternateDescriptorFile.Exist() {
			return alternateDescriptorFile
		}
	}
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
