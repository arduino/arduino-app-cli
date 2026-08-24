// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e"
	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func GetHttpclient(t *testing.T, opts ...e2e.ArduinoAppCLIOption) *client.ClientWithResponses {
	t.Helper()
	c, _ := GetHttpclientAndAddr(t, opts...)
	return c
}

// GetHttpclientAndAddr returns the HTTP client together with the daemon base URL.
// Use this when you need to make raw HTTP requests (e.g. SSE streams).
func GetHttpclientAndAddr(t *testing.T, opts ...e2e.ArduinoAppCLIOption) (*client.ClientWithResponses, string) {
	t.Helper()
	cli := e2e.CreateEnvForDaemon(t, opts...)
	t.Cleanup(cli.CleanUp)
	httpClient, err := client.NewClientWithResponses(cli.DaemonAddr)
	require.NoError(t, err)
	return httpClient, cli.DaemonAddr
}

func newSSEClient(req *http.Request, lastEventID int64) (events chan Event, err error) {

	if lastEventID > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", lastEventID))
	}
	resp, err := http.DefaultClient.Do(req) //nolint
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got response status code %d", resp.StatusCode)
	}
	events = make(chan Event)
	go loop(resp.Body, events)
	return events, nil
}

type Event struct {
	ID    string
	Event string
	Data  []byte // json
}

func loop(r io.ReadCloser, events chan Event) {
	defer r.Close()
	reader := bufio.NewReader(r)

	evt := Event{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			close(events)
			return
		}
		switch {
		case strings.HasPrefix(line, "data:"):
			evt.Data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case strings.HasPrefix(line, "event:"):
			evt.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "id:"):
			evt.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "\n"):
			events <- evt
		default:
			fmt.Fprintf(os.Stderr, "Unknown line: '%s'", line)
			close(events)
		}
	}
}
