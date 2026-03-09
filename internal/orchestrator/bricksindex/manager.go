package bricksindex

import (
	"fmt"

	"github.com/arduino/go-paths-helper"
)

type Manager struct {
	sources []BrickSource
}

type BrickSource interface {
	// TODO: create a struct for a brick ID not a string
	GetByID(id string) (*Brick, bool)
	ListBricks() ([]Brick, bool)

	GetReadme(id string) (string, error)
	GetComposePath(id string) (*paths.Path, bool)
	GetApiDocPath(id string) (*paths.Path, error)
	GetCodeExamplesPath(id string) (paths.PathList, error)
}

func New(builtin BrickSource) (*Manager, error) {
	return &Manager{
		sources: []BrickSource{builtin},
	}, nil
}

// WithBrickSource adds a new BrickSource to the manager, it will be prioritized over the existing ones
// this is useful to add a local brick source that can override the builtin one
// it returns a new Manager instance with the new source added, it does not modify the existing one
func (m Manager) WithBrickSource(source BrickSource) *Manager {
	return &Manager{sources: append([]BrickSource{source}, m.sources...)}
}

func (m *Manager) ListBricks() []Brick {
	var bricks []Brick
	for _, src := range m.sources {
		if b, ok := src.ListBricks(); ok {
			bricks = append(bricks, b...)
		}
	}
	return bricks
}

func (m *Manager) FindBrickByID(id string) (*Brick, bool) {
	for _, src := range m.sources {
		if b, ok := src.GetByID(id); ok {
			return b, true
		}
	}
	return nil, false
}

func (m *Manager) ComposePath(id string) (*paths.Path, error) {
	for _, src := range m.sources {
		if b, ok := src.GetComposePath(id); ok {
			return b, nil
		}
	}
	return nil, fmt.Errorf("compose path for brick %s not found", id)
}

func (m *Manager) GetBrickReadmeFromID(id string) (string, error) {
	for _, src := range m.sources {
		if b, err := src.GetReadme(id); err == nil {
			return b, nil
		}
	}
	return "", fmt.Errorf("cannot get readme for brick %s", id)
}

func (m *Manager) GetBrickApiDocPathFromID(id string) (string, error) {
	for _, src := range m.sources {
		if b, err := src.GetApiDocPath(id); err == nil {
			return b.String(), nil
		}
	}
	return "", fmt.Errorf("cannot get api-docs for brick %s", id)
}

func (m *Manager) GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error) {
	for _, src := range m.sources {
		if b, err := src.GetCodeExamplesPath(id); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("cannot get code examples for brick %s", id)
}
