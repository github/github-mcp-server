package log

import (
	"fmt"
	"time"
)

// Field is a key-value pair for structured logging.
// This type is backend-agnostic — adapters convert it to their native field type.
type Field struct {
	Key   string
	Value any
}

// Convenience constructors for common field types.

func String(key, value string) Field                 { return Field{Key: key, Value: value} }
func Int(key string, value int) Field                { return Field{Key: key, Value: value} }
func Int64(key string, value int64) Field            { return Field{Key: key, Value: value} }
func Float64(key string, value float64) Field        { return Field{Key: key, Value: value} }
func Bool(key string, value bool) Field              { return Field{Key: key, Value: value} }
func Err(err error) Field                            { return Field{Key: "error", Value: err} }
func Duration(key string, value time.Duration) Field { return Field{Key: key, Value: value} }
func Any(key string, value any) Field                { return Field{Key: key, Value: value} }

// Stringer returns the string representation of a Field.
func (f Field) String() string {
	return fmt.Sprintf("%s=%v", f.Key, f.Value)
}
