// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package app

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

var ErrInvalidID = errors.New("not a valid id")

type ID struct {
	path                 *paths.Path
	encodedID            string
	isFromKnownLocaltion bool
	isExample            bool
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

type IDProvider struct {
	cfg config.Configuration
}

func NewAppIDProvider(cfg config.Configuration) *IDProvider {
	return &IDProvider{cfg: cfg}
}

func (p *IDProvider) IDFromBase64(id string) (ID, error) {
	decodedID, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return ID{}, err
	}
	return p.parseID(string(decodedID))
}

func (p *IDProvider) IDFromPath(path *paths.Path) (ID, error) {
	if path == nil || !path.Exist() {
		return ID{}, ErrInvalidID
	}
	path, err := path.Abs()
	if err != nil {
		return ID{}, err
	}

	var (
		id                  string
		isFromKnownLocation bool
		isExample           bool
	)
	switch {
	case strings.HasPrefix(path.String(), p.cfg.AppsDir().String()):
		rel, err := path.RelFrom(p.cfg.AppsDir())
		if err != nil {
			return ID{}, ErrInvalidID
		}
		id = "user:" + rel.String()
		isFromKnownLocation = true
	case strings.HasPrefix(path.String(), p.cfg.ExamplesDir().String()):
		rel, err := path.RelFrom(p.cfg.ExamplesDir())
		if err != nil {
			return ID{}, ErrInvalidID
		}
		id = "examples:" + rel.String()
		isFromKnownLocation = true
		isExample = true
	default:
		id = path.String()
	}

	return ID{
		path:                 path,
		encodedID:            base64.RawURLEncoding.EncodeToString([]byte(id)),
		isFromKnownLocaltion: isFromKnownLocation,
		isExample:            isExample,
	}, nil
}

// ParseID parses a string into an ID.
// It accepts both absolute paths and relative paths.
func (p *IDProvider) ParseID(id string) (ID, error) {
	return p.parseID(id)
}

func (p *IDProvider) parseID(id string) (ID, error) {
	var path *paths.Path

	prefix, appPath, found := strings.Cut(id, ":")
	if found {
		var isExample bool
		switch prefix {
		case "user":
			path = p.cfg.AppsDir().Join(appPath)
		case "examples":
			path = p.cfg.ExamplesDir().Join(appPath)
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
