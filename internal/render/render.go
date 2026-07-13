// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func EncodeResponse(w http.ResponseWriter, statusCode int, resp any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(statusCode)
	if resp == nil {
		return
	}
	// 204 status code doesn't allow sending body. This will prevent possible
	// missuse of the EncodeResponse function.
	if statusCode == http.StatusNoContent {
		return
	}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("encode response", slog.Any("err", err))
	}
}

func EncodeByteResponse(w http.ResponseWriter, statusCode int, resp []byte) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.WriteHeader(statusCode)
	if resp == nil {
		return
	}
	// 204 status code doesn't allow sending body. This will prevent possible
	// missuse of the EncodeResponse function.
	if statusCode == http.StatusNoContent {
		return
	}
	_, _ = w.Write(resp)
}

func EncodeZipResponse(w http.ResponseWriter, statusCode int, resp []byte, filename string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w.WriteHeader(statusCode)

	if resp == nil {
		return
	}

	if _, err := w.Write(resp); err != nil {
		slog.Error("failed to write zip response", slog.String("error", err.Error()))
	}
}

func EncodeTarGzResponse(w http.ResponseWriter, statusCode int, resp []byte, filename string) {
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w.WriteHeader(statusCode)

	if resp == nil {
		return
	}

	if _, err := w.Write(resp); err != nil {
		slog.Error("failed to write tar.gz response", slog.String("error", err.Error()))
	}
}

// EncodeTarGzStream streams a .tar.gz body from r to the client without buffering it in
// memory. Use it for potentially large payloads (e.g. releases bundling AI models) where
// EncodeTarGzResponse's full []byte would risk OOM on memory-constrained devices.
func EncodeTarGzStream(w http.ResponseWriter, statusCode int, r io.Reader, filename string) {
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	w.WriteHeader(statusCode)

	if r == nil {
		return
	}

	if _, err := io.Copy(w, r); err != nil {
		slog.Error("failed to stream tar.gz response", slog.String("error", err.Error()))
	}
}
