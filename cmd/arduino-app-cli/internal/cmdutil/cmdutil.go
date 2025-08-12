package cmdutil

import (
	"encoding/base64"
	"strings"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

// IDToAlias returns the string representation of an app ID in a readable and short way.
// Either with the id itself or a relative path if possible.
func IDToAlias(id orchestrator.ID) string {
	v := id.String()
	res, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return v
	}

	v = string(res)
	if strings.Contains(v, ":") {
		return v
	}

	wd, err := paths.Getwd()
	if err != nil {
		return v
	}
	rel, err := paths.New(v).RelFrom(wd)
	if err != nil {
		return v
	}
	if !strings.HasPrefix(rel.String(), "./") && !strings.HasPrefix(rel.String(), "../") {
		return "./" + rel.String()
	}
	return rel.String()
}
