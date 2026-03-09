package applocal

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

func TestLoadLocalAppBricks(t *testing.T) {

	t.Run("it detects local bricks in an app", func(t *testing.T) {
		index, err := Load(paths.New("testdata/AppWithLocalBricks"))
		require.NoError(t, err)
		assert.Len(t, index.Bricks, 2)

		want := []AppBrick{
			{
				FullPath:    paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick"),
				ComposeFile: paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/brick_compose.yaml"),
				ReadmeFile:  paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/README.md"),
				Brick: bricksindex.Brick{
					ID:               "dneri:my_first_brick",
					Name:             "My First Brick",
					Description:      "This is my first brick",
					RequireContainer: true,
				},
			},
			{
				FullPath:    paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick"),
				ComposeFile: paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/brick_compose.yaml"),
				ReadmeFile:  paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/README.md"),
				Brick: bricksindex.Brick{
					ID:          "dneri:my_second_brick",
					Name:        "My Second Brick",
					Description: "This is my second brick",
				},
			},
		}
		assert.Equal(t, want, index.Bricks)
	})

	t.Run("it returns an error if local bricks are missing", func(t *testing.T) {
		index, err := Load(paths.New("testdata/AppMissingLocalBricks"))
		require.Error(t, err)
		assert.Nil(t, index)
	})
}
