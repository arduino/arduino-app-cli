package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/pkg/render"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func monitorStream(mon net.Conn, ws *websocket.Conn) {
	logWebsocketError := func(msg string, err error) {
		// Do not log simple close or interruption errors
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
			if e, ok := err.(*websocket.CloseError); ok {
				slog.Error(msg, slog.String("closecause", fmt.Sprintf("%d: %s", e.Code, err)))
			} else {
				slog.Error(msg, slog.String("error", err.Error()))
			}
		}
	}
	logSocketError := func(msg string, err error) {
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			slog.Error(msg, slog.String("error", err.Error()))
		}
	}
	go func() {
		defer mon.Close()
		defer ws.Close()
		for {
			// Read from websocket and write to monitor
			_, msg, err := ws.ReadMessage()
			if err != nil {
				logWebsocketError("Error reading from websocket", err)
				return
			}
			if _, err := mon.Write(msg); err != nil {
				logSocketError("Error writing to monitor", err)
				return
			}
		}
	}()
	go func() {
		defer mon.Close()
		defer ws.Close()
		buff := [1024]byte{}
		for {
			// Read from monitor and write to websocket
			n, err := mon.Read(buff[:])
			if err != nil {
				logSocketError("Error reading from monitor", err)
				return
			}

			if err := ws.WriteMessage(websocket.BinaryMessage, buff[:n]); err != nil {
				logWebsocketError("Error writing to websocket", err)
				return
			}
		}
	}()
}

func HandleMonitorWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Connect to monitor
		mon, err := net.DialTimeout("tcp", "127.0.0.1:7500", time.Second)
		if err != nil {
			slog.Error("Unable to connect to monitor", slog.String("error", err.Error()))
			render.EncodeResponse(w, http.StatusServiceUnavailable, models.ErrorResponse{Details: "Unable to connect to monitor: " + err.Error()})
			return
		}

		// Upgrade the connection to websocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Remember to close monitor connection if websocket upgrade fails.
			mon.Close()

			render.EncodeResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to upgrade connection"})
			return
		}

		// Now the connection is managed by the websocket library, let's move the handlers in the goroutine
		go monitorStream(mon, conn)

		// and return nothing to the http library
	}
}
