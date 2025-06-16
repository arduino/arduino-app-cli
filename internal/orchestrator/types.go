package orchestrator

import (
	"errors"
	"path"
	"strings"

	"github.com/arduino/go-paths-helper"
)

var ErrInvalidID = errors.New("not a valid id")

// ID represents an identifier for an application or example in the orchestrator.
// It can be in three formats:
// - "user/<path_to_app>", for custom user applications in default directories
// - "examples/<path_to_app>", for built-in examples
// - "/<path_to_app>" for applications not in default directories
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

	if !p.Exist() {
		return "", ErrInvalidID
	}

	p, err := p.Abs()
	if err != nil {
		return "", err
	}
	return ID(p.String()), nil
}

// ParseID parses a string into an ID.
// It accepts both absolute paths and relative paths.
func ParseID(id string) (ID, error) {
	if id == "" {
		return "", ErrInvalidID
	}
	// Attempt to expand the path to absolute path.
	if p, err := paths.New(id).Abs(); err == nil && p.Exist() {
		id = p.String()
	}

	v := ID(id)
	return v, v.Validate()
}

func (id ID) IsExample() bool {
	return strings.HasPrefix(string(id), "examples/")
}

func (id ID) IsApp() bool {
	return strings.HasPrefix(string(id), "user/")
}

func (id ID) IsPath() bool {
	return strings.HasPrefix(string(id), "/")
}

func (id ID) ToPath() *paths.Path {
	switch {
	case id.IsApp():
		return orchestratorConfig.AppsDir().Join(strings.TrimPrefix(string(id), "user/"))
	case id.IsExample():
		return orchestratorConfig.DataDir().Join(string(id))
	default:
		return paths.New(string(id))
	}
}

func (id ID) Rel() string {
	if id.IsPath() {
		wd, err := paths.Getwd()
		if err != nil {
			return string(id)
		}
		rel, err := paths.New(string(id)).RelFrom(wd)
		if err != nil {
			return string(id)
		}
		if !strings.HasPrefix(rel.String(), "./") && !strings.HasPrefix(rel.String(), "../") {
			return "./" + rel.String()
		}
		return rel.String()
	}
	return string(id)
}
func (id ID) Validate() error {
	if !id.IsApp() &&
		!id.IsExample() &&
		!id.IsPath() ||
		(id.IsPath() &&
			!paths.New(string(id)).Exist()) {
		return ErrInvalidID
	}
	return nil
}
