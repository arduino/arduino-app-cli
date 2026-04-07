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

package orchestrator

import (
	"bytes"
)

// CallbackWriter is a custom writer that processes each line calling the callback.
type CallbackWriter struct {
	callback func(line string)
	buffer   []byte
}

// NewCallbackWriter creates a new CallbackWriter.
func NewCallbackWriter(process func(line string)) *CallbackWriter {
	return &CallbackWriter{
		callback: process,
		buffer:   make([]byte, 0, 1024),
	}
}

// Write implements the io.Writer interface.
func (p *CallbackWriter) Write(data []byte) (int, error) {
	p.buffer = append(p.buffer, data...)
	for {
		idx := bytes.IndexByte(p.buffer, '\n')
		if idx == -1 {
			break
		}
		line := p.buffer[:idx] // Do not include \n
		p.buffer = p.buffer[idx+1:]
		p.callback(string(line))
	}
	return len(data), nil
}
