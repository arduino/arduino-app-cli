// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
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
