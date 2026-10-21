// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckUser(t *testing.T) {
	for _, test := range []struct {
		name      string
		uid       int
		allowRoot bool
		accountID string
		lookupErr error
		wantError string
	}{
		{name: "root is rejected", uid: 0, accountID: "1001", wantError: "non-root arduino user"},
		{name: "root override is preserved", uid: 0, allowRoot: true},
		{name: "original UID is accepted", uid: 1000, accountID: "1000"},
		{name: "allocated UID is accepted", uid: 1001, accountID: "1001"},
		{name: "unrelated UID 1000 is rejected", uid: 1000, accountID: "1001", wantError: "non-root arduino user"},
		{name: "missing account is reported", uid: 1001, lookupErr: errors.New("account not found"), wantError: "account not found"},
		{name: "root arduino account is rejected", uid: 0, accountID: "0", wantError: "non-root arduino user"},
		{name: "development override is preserved", uid: 501, allowRoot: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkUser(test.uid, test.allowRoot, func(name string) (*user.User, error) {
				require.False(t, test.allowRoot, "the override must not require an arduino account")
				require.Equal(t, "arduino", name)
				return &user.User{Uid: test.accountID}, test.lookupErr
			})
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
