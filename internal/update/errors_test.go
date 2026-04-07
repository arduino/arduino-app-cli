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
