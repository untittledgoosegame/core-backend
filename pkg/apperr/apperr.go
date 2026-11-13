package apperr

import "net/http"

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func New(status int, message string) *Error { return &Error{Status: status, Message: message} }

func BadRequest(message string) *Error   { return New(http.StatusBadRequest, message) }
func Unauthorized(message string) *Error { return New(http.StatusUnauthorized, message) }
func NotFound(message string) *Error     { return New(http.StatusNotFound, message) }
func Conflict(message string) *Error     { return New(http.StatusConflict, message) }
func TooMany(message string) *Error      { return New(http.StatusTooManyRequests, message) }
func Unavailable(message string) *Error  { return New(http.StatusServiceUnavailable, message) }
