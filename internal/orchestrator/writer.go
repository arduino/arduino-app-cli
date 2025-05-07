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
