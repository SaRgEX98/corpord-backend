package apperrors

import "net/http"

// APIError represents a standard API error format
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new API error
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

// Predefined API errors
var (
	// 4xx errors
	ErrBadRequest   = NewAPIError(http.StatusBadRequest, "bad_request", "Некорректный запрос")
	ErrUnauthorized = NewAPIError(http.StatusUnauthorized, "unauthorized", "Не авторизован")
	ErrForbidden    = NewAPIError(http.StatusForbidden, "forbidden", "Доступ запрещен")
	ErrNotFound     = NewAPIError(http.StatusNotFound, "not_found", "Ресурс не найден")

	// 5xx errors
	ErrInternal = NewAPIError(http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a standard success response
type SuccessResponse struct {
	Message string `json:"message"`
}
