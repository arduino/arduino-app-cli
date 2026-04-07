// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
