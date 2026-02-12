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
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

type Parser func(path ...string) (any, error)
type ParserBuilder func() (Parser, error)

type Builder struct {
	pb []ParserBuilder
}

func New() *Builder {
	return &Builder{}
}

func (c *Builder) WithParser(p ParserBuilder) *Builder {
	c.pb = append(c.pb, p)
	return c
}

func (c *Builder) Parse(config any) error {

	// Build parsers
	parsers := []Parser{}
	for _, c := range c.pb {
		p, err := c()
		if err != nil {
			return fmt.Errorf("cannot load parser: %w", err)
		}
		parsers = append(parsers, p)
	}

	parseAll := func(path ...string) (any, error) {
		var errAll error
		for _, parser := range slices.Backward(parsers) {
			value, err := parser(path...)
			if err == nil {
				return value, nil
			}
			errAll = errors.Join(errAll, err)
		}
		return nil, errAll
	}

	return decode(nil, parseAll, config)
}

func EnvParser() ParserBuilder {
	return func() (Parser, error) {
		return func(path ...string) (any, error) {
			key := strings.Join(path, "__")
			key = toSnakeCase(key, UpperCase)
			value, exists := os.LookupEnv(key)
			if exists {
				return value, nil
			}
			return nil, fmt.Errorf("env varialbe %q not found", key)
		}, nil
	}
}

func TomlParser(filePath string) ParserBuilder {
	return func() (Parser, error) {
		var data map[string]any
		_, err := toml.DecodeFile(filePath, &data)
		if err != nil {
			return nil, fmt.Errorf("cannot decode toml file: %w", err)
		}

		return func(path ...string) (any, error) {
			var current any = data
			for _, p := range path {
				p = toSnakeCase(p, LowerCase)
				if m, ok := current.(map[string]any); ok {
					current, ok = m[p]
					if !ok {
						return nil, fmt.Errorf("key %q not found", p)
					}
				} else {
					break
				}
			}

			return current, nil
		}, nil
	}
}

type CaseType int

const (
	UpperCase CaseType = iota
	LowerCase
)

func toSnakeCase(camel string, caseType CaseType) string {
	var convertCase func(rune) rune
	switch caseType {
	case UpperCase:
		convertCase = func(c rune) rune {
			if unicode.IsLower(c) {
				return 'A' + c - 'a'
			}
			return c
		}
	case LowerCase:
		convertCase = func(c rune) rune {
			if unicode.IsUpper(c) {
				return 'a' + c - 'A'
			}
			return c
		}
	default:
		panic("Invalid caseType provided")
	}

	isLowerOrDigit := func(c rune) bool {
		return unicode.IsLower(c) || unicode.IsDigit(c)
	}

	isLower := unicode.IsLower

	var buf strings.Builder
	for i, c := range camel {
		if c == '_' {
			buf.WriteRune('_')
			continue
		}

		// Add underscore if c is upper case and either the previous character is lower case or the next one is lower case.
		// This is to handle acronyms like "URL" or "HTTP".
		if unicode.IsUpper(c) && i > 0 && camel[i-1] != '_' && (isLowerOrDigit(rune(camel[i-1])) || i+1 < len(camel) && isLower(rune(camel[i+1]))) {
			buf.WriteRune('_')
		}

		buf.WriteRune(convertCase(c))
	}
	return buf.String()
}
