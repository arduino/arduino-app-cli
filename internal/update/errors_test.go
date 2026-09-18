// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateError(t *testing.T) {
	t.Run("known error", func(t *testing.T) {
		var err error = &UpdateError{
			Code:    NoInternetConnectionCode,
			Details: "no internet connection available",
		}
		assert.Equal(t, "no internet connection available", err.Error())
		assert.Equal(t, "no internet connection available", fmt.Sprintf("%s", err))
		assert.Equal(t, NoInternetConnectionCode, GetUpdateErrorCode(err))
	})

	t.Run("unknown error", func(t *testing.T) {
		var underlyingErr = errors.New("underlying error")
		var updateErr error = NewUnkownError(underlyingErr)

		assert.Equal(t, "underlying error", updateErr.Error())
		assert.Equal(t, "underlying error", fmt.Sprintf("%s", updateErr))
		assert.Equal(t, underlyingErr, errors.Unwrap(updateErr))
		assert.True(t, errors.Is(updateErr, underlyingErr))
		assert.Equal(t, UnknownErrorCode, GetUpdateErrorCode(updateErr))
	})
}
