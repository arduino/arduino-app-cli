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
