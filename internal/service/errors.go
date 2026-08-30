package service

import (
	"errors"
	"strconv"
)

// Sentinel business errors mapped to HTTP codes by the handler layer.
var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
)

// errf wraps a sentinel with a human-readable message.
func errf(base error, msg string) error {
	return &wrapped{base: base, msg: msg}
}

func errorsBadRequest(msg string) error {
	if msg == "" {
		return ErrBadRequest
	}
	return errf(ErrBadRequest, msg)
}

func errNotFound(id uint) error {
	return &wrapped{base: ErrNotFound, msg: "record not found: id=" + strconv.FormatUint(uint64(id), 10)}
}

// wrapped carries a sentinel + message; handler layer matches on the sentinel.
type wrapped struct {
	base error
	msg  string
}

func (w *wrapped) Error() string { return w.msg }
func (w *wrapped) Unwrap() error { return w.base }

// itob renders a material id as a string for error messages.
func itob(v uint) string { return strconv.FormatUint(uint64(v), 10) }
