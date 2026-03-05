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
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"go.bug.st/f"
)

func TestLoad(t *testing.T) {
	t.Run("it fails if the app path is nil", func(t *testing.T) {
		app, err := Load(nil)
		assert.Error(t, err)
		assert.Empty(t, app)
		assert.Contains(t, err.Error(), "empty app path")
	})

	t.Run("it fails if the app path is empty", func(t *testing.T) {
		app, err := Load(paths.New(""))
		assert.Error(t, err)
		assert.Empty(t, app)
		assert.Contains(t, err.Error(), "empty app path")
	})

	t.Run("it fails if the app path exist but it's a file", func(t *testing.T) {
		_, err := Load(paths.New("testdata/app.yaml"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "app path must be a directory")
	})

	t.Run("it fails if the app path does not exist", func(t *testing.T) {
		_, err := Load(paths.New("testdata/this-folder-does-not-exist"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "app path is not valid")
	})

	t.Run("it loads an app correctly", func(t *testing.T) {
		app, err := Load(paths.New("testdata/AppSimple"))
		assert.NoError(t, err)
		assert.NotEmpty(t, app)

		assert.NotNil(t, app.MainPythonFile)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppSimple/python/main.py")), app.MainPythonFile.String())
		sketchPath, ok := app.GetSketchPath()
		assert.True(t, ok)
		assert.NotNil(t, sketchPath)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppSimple/sketch")), sketchPath.String())
		assert.Equal(t, "Simple App", app.Descriptor.Name)
		assert.Equal(t, "this is a simple app.", app.Descriptor.Description)
		assert.Empty(t, app.Descriptor.Ports)
		assert.Empty(t, app.Descriptor.Bricks)
	})

	t.Run("should extract description from README.md if not set in app.yaml", func(t *testing.T) {
		app, err := Load(paths.New("testdata/AppSimpleNoDescription"))
		assert.NoError(t, err)
		assert.NotEmpty(t, app)

		assert.Equal(t, "Simple App Without Description", app.Descriptor.Name)
		assert.Equal(t, "Simple app is used for testing purposes.", app.Descriptor.Description)
	})

	t.Run("it loads an app with missing sketch folder", func(t *testing.T) {
		app, err := Load(paths.New("testdata/MissingSketch"))
		assert.NoError(t, err)
		assert.NotEmpty(t, app)

		assert.NotNil(t, app.MainPythonFile)

		sketchPath, ok := app.GetSketchPath()
		assert.False(t, ok)
		assert.Nil(t, sketchPath)
	})
}

func TestMissingDescriptor(t *testing.T) {
	appFolderPath := paths.New("testdata", "MissingDescriptor")

	// Load app
	app, err := Load(appFolderPath)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "descriptor app.yaml file missing from app")
	assert.Empty(t, app)
}

func TestMissingMains(t *testing.T) {
	appFolderPath := paths.New("testdata", "MissingMains")

	// Load app
	app, err := Load(appFolderPath)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "main python file and sketch file missing from app")
	assert.Empty(t, app)
}

func TestExtractFirstParagraph(t *testing.T) {
	tests := []struct {
		name     string
		input    io.Reader
		expected string
	}{
		{
			name:     "it extracts the first paragraph from a markdown string",
			input:    bytes.NewReader([]byte("# Title\n\nThis is the first paragraph.\n\nThis is the second paragraph.")),
			expected: "This is the first paragraph.",
		},
		{
			name:     "it extracts the first paragraph if there are no title",
			input:    bytes.NewReader([]byte("This is the first paragraph.\n\nThis is the second paragraph.")),
			expected: "This is the first paragraph.",
		},
		{
			name:     "it returns empty string if there are no paragraphs",
			input:    bytes.NewReader([]byte("# Title")),
			expected: "",
		},
		{
			name:     "it returns the first valid paragraph even if there are multiple newlines",
			input:    bytes.NewReader([]byte("# Title\n\n\n\n the first valid paragraph.")),
			expected: "the first valid paragraph.",
		},
		{
			name: "it returns multiple lines of the first paragraph",
			input: bytes.NewReader([]byte(`# Title

This is the first line of the first paragraph.
This is the second line of the first paragraph.

This is the second paragraph.`)),
			expected: "This is the first line of the first paragraph. This is the second line of the first paragraph.",
		},
		{
			name:     "it returns the first paragraph cleared from bold or italic markdown syntax",
			input:    bytes.NewReader([]byte("# Title\n\n**This is the bold** paragraph.\n*This is italic* paragraph.")),
			expected: "This is the bold paragraph. This is italic paragraph.",
		},
		{
			name:     "it returns the first paragraph cleared from link markdown syntax",
			input:    bytes.NewReader([]byte("# Title\n\nThis is a [link](https://example.com) paragraph.")),
			expected: "This is a link paragraph.",
		},
		{
			name:     "it ignores images at the beginning of the paragraph",
			input:    bytes.NewReader([]byte("# Title\n\n![Banner](image.png)\nThis is the actual description.")),
			expected: "This is the actual description.",
		},
		{
			name:     "it returns empty string if the paragraph contains only an image",
			input:    bytes.NewReader([]byte("# Title\n\n![Banner](image.png)")),
			expected: "",
		},
		{
			name:     "it should include inline code content",
			input:    bytes.NewReader([]byte("# Title\n\nThis is `code` example.")),
			expected: "This is code example.",
		},
		{
			name:     "it should return inline code paragraph",
			input:    bytes.NewReader([]byte("# Title\n\n`hello world`")),
			expected: "hello world",
		},
		{
			name:     "it should handle hard line break",
			input:    bytes.NewReader([]byte("# Title\n\nFirst line.  \nSecond line.")),
			expected: "First line. Second line.",
		},
		{
			name: "it should skip paragraph containing only linked image",
			input: bytes.NewReader([]byte(`# Title

[![Alt](img.png)](https://example.com)

Real paragraph.`)),
			expected: "Real paragraph.",
		},
		{
			name:     "it should skip image-only paragraph and return next paragraph",
			input:    bytes.NewReader([]byte("# Title\n\n![Banner](image.png)\n\nThis is the real first paragraph.")),
			expected: "This is the real first paragraph.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := extractFirstParagraph(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "it returns the string unchanged if shorter than max",
			input:    "Short description.",
			max:      50,
			expected: "Short description.",
		},
		{
			name:     "it returns the string unchanged if equal to max",
			input:    "Exactly fifty chars long description right here!!",
			max:      50,
			expected: "Exactly fifty chars long description right here!!",
		},
		{
			name:     "it truncates at word boundary",
			input:    "This is a very long description that exceeds the maximum allowed length",
			max:      50,
			expected: "This is a very long description that exceeds the",
		},
		{
			name:     "it truncates at char boundary if no space found",
			input:    "Abcdefghijklmnopqrstuvwxyz",
			max:      10,
			expected: "Abcdefghij",
		},
		{
			name:     "it returns empty string if input is empty",
			input:    "",
			max:      50,
			expected: "",
		},
		{
			name:     "it returns empty string if max is zero",
			input:    "Some text",
			max:      0,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := truncateDescription(test.input, test.max)
			assert.Equal(t, test.expected, result)
		})
	}
}
