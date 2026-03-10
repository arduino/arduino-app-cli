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
		want := []AppBrick{
			{
				FullPath:     paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick"),
				composeFile:  paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/brick_compose.yaml"),
				examplesPath: paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/examples"),
				readmeFile:   paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/README.md"),
				docsAPIPath:  paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/docs/API.md"),
				Brick: bricksindex.Brick{
					ID:               "my_first_brick",
					Name:             "My First Brick",
					Description:      "This is my first brick",
					RequireContainer: true,
				},
			},
			{
				FullPath:     paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick"),
				composeFile:  paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/brick_compose.yaml"),
				examplesPath: paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/examples"),
				readmeFile:   paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/README.md"),
				docsAPIPath:  paths.New("testdata/AppWithLocalBricks/bricks/my_second_brick/docs/API.md"),
				Brick: bricksindex.Brick{
					ID:          "my_second_brick",
					Name:        "My Second Brick",
					Description: "This is my second brick",
				},
			},
		}

		index, err := Load(paths.New("testdata/AppWithLocalBricks"))
		assert.Equal(t, want, index.Bricks)
		require.NoError(t, err)
		assert.Len(t, index.Bricks, 2)

		compose, ok := index.GetComposePath("my_first_brick")
		assert.True(t, ok)
		assert.Equal(t, paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/brick_compose.yaml"), compose)

		examples, err := index.GetExamplesPath("my_first_brick")
		assert.NoError(t, err)
		assert.Equal(t, paths.NewPathList("testdata/AppWithLocalBricks/bricks/my_first_brick/examples/01_basic.py", "testdata/AppWithLocalBricks/bricks/my_first_brick/examples/02_advanced.py"), examples)

		apiDoc, err := index.GetApiDocPath("my_first_brick")
		assert.NoError(t, err)
		assert.Equal(t, paths.New("testdata/AppWithLocalBricks/bricks/my_first_brick/docs/API.md"), apiDoc)
	})

	t.Run("it returns an error if local bricks are missing", func(t *testing.T) {
		index, err := Load(paths.New("testdata/AppMissingLocalBricks"))
		require.Error(t, err)
		assert.Nil(t, index)
	})
}
