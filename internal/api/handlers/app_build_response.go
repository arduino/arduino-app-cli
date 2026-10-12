// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sync"
	"time"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/api/models"
)

// buildHeartbeatInterval is how often a build response sends an empty part while the
// archive is not ready: a client that drops a request left without data keeps this one.
const buildHeartbeatInterval = 15 * time.Second

// buildResponse is the multipart/mixed body of a build request. It is answered at once
// with a 200 and kept alive with empty parts until the build is over: the last part is
// the release archive, or the error of the build as JSON, the status being already sent.
type buildResponse struct {
	w      http.ResponseWriter
	writer *multipart.Writer

	stopOnce sync.Once
	stop     chan struct{}
	stopped  chan struct{}
}

// startBuildResponse sends the status and the headers, and starts the heartbeat.
func startBuildResponse(w http.ResponseWriter, heartbeat time.Duration) *buildResponse {
	b := &buildResponse{
		w:       w,
		writer:  multipart.NewWriter(w),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	w.Header().Set("Content-Type", mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": b.writer.Boundary()}))
	w.WriteHeader(http.StatusOK)
	b.flush()

	go func() {
		defer close(b.stopped)
		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-b.stop:
				return
			case <-ticker.C:
				// A failure here is a client that left: the build is canceled with its request.
				if _, err := b.writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain"}}); err != nil {
					return
				}
				b.flush()
			}
		}
	}()
	return b
}

// SendArchive ends the response with the release archive.
func (b *buildResponse) SendArchive(archive *paths.Path, fileName string) error {
	b.stopHeartbeat()

	file, err := archive.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := b.writer.CreatePart(textproto.MIMEHeader{
		"Content-Type":        {"application/gzip"},
		"Content-Disposition": {fmt.Sprintf(`attachment; filename=%q`, fileName)},
	})
	if err != nil {
		return err
	}
	// Left without its closing boundary if the copy fails, for the client to tell that the
	// archive is not whole.
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	return b.writer.Close()
}

// SendError ends the response with the error of the build, in place of the archive.
func (b *buildResponse) SendError(response models.ErrorResponse) {
	b.stopHeartbeat()

	part, err := b.writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	if err == nil {
		err = json.NewEncoder(part).Encode(response)
	}
	if err == nil {
		err = b.writer.Close()
	}
	if err != nil {
		slog.Error("unable to send the build error", slog.String("error", err.Error()))
	}
}

// stopHeartbeat waits for the heartbeat to be over: the parts are written one at a time.
func (b *buildResponse) stopHeartbeat() {
	b.stopOnce.Do(func() { close(b.stop) })
	<-b.stopped
}

func (b *buildResponse) flush() {
	_ = http.NewResponseController(b.w).Flush()
}
