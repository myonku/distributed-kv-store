package errors

import "errors"

var (
	ErrNotLeader         = errors.New("not leader")
	UnSupportedOperation = errors.New("unsupported operation")
	UnSupportedMode      = errors.New("unsupported mode")
)
