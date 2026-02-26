// This file is part of arduino-app-cli.
//
// Copyright Copyright (C) Arduino s.r.l. and/or its affiliated companies
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
