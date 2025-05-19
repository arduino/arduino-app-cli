package orchestrator

import (
	"errors"
	"path"
	"strings"

	"github.com/arduino/go-paths-helper"
)

var ErrInvalidID = errors.New("not a valid id")

type ID string

func NewIDFromPath(p *paths.Path) (ID, error) {
	id, found := strings.CutPrefix(p.String(), orchestratorConfig.AppsDir().String())
	if found {
		return ID(path.Join("user", id)), nil
	}

	id, found = strings.CutPrefix(p.String(), orchestratorConfig.ExamplesDir().String())
	if found {
		return ID(path.Join("examples", id)), nil
	}

	return "", ErrInvalidID
}

func (id ID) IsExample() bool {
	return strings.HasPrefix(string(id), "examples/")
}

func (id ID) IsApp() bool {
	return strings.HasPrefix(string(id), "user/")
}

func (id ID) ToPath() (*paths.Path, error) {
	switch {
	case id.IsApp():
		return orchestratorConfig.AppsDir().Join(strings.TrimPrefix(string(id), "user/")), nil
	case id.IsExample():
		return orchestratorConfig.DataDir().Join(string(id)), nil
	}
	return nil, ErrInvalidID
}

func (id ID) Validate() error {
	if !id.IsApp() && !id.IsExample() {
		return ErrInvalidID
	}
	return nil
}
