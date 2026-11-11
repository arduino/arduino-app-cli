// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package appbook

import (
	"fmt"
)

type Status string

const (
	StatusStarting      Status = "starting"
	StatusRunning       Status = "running"
	StatusStopping      Status = "stopping"
	StatusStopped       Status = "stopped"
	StatusFailed        Status = "failed"
	StatusUninitialized Status = "uninitialized"
)

func ParseStatus(s string) (Status, error) {
	s1 := Status(s)
	return s1, s1.Validate()
}

func (s Status) Validate() error {
	switch s {
	case StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed, StatusUninitialized:
		return nil
	default:
		return fmt.Errorf("status should be one of %v", s.AllowedStatuses())
	}
}

func (s Status) AllowedStatuses() []Status {
	return []Status{StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed, StatusUninitialized}
}
