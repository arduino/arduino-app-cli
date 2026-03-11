package bricksindex

import (
	"testing"

	paths "github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/assert"
)

type mockBrickSource struct {
	id     string
	bricks []Brick
}

func (m *mockBrickSource) GetByID(id string) (*Brick, bool) {
	for _, b := range m.bricks {
		if b.ID == id {
			return &b, true
		}
	}
	return nil, false
}

func (m *mockBrickSource) ListBricks() ([]Brick, bool) {
	return m.bricks, true
}

func (m *mockBrickSource) GetReadme(id string) (string, error)               { return "", nil }
func (m *mockBrickSource) GetComposePath(id string) (*paths.Path, bool)      { return nil, false }
func (m *mockBrickSource) GetApiDocPath(id string) (*paths.Path, error)      { return nil, nil }
func (m *mockBrickSource) GetExamplesPath(id string) (paths.PathList, error) { return nil, nil }

func TestWithBrickSourcePriority(t *testing.T) {
	builtin := &mockBrickSource{id: "builtin", bricks: []Brick{{ID: "a"}}}
	local := &mockBrickSource{id: "local", bricks: []Brick{{ID: "b"}}}
	mgr, _ := New(builtin)
	newMgr := mgr.WithBrickSource(local)

	assert.Len(t, newMgr.sources, 2)
	assert.Equal(t, newMgr.sources[0], local)
	assert.Equal(t, newMgr.sources[1], builtin)
}

func TestManagerListBricks(t *testing.T) {
	builtin := &mockBrickSource{id: "builtin", bricks: []Brick{{ID: "a"}}}
	mgr, _ := New(builtin)
	mgr = mgr.WithBrickSource(&mockBrickSource{id: "local", bricks: []Brick{{ID: "b"}}})
	bricks := mgr.ListBricks()

	// local brick "b" should come before builtin brick "a" because local source has priority
	want := []Brick{
		{ID: "b"},
		{ID: "a"},
	}
	assert.Len(t, bricks, 2)
	assert.Equal(t, want, bricks)
}

func TestManageGetBrick(t *testing.T) {
	builtin := &mockBrickSource{id: "builtin", bricks: []Brick{{ID: "a", Name: "builtin a"}}}
	mgr, _ := New(builtin)
	mgr = mgr.WithBrickSource(&mockBrickSource{id: "local", bricks: []Brick{{ID: "a", Name: "local a"}}})

	// local brick "a" should come before builtin brick "a" because local source has priority
	bBrick, found := mgr.FindBrickByID("a")
	assert.True(t, found)
	assert.Equal(t, "local a", bBrick.Name)
}
