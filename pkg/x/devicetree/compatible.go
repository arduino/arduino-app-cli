// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package devicetree

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"strings"
)

type Compatible []string

func LoadCompatible() Compatible {
	return getCompatibleFromFS(os.DirFS("/"))
}

func (c Compatible) IsCompatibleWith(prefix string) bool {
	for _, comp := range c {
		if strings.HasPrefix(comp, prefix) {
			return true
		}
	}
	return false
}

func getCompatibleFromFS(fs fs.FS) Compatible {
	var compatibles []string
	if comp, err := fs.Open("sys/firmware/devicetree/base/compatible"); err == nil {
		defer comp.Close()
		if data, err := io.ReadAll(comp); err == nil {
			for _, c := range bytes.Split(data, []byte{'\x00'}) {
				c = bytes.Trim(c, "\x00 \t\n\r") // trim null bytes and whitespace
				if len(c) > 0 {
					compatibles = append(compatibles, string(c))
				}
			}
		}
	}
	return compatibles
}
