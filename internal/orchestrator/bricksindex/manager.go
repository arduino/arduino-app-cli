package bricksindex

import (
	"fmt"
	"log/slog"

	"github.com/arduino/go-paths-helper"
)

type Manager struct {
	sources []BrickSource
}

type BrickSource interface {
	GetByID(id string) (*Brick, bool)
	ListBricks() ([]Brick, bool)

	GetReadme(id string) (string, error)
	GetComposePath(id string) (*paths.Path, bool)
	GetApiDocPath(id string) (*paths.Path, error)
	GetExamplesPath(id string) (paths.PathList, error)
}

func New(builtin BrickSource) (*Manager, error) {
	return &Manager{
		sources: []BrickSource{builtin},
	}, nil
}

func (m *Manager) WithBrickSource(source BrickSource) *Manager {
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
		} else {
			slog.Warn("cannot open readme for brick", "brickID", id, "error", err.Error())
		}
	}
	return "", fmt.Errorf("readme for brick %s not found", id)
}

func (m *Manager) GetBrickApiDocPathFromID(id string) (string, error) {
	for _, src := range m.sources {
		if b, err := src.GetApiDocPath(id); err == nil {
			return b.String(), nil
		} else {
			slog.Warn("cannot open api-docs for brick", "brickID", id, "error", err.Error())
		}
	}
	return "", fmt.Errorf("api-docs for brick %s not found", id)
}

func (m *Manager) GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error) {
	for _, src := range m.sources {
		if b, err := src.GetExamplesPath(id); err == nil {
			return b, nil
		} else {
			slog.Warn("cannot open code examples for brick", "brickID", id, "error", err.Error())
		}
	}
	return nil, fmt.Errorf("code examples for brick %s not found", id)
}
