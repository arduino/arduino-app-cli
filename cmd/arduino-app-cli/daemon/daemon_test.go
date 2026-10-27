// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package daemon

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

func TestStartDefaultAppTriesAgainUntilStarted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		begin := time.Now()
		startDefaultApp(t.Context(), func() error {
			attempts++
			if attempts < 3 {
				return errors.New("exit status 1")
			}
			return nil
		})

		require.Equal(t, 3, attempts)
		require.Equal(t, 10*time.Second+20*time.Second, time.Since(begin))
	})
}

func TestStartDefaultAppGivesUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		begin := time.Now()
		startDefaultApp(t.Context(), func() error {
			attempts++
			return errors.New("exit status 1")
		})

		require.Equal(t, 6, attempts)
		require.Equal(t, (10+20+40+80+160)*time.Second, time.Since(begin))
	})
}

func TestStartDefaultAppStopsWhenAnotherAppIsRunning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		startDefaultApp(t.Context(), func() error {
			attempts++
			return &orchestrator.AppRunningError{Name: "other"}
		})

		require.Equal(t, 1, attempts)
	})
}

func TestStartDefaultAppStopsWithTheDaemon(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		attempts := 0
		startDefaultApp(ctx, func() error {
			attempts++
			cancel()
			return errors.New("exit status 1")
		})

		require.Equal(t, 1, attempts)
	})
}
