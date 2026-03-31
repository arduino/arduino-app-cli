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

package render

import (
	"encoding/json"
	"fmt"
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
