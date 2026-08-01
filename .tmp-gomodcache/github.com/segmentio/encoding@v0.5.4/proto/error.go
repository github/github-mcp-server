package proto

import (
	"errors"
	"fmt"
)

var ErrWireTypeUnknown = errors.New("unknown wire type")

type UnmarshalFieldError struct {
	FieldNumer int
	WireType   int
	Err        error
}

func (e *UnmarshalFieldError) Error() string {
	return fmt.Sprintf("field number %d with wire type %d: %v", e.FieldNumer, e.WireType, e.Err)
}

func (e *UnmarshalFieldError) Unwrap() error { return e.Err }

func fieldError(f fieldNumber, t wireType, err error) error {
	return &UnmarshalFieldError{
		FieldNumer: int(f),
		WireType:   int(t),
		Err:        err,
	}
}
