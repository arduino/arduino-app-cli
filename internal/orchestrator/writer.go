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
