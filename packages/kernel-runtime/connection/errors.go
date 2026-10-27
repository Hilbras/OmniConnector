package connection

import "errors"

var (
	ErrMissingID = errors.New("connection ID is required")
	ErrDuplicate = errors.New("connection already exists")
	ErrNotFound  = errors.New("connection not found")
)
