// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package dockerhelper

import "bytes"

// CallbackWriter is an io.Writer that invokes a callback for each newline-delimited line.
type CallbackWriter struct {
	callback func(line string)
	buffer   []byte
}

// NewCallbackWriter creates a new CallbackWriter.
func NewCallbackWriter(callback func(line string)) *CallbackWriter {
	return &CallbackWriter{
		callback: callback,
		buffer:   make([]byte, 0, 1024),
	}
}

// Write implements the io.Writer interface.
func (w *CallbackWriter) Write(data []byte) (int, error) {
	w.buffer = append(w.buffer, data...)
	for {
		idx := bytes.IndexByte(w.buffer, '\n')
		if idx == -1 {
			break
		}
		line := w.buffer[:idx]
		w.buffer = w.buffer[idx+1:]
		w.callback(string(line))
	}
	return len(data), nil
}
