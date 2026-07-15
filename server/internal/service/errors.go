package service

import "errors"

var (
	ErrForbidden      = errors.New("forbidden")
	ErrNotFound       = errors.New("not_found")
	ErrInvalidInput   = errors.New("invalid_input")
	ErrConflict       = errors.New("conflict")
	ErrContentTooLong = errors.New("content too long")
)
