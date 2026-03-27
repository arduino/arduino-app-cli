package bricksindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
)

func TestLoadBrickFromFolder(t *testing.T) {
	folderIndex := loadFromFolder(paths.New("testdata"))
	assert.Len(t, folderIndex, 1)
	assert.Equal(t, "my-first-brick", folderIndex[0].ID)
	assert.Equal(t, "My First Brick", folderIndex[0].Name)
	assert.Equal(t, "Local", folderIndex[0].Source)
	assert.NotNil(t, folderIndex[0].composeFile)
	assert.Equal(t, "testdata/my-first-brick/brick_compose.yaml", folderIndex[0].composeFile.String())
	assert.NotNil(t, folderIndex[0].readmeFile)
	assert.Equal(t, "testdata/my-first-brick/README.md", folderIndex[0].readmeFile.String())
	assert.NotNil(t, folderIndex[0].examplesPath)
	assert.Equal(t, "testdata/my-first-brick/examples", folderIndex[0].examplesPath.String())
}
