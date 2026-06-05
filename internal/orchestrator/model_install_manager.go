// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package orchestrator

import "sync"

type ModelInstallEventType string

const (
	// Container-emitted event types (match the Python script's event field).
	ModelInstallEventStart    ModelInstallEventType = "start"
	ModelInstallEventUpdate   ModelInstallEventType = "update"
	ModelInstallEventComplete ModelInstallEventType = "complete"
	ModelInstallEventInfo     ModelInstallEventType = "info"
	ModelInstallEventError    ModelInstallEventType = "error"
	// Synthetic event emitted by Go after the container exits.
	ModelInstallEventDone ModelInstallEventType = "done"
)

type ModelInstallEvent struct {
	ModelID     string                `json:"model_id"`
	Type        ModelInstallEventType `json:"type"`
	Description string                `json:"description,omitempty"`
	Current     int64                 `json:"current,omitempty"`
	Total       int64                 `json:"total,omitempty"`
	Unit        string                `json:"unit,omitempty"`
	Percentage  string                `json:"percentage,omitempty"`
	Artifacts   []string              `json:"artifacts,omitempty"`
}

// ModelInstallManager is a global pub/sub hub for model install progress events.
// All subscribers receive events for every model; each event carries its ModelID
// so the frontend can filter by model.
type ModelInstallManager struct {
	mu   sync.Mutex
	subs map[chan ModelInstallEvent]struct{}
}

func NewModelInstallManager() *ModelInstallManager {
	return &ModelInstallManager{subs: make(map[chan ModelInstallEvent]struct{})}
}

func (m *ModelInstallManager) Subscribe() chan ModelInstallEvent {
	ch := make(chan ModelInstallEvent, 100)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes the channel from the list of subscribers and closes it.
func (m *ModelInstallManager) Unsubscribe(ch chan ModelInstallEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, ch)
	close(ch)
}

func (m *ModelInstallManager) broadcast(modelID string, event ModelInstallEvent) {
	event.ModelID = modelID
	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subs {
		select {
		case ch <- event:
		default: // drop if subscriber is not keeping up
		}
	}
}
