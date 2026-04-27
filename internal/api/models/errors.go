package models

type ErrorResponse struct {
	Code    string `json:"code,omitempty"`
	Details string `json:"details"`
}
