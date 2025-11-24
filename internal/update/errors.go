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
