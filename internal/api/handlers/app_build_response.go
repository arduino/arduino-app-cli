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
	"github.com/arduino/arduino-app-cli/internal/render"
)

// buildHeartbeatInterval keeps alive the clients that drop a request left without data.
const buildHeartbeatInterval = 15 * time.Second

// buildResponse is a multipart/mixed body: empty parts while the build runs, then the
// release archive or the error of the build as the last part.
type buildResponse struct {
	w      http.ResponseWriter
	writer *multipart.Writer

	stopOnce sync.Once
	stop     chan struct{}
	stopped  chan struct{}
}

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
				if _, err := b.writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain"}}); err != nil {
					return
				}
				b.flush()
			}
		}
	}()
	return b
}

func (b *buildResponse) SendArchive(archive *paths.Path, fileName string) error {
	b.Close()

	file, err := archive.Open()
	if err != nil {
		b.SendError(models.ErrorResponse{Code: string(render.InternalServiceErr), Details: "unable to read the release archive"})
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
	// No closing boundary if the copy fails: the client can tell the archive is truncated.
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	return b.writer.Close()
}

func (b *buildResponse) SendError(response models.ErrorResponse) {
	b.Close()

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

// Close stops the heartbeat. It does not end the body: SendArchive and SendError do.
// It can be called more than once.
func (b *buildResponse) Close() {
	b.stopOnce.Do(func() { close(b.stop) })
	<-b.stopped
}

func (b *buildResponse) flush() {
	_ = http.NewResponseController(b.w).Flush()
}
