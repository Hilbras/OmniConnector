package contracts

import "errors"

// Error implements the error interface so integrations can return a
// normalized *Error directly from Execute and other contract methods.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Category) + ": " + e.Message
}

// NewError builds a normalized error. Retryable is derived from the category
// and can be overridden by the caller afterwards.
func NewError(category ErrorCategory, code, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Category:  category,
		Retryable: category == ErrorCategoryRateLimited || category == ErrorCategoryNetwork || category == ErrorCategoryTimeout,
	}
}

// AsError extracts a normalized *Error from err, reporting whether one was found.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e, true
	}
	return nil, false
}
