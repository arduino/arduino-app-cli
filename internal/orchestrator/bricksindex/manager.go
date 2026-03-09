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

	// GetBrickComposeFilePathFromID
	ComposePath(id string) (*paths.Path, bool)
	GetBrickReadmeFromID(id string) (string, error)
	GetBrickApiDocPathFromID(id string) (string, error)
	GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error)
}

func New(builtin BrickSource) (*Manager, error) {
	return &Manager{
		sources: []BrickSource{builtin},
	}, nil
}

func (r *Manager) WithAppLocalSource(source BrickSource) *Manager {
	return &Manager{sources: append([]BrickSource{source}, r.sources...)}
}

func (r *Manager) ListBricks() []Brick {
	var bricks []Brick
	for _, src := range r.sources {
		if b, ok := src.ListBricks(); ok {
			bricks = append(bricks, b...)
		}
	}
	return bricks
}

func (r *Manager) FindBrickByID(id string) (*Brick, bool) {
	for _, src := range r.sources {
		if b, ok := src.GetByID(id); ok {
			return b, true
		}
	}
	return nil, false
}

func (r *Manager) ComposePath(id string) (*paths.Path, error) {
	for _, src := range r.sources {
		if b, ok := src.ComposePath(id); ok {
			return b, nil
		}
	}
	return nil, fmt.Errorf("compose path for brick %s not found", id)
}

func (r *Manager) GetBrickReadmeFromID(id string) (string, error) {
	for _, src := range r.sources {
		if b, err := src.GetBrickReadmeFromID(id); err == nil {
			return b, nil
		}
	}
	return "", fmt.Errorf("cannot get readme for brick %s", id)
}

func (r *Manager) GetBrickApiDocPathFromID(id string) (string, error) {
	for _, src := range r.sources {
		if b, err := src.GetBrickApiDocPathFromID(id); err == nil {
			return b, nil
		}
	}
	return "", fmt.Errorf("cannot get api-docs for brick %s", id)
}

func (r *Manager) GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error) {
	for _, src := range r.sources {
		if b, err := src.GetBrickCodeExamplesPathFromID(id); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("cannot get code examples for brick %s", id)
}
