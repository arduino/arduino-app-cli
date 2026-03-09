package bricksmanager

import (
	"fmt"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/brickslocalindex"
)

type Manager struct {
	sources []BrickSource
}

type BrickSource interface {
	// TODO: create a struct for a brick ID not a string
	FindBrickByID(id string) (*bricksindex.Brick, bool)
	ListBricks() ([]bricksindex.Brick, bool)

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

func (r *Manager) WithAppLocalSource(local *brickslocalindex.LocalBricksIndex) *Manager {
	for _, s := range r.sources {
		if _, ok := s.(*brickslocalindex.LocalBricksIndex); ok {
			return r // already present
		}
	}

	return &Manager{sources: append([]BrickSource{local}, r.sources...)}
}

func (r *Manager) ListBricks() []bricksindex.Brick {
	var bricks []bricksindex.Brick
	for _, src := range r.sources {
		if b, ok := src.ListBricks(); ok {
			bricks = append(bricks, b...)
		}
	}
	return bricks
}

func (r *Manager) FindBrickByID(id string) (*bricksindex.Brick, bool) {
	for _, src := range r.sources {
		if b, ok := src.FindBrickByID(id); ok {
			return b, true
		}
	}
	return nil, false
}

func (r *Manager) ComposePath(id string) (*paths.Path, error) {
	_, found := r.FindBrickByID(id)
	if !found {
		return nil, fmt.Errorf("brick %s not found", id)
	}

	if p, err := r.ComposePath(id); err == nil {
		return p, nil
	}

	return nil, fmt.Errorf("compose path for brick %s not found", id)
}

func (r *Manager) GetBrickReadmeFromID(id string) (string, error) {
	_, found := r.FindBrickByID(id)
	if !found {
		return "", fmt.Errorf("brick %s not found", id)
	}

	if content, err := r.GetBrickReadmeFromID(id); err == nil {
		return content, nil
	}
	return "", fmt.Errorf("cannot get readme for brick %s", id)
}

func (r *Manager) GetBrickApiDocPathFromID(id string) (string, error) {
	_, found := r.FindBrickByID(id)
	if !found {
		return "", fmt.Errorf("brick %s not found", id)
	}

	if path, err := r.GetBrickApiDocPathFromID(id); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("cannot get api-docs for brick %s", id)
}

func (r *Manager) GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error) {
	_, found := r.FindBrickByID(id)
	if !found {
		return nil, fmt.Errorf("brick %s not found", id)
	}

	if paths, err := r.GetBrickCodeExamplesPathFromID(id); err == nil {
		return paths, nil
	}
	return nil, fmt.Errorf("cannot get code examples for brick %s", id)
}
