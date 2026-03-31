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

import "errors"

type ErrorCode string

// TODO: add the error to the openAPI spec as an enum
const (
	NoInternetConnectionCode ErrorCode = "NO_INTERNET_CONNECTION"
	OperationInProgressCode  ErrorCode = "OPERATION_IN_PROGRESS"
	UnknownErrorCode         ErrorCode = "UNKNOWN_ERROR"
)

var (
	ErrOperationAlreadyInProgress = &UpdateError{
		Code:    OperationInProgressCode,
		Details: "an operation is already in progress",
	}
	ErrNoInternetConnection = &UpdateError{
		Code:    NoInternetConnectionCode,
		Details: "no internet connection available",
	}
)

type UpdateError struct {
	Code    ErrorCode `json:"code"`
	Details string    `json:"details"`

	err error
}

func (e *UpdateError) Error() string {
	return e.Details
}

func (e *UpdateError) Unwrap() error {
	return e.err
}

func NewUnkownError(err error) *UpdateError {
	return &UpdateError{
		Details: err.Error(),
		err:     err,
	}
}

func GetUpdateErrorCode(err error) ErrorCode {
	var updateError *UpdateError
	if errors.As(err, &updateError) {
		if updateError.Code != "" {
			return updateError.Code
		}
	}
	return UnknownErrorCode
}
