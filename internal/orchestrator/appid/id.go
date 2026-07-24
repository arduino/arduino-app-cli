// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appid

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
	"github.com/arduino/arduino-app-cli/internal/platform"
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
//
//nolint:unparam // json.Marshaler requires an error return even when this implementation cannot fail.
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

type Provider struct {
	cfg  config.Configuration
	plat platform.Platform
}

func NewAppProvider(cfg config.Configuration, plat platform.Platform) *Provider {
	return &Provider{cfg: cfg, plat: plat}
}

func (p *Provider) IDFromBase64(id string) (ID, error) {
	decodedID, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return ID{}, err
	}
	return p.parseID(string(decodedID))
}

func (p *Provider) IDFromPath(path *paths.Path) (ID, error) {
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
	// users app
	if strings.HasPrefix(path.String(), p.cfg.AppsDir().String()) {
		rel, err := path.RelFrom(p.cfg.AppsDir())
		if err != nil {
			return ID{}, ErrInvalidID
		}
		id = "user:" + rel.String()
		isFromKnownLocation = true
	} else {
		// search in "core-and-foundational" and "bricks"
		if strings.HasPrefix(path.String(), p.cfg.ExamplesBaseDir().Join("core-and-foundational").String()) ||
			strings.HasPrefix(path.String(), p.cfg.ExamplesBaseDir().Join("bricks").String()) {
			rel, err := path.RelFrom(p.cfg.ExamplesBaseDir())
			if err != nil {
				return ID{}, ErrInvalidID
			}
			id = "examples:" + rel.String()
			isFromKnownLocation = true
			isExample = true
		} else {
			// search in examples/inspirational
			for _, example := range p.cfg.ExamplesDirs(p.plat) {
				if strings.HasPrefix(path.String(), example.String()) {
					rel, err := path.RelFrom(example)
					if err != nil {
						return ID{}, ErrInvalidID
					}
					id = "examples:" + rel.String()
					isFromKnownLocation = true
					isExample = true
					break
				}
			}
		}
	}

	if id == "" {
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
func (p *Provider) ParseID(id string) (ID, error) {
	return p.parseID(id)
}

func (p *Provider) parseID(id string) (ID, error) {
	prefix, appPath, found := strings.Cut(id, ":")
	if !found {
		return p.buildIdFromSystemPath(id)
	}

	switch prefix {
	case "user":
		return p.buildIDFromKnownLocation(id, p.cfg.AppsDir().Join(appPath), false), nil
	case "examples":
		path, err := p.resolveExamplePath(appPath)
		if err != nil {
			return ID{}, err
		}
		return p.buildIDFromKnownLocation(id, path, true), nil
	}
	return ID{}, ErrInvalidID
}

// resolveExamplePath resolves the path for an "examples:" id.
func (p *Provider) resolveExamplePath(appPath string) (*paths.Path, error) {
	// examples: always process examples/inspirational (legacy)
	for _, examplePath := range p.cfg.ExamplesDirs(p.plat) {
		if path := examplePath.Join(appPath); path.Exist() {
			return path, nil
		}
	}

	firstPath, _, found := strings.Cut(appPath, "/")
	// inspirational is reserved and must not be addressable directly
	if found && firstPath == "inspirational" {
		return nil, ErrInvalidID
	}

	// process core-and-foundational and bricks
	if found && (firstPath == "core-and-foundational" || firstPath == "bricks") {
		if path := p.cfg.ExamplesBaseDir().Join(appPath); path.Exist() {
			return path, nil
		}
	}

	return nil, ErrInvalidID
}

// builds an ID from a configuration defined path
func (p *Provider) buildIDFromKnownLocation(id string, path *paths.Path, isExample bool) ID {
	return ID{
		path:                 path,
		encodedID:            base64.RawURLEncoding.EncodeToString([]byte(id)),
		isFromKnownLocaltion: true,
		isExample:            isExample,
	}
}

// builds an ID from a raw filesystem path
func (p *Provider) buildIdFromSystemPath(id string) (ID, error) {
	path := paths.New(id)
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
