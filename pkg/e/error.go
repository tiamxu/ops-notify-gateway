package e

import "fmt"

const (
	CodeBadRequest    = 40001
	CodeUnauthorized  = 40101
	CodeNotFound      = 40401
	CodeInternal      = 50001
	CodeSendFailed    = 50002
	CodeConfigInvalid = 50003
)

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func BadRequest(format string, args ...interface{}) *Error {
	return New(CodeBadRequest, fmt.Sprintf(format, args...))
}

func Internal(format string, args ...interface{}) *Error {
	return New(CodeInternal, fmt.Sprintf(format, args...))
}

func SendFailed(format string, args ...interface{}) *Error {
	return New(CodeSendFailed, fmt.Sprintf(format, args...))
}
