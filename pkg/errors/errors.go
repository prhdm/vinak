package errors

import (
	"fmt"
	"net/http"
)

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	ErrorType  string `json:"error"`
}

func NewAPIError(statusCode int, message string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		ErrorType:  http.StatusText(statusCode),
	}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.StatusCode, e.ErrorType, e.Message)
}

// Common API Errors
var (
	ErrBadRequest         = NewAPIError(http.StatusBadRequest, "Invalid request")
	ErrUnauthorized       = NewAPIError(http.StatusUnauthorized, "Unauthorized access")
	ErrForbidden          = NewAPIError(http.StatusForbidden, "Access forbidden")
	ErrNotFound           = NewAPIError(http.StatusNotFound, "Resource not found")
	ErrInternalServer     = NewAPIError(http.StatusInternalServerError, "Internal server error")
	ErrServiceUnavailable = NewAPIError(http.StatusServiceUnavailable, "Service temporarily unavailable")
)

// Payment Gateway Errors
type PaymentGatewayError struct {
	Gateway string
	Err     error
}

func NewPaymentGatewayError(gateway string, err error) *PaymentGatewayError {
	return &PaymentGatewayError{
		Gateway: gateway,
		Err:     err,
	}
}

func (e *PaymentGatewayError) Error() string {
	return fmt.Sprintf("payment gateway %s error: %v", e.Gateway, e.Err)
}

// Validation Errors
type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}
