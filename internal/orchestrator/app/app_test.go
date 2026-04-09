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
	"bytes"
	"io"
	"os"
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

	t.Run("it loads an app with local bricks", func(t *testing.T) {
		app, err := Load(paths.New("testdata/AppWithLocalBricks"))
		assert.NoError(t, err)
		assert.NotEmpty(t, app)
		assert.Len(t, app.LocalBricks, 1)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppWithLocalBricks/bricks/my-first-brick")), app.LocalBricks[0].FullPath.String())
		assert.Equal(t, "my-first-brick", app.LocalBricks[0].ID)
		assert.Equal(t, "My First Brick", app.LocalBricks[0].Name)
		assert.Equal(t, "App", app.LocalBricks[0].Source)
		assert.NotNil(t, app.LocalBricks[0].ComposeFile)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppWithLocalBricks/bricks/my-first-brick/brick_compose.yaml")), app.LocalBricks[0].ComposeFile.String())
		assert.NotNil(t, app.LocalBricks[0].ReadmeFile)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppWithLocalBricks/bricks/my-first-brick/README.md")), app.LocalBricks[0].ReadmeFile.String())
		assert.NotNil(t, app.LocalBricks[0].ExamplesPath)
		assert.Equal(t, f.Must(filepath.Abs("testdata/AppWithLocalBricks/bricks/my-first-brick/examples")), app.LocalBricks[0].ExamplesPath.String())
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

func TestLoadBricksFromFolder(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(t *testing.T, bricksDir *paths.Path)
		expectedBrickCount int
		expectedBrickIDs   []string
	}{
		{
			name: "loads multiple valid bricks",
			setup: func(t *testing.T, bricksDir *paths.Path) {
				// Create brick1
				brick1Dir := bricksDir.Join("brick1")
				assert.NoError(t, brick1Dir.MkdirAll())
				brick1Config := `id: brick1`
				assert.NoError(t, os.WriteFile(brick1Dir.Join("brick_config.yaml").String(), []byte(brick1Config), 0600))

				// Create brick2
				brick2Dir := bricksDir.Join("brick2")
				assert.NoError(t, brick2Dir.MkdirAll())
				brick2Config := `id: brick2`
				assert.NoError(t, os.WriteFile(brick2Dir.Join("brick_config.yaml").String(), []byte(brick2Config), 0600))

				// Create brick3
				brick3Dir := bricksDir.Join("brick3")
				assert.NoError(t, brick3Dir.MkdirAll())
				brick3Config := `id: brick3`
				assert.NoError(t, os.WriteFile(brick3Dir.Join("brick_config.yaml").String(), []byte(brick3Config), 0600))
			},
			expectedBrickCount: 3,
			expectedBrickIDs:   []string{"brick1", "brick2", "brick3"},
		},
		{
			name: "skips directories without brick_config.yaml",
			setup: func(t *testing.T, bricksDir *paths.Path) {
				// Create valid brick
				validBrickDir := bricksDir.Join("valid_brick")
				assert.NoError(t, validBrickDir.MkdirAll())
				validBrickConfig := `id: valid_brick`
				assert.NoError(t, os.WriteFile(validBrickDir.Join("brick_config.yaml").String(), []byte(validBrickConfig), 0600))

				// Create directory without brick_config.yaml
				invalidDir := bricksDir.Join("no_config")
				assert.NoError(t, invalidDir.MkdirAll())
				assert.NoError(t, os.WriteFile(invalidDir.Join("some_file.txt").String(), []byte("test"), 0600))
			},
			expectedBrickCount: 1,
			expectedBrickIDs:   []string{"valid_brick"},
		},
		{
			name: "skips bricks with missing ID field",
			setup: func(t *testing.T, bricksDir *paths.Path) {
				// Create valid brick
				validBrickDir := bricksDir.Join("valid_brick")
				assert.NoError(t, validBrickDir.MkdirAll())
				validBrickConfig := `id: valid_brick`
				assert.NoError(t, os.WriteFile(validBrickDir.Join("brick_config.yaml").String(), []byte(validBrickConfig), 0600))

				// Create brick without ID (invalid)
				noIDDir := bricksDir.Join("no_id_brick")
				assert.NoError(t, noIDDir.MkdirAll())
				noIDConfig := `name: No ID Brick`
				assert.NoError(t, os.WriteFile(noIDDir.Join("brick_config.yaml").String(), []byte(noIDConfig), 0600))
			},
			expectedBrickCount: 1,
			expectedBrickIDs:   []string{"valid_brick"},
		},
		{
			name: "loads bricks with additional optional files",
			setup: func(t *testing.T, bricksDir *paths.Path) {
				brickDir := bricksDir.Join("full_brick")
				assert.NoError(t, brickDir.MkdirAll())

				brickConfig := `id: full_brick`
				assert.NoError(t, os.WriteFile(brickDir.Join("brick_config.yaml").String(), []byte(brickConfig), 0600))

				// Create optional files
				assert.NoError(t, os.WriteFile(brickDir.Join("brick_compose.yaml").String(), []byte("services: {}"), 0600))
				assert.NoError(t, os.WriteFile(brickDir.Join("README.md").String(), []byte("# Full Brick"), 0600))
			},
			expectedBrickCount: 1,
			expectedBrickIDs:   []string{"full_brick"},
		},
		{
			name: "loads bricks from nested subfolder structure",
			setup: func(t *testing.T, bricksDir *paths.Path) {
				// Create brick in root level
				brick1Dir := bricksDir.Join("brick1")
				assert.NoError(t, brick1Dir.MkdirAll())
				brick1Config := `id: brick1`
				assert.NoError(t, os.WriteFile(brick1Dir.Join("brick_config.yaml").String(), []byte(brick1Config), 0600))

				nestedDir := bricksDir.Join("nested/brick1")
				assert.NoError(t, nestedDir.MkdirAll())
				brick2Config := `id: brick2`
				assert.NoError(t, os.WriteFile(nestedDir.Join("brick_config.yaml").String(), []byte(brick2Config), 0600))

				deepDir := bricksDir.Join("nested/subnested/brick2")
				assert.NoError(t, deepDir.MkdirAll())
				brick3Config := `id: brick3`
				assert.NoError(t, os.WriteFile(deepDir.Join("brick_config.yaml").String(), []byte(brick3Config), 0600))
			},
			expectedBrickCount: 3,
			expectedBrickIDs:   []string{"brick1", "brick2", "brick3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			bricksDir := paths.New(tmpDir)

			tt.setup(t, bricksDir)

			bricks := loadBricksFromFolder(bricksDir)

			assert.Equal(t, tt.expectedBrickCount, len(bricks), "expected %d bricks but got %d", tt.expectedBrickCount, len(bricks))

			for i, expectedID := range tt.expectedBrickIDs {
				assert.Equal(t, expectedID, bricks[i].ID, "brick %d has incorrect ID", i)
			}
		})
	}
}

func TestLoadBricksFromFolderWithNoBricks(t *testing.T) {
	t.Run("returns nil for nonexistent directory", func(t *testing.T) {
		bricksDir := paths.New("/nonexistent/path")
		bricks := loadBricksFromFolder(bricksDir)
		assert.Nil(t, bricks)
		assert.Len(t, bricks, 0)
	})

	t.Run("returns empty slice for directory with no bricks", func(t *testing.T) {
		tmpDir := t.TempDir()
		bricksDir := paths.New(tmpDir)

		// Create directory without any brick_config.yaml files
		otherDir := bricksDir.Join("other")
		assert.NoError(t, otherDir.MkdirAll())

		bricks := loadBricksFromFolder(bricksDir)
		assert.Empty(t, bricks)
	})
}
