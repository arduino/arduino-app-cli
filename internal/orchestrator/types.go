package orchestrator

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/arduino/go-paths-helper"
)

var ErrInvalidID = errors.New("not a valid id")

type ID struct {
	path                 *paths.Path
	encodedID            string
	isFromKnownLocaltion bool
	isExample            bool
}

func NewIDFromBase64(id string) (ID, error) {
	decodedID, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return ID{}, err
	}
	return parseID(string(decodedID))
}

func NewIDFromPath(p *paths.Path) (ID, error) {
	if p == nil || !p.Exist() {
		return ID{}, ErrInvalidID
	}
	p, err := p.Abs()
	if err != nil {
		return ID{}, err
	}

	var (
		id                  string
		isFromKnownLocation bool
		isExample           bool
	)
	switch {
	case strings.HasPrefix(p.String(), orchestratorConfig.AppsDir().String()):
		rel, err := p.RelFrom(orchestratorConfig.AppsDir())
		if err != nil {
			return ID{}, ErrInvalidID
		}
		id = "user:" + rel.String()
		isFromKnownLocation = true
	case strings.HasPrefix(p.String(), orchestratorConfig.ExamplesDir().String()):
		rel, err := p.RelFrom(orchestratorConfig.ExamplesDir())
		if err != nil {
			return ID{}, ErrInvalidID
		}
		id = "examples:" + rel.String()
		isFromKnownLocation = true
		isExample = true
	default:
		id = p.String()
	}

	return ID{
		path:                 p,
		encodedID:            base64.RawURLEncoding.EncodeToString([]byte(id)),
		isFromKnownLocaltion: isFromKnownLocation,
		isExample:            isExample,
	}, nil
}

func parseID(id string) (ID, error) {
	var path *paths.Path

	prefix, appPath, found := strings.Cut(id, ":")
	if found {
		var isExample bool
		switch prefix {
		case "user":
			path = orchestratorConfig.AppsDir().Join(appPath)
		case "examples":
			path = orchestratorConfig.ExamplesDir().Join(appPath)
			isExample = true
		default:
			return ID{}, ErrInvalidID
		}
		return ID{
			path:                 path,
			encodedID:            base64.RawURLEncoding.EncodeToString([]byte(id)),
			isFromKnownLocaltion: true,
			isExample:            isExample,
		}, nil
	}

	path = paths.New(id)
	if path == nil {
		return ID{}, ErrInvalidID
	}

	path, err := path.Abs()
	if err != nil || !path.Exist() {
		return ID{}, ErrInvalidID
	}
	return ID{
		path:      path,
		encodedID: base64.RawURLEncoding.EncodeToString([]byte(id)),
	}, nil
}

// ParseID parses a string into an ID.
// It accepts both absolute paths and relative paths.
func ParseID(id string) (ID, error) {
	return parseID(id)
}

func (id ID) IsExample() bool {
	return id.isExample
}

func (id ID) IsApp() bool {
	return !id.isExample
}

func (id ID) ToPath() *paths.Path {
	return id.path.Clone()
}

func (id ID) String() string {
	return id.encodedID
}

// MarshalJSON implements the json.Marshaler interface for ID.
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.encodedID + `"`), nil
}

// Equal implements the go-cmp equality interface.
func (id ID) Equal(other ID) bool {
	return id.path.EqualsTo(other.path) &&
		id.isFromKnownLocaltion == other.isFromKnownLocaltion &&
		id.isExample == other.isExample &&
		id.encodedID == other.encodedID
}
