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

package render

import (
	"encoding/json"
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
