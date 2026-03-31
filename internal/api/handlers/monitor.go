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

package handlers

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/monitor"
	"github.com/arduino/arduino-app-cli/internal/render"
)

func HandleMonitorWS(allowedOrigins []string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return checkOrigin(r.Header.Get("Origin"), allowedOrigins)
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade the connection to websocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Remember to close monitor connection if websocket upgrade fails.

			slog.Error("Failed to upgrade connection", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to upgrade connection: " + err.Error()})
			return
		}

		// Now the connection is managed by the websocket library, let's move the handlers in the goroutine
		start, err := monitor.NewMonitorHandler(&wsReadWriteCloser{conn: conn})
		if err != nil {
			slog.Error("Unable to start monitor handler", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusInternalServerError, models.ErrorResponse{Details: "Unable to start monitor handler: " + err.Error()})
			return
		}
		go start()

		// and return nothing to the http library
	}
}

func splitOrigin(origin string) (scheme, host, port string, err error) {
	parts := strings.SplitN(origin, "://", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid origin format: %s", origin)
	}
	scheme = parts[0]
	hostPort := parts[1]
	hostParts := strings.SplitN(hostPort, ":", 2)
	host = hostParts[0]
	if len(hostParts) == 2 {
		port = hostParts[1]
	} else {
		port = "*"
	}
	return scheme, host, port, nil
}

func checkOrigin(origin string, allowedOrigins []string) bool {
	scheme, host, port, err := splitOrigin(origin)
	if err != nil {
		slog.Error("WebSocket origin check failed", slog.String("origin", origin), slog.String("error", err.Error()))
		return false
	}
	for _, allowed := range allowedOrigins {
		allowedScheme, allowedHost, allowedPort, err := splitOrigin(allowed)
		if err != nil {
			panic(err)
		}
		if allowedScheme != scheme {
			continue
		}
		if allowedHost != host && allowedHost != "*" {
			continue
		}
		if allowedPort != port && allowedPort != "*" {
			continue
		}
		return true
	}
	slog.Error("WebSocket origin check failed", slog.String("origin", origin))
	return false
}

type wsReadWriteCloser struct {
	conn *websocket.Conn

	buff []byte
}

func (w *wsReadWriteCloser) Read(p []byte) (n int, err error) {
	if len(w.buff) > 0 {
		n = copy(p, w.buff)
		w.buff = w.buff[n:]
		return n, nil
	}

	ty, message, err := w.conn.ReadMessage()
	if err != nil {
		return 0, mapWebSocketErrors(err)
	}
	if ty != websocket.BinaryMessage && ty != websocket.TextMessage {
		return
	}
	n = copy(p, message)
	w.buff = message[n:]
	return n, nil
}

func (w *wsReadWriteCloser) Write(p []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, mapWebSocketErrors(err)
	}
	return len(p), nil
}

func (w *wsReadWriteCloser) Close() error {
	w.buff = nil
	return w.conn.Close()
}

func mapWebSocketErrors(err error) error {
	if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
		return net.ErrClosed
	}
	return err
}
