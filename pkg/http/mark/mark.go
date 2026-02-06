// Package mark provides a mechanism for tagging errors with a well-known error value.
package mark

import "errors"

// This list of errors is not exhaustive, but is a good starting point for most
// applications. Feel free to add more as needed, but don't go overboard.
// Remember, the specific types of errors are only important so far as someone
// calling your code might want to write logic to handle each type of error
// differently.
//
// Do not add application-specific errors to this list. Instead, just define
// your own package with your own application-specific errors, and use this
// package to mark errors with them. The errors in this package are not special,
// they're just plain old errors.
//
// Not all errors need to be marked. An error that is not marked should be
// treated as an unexpected error that cannot be handled by calling code. This
// is often the case for network errors or logic errors.
var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrBadRequest      = errors.New("bad request")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrCancelled       = errors.New("request cancelled")
	ErrUnavailable     = errors.New("unavailable")
	ErrTimedout        = errors.New("request timed out")
	ErrTooLarge        = errors.New("request is too large")
	ErrTooManyRequests = errors.New("too many requests")
	ErrForbidden       = errors.New("forbidden")
)

// With wraps err with another error that will return true from errors.Is and
// errors.As for both err and markErr, and anything either may wrap.
func With(err, markErr error) error {
	if err == nil {
		return nil
	}
	return marked{wrapped: err, mark: markErr}
}

type marked struct {
	wrapped error
	mark    error
}

func (f marked) Is(target error) bool {
	// if this is false, errors.Is will call unwrap and retry on the wrapped
	// error.
	return errors.Is(f.mark, target)
}

func (f marked) As(target any) bool {
	// if this is false, errors.As will call unwrap and retry on the wrapped
	// error.
	return errors.As(f.mark, target)
}

func (f marked) Unwrap() error {
	return f.wrapped
}

func (f marked) Error() string {
	return f.mark.Error() + ": " + f.wrapped.Error()
}
