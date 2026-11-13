package skills

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// DefaultPageSize is the number of entries a page carries when the caller does
// not choose otherwise.
const DefaultPageSize = 50

// cursorPrefix versions the opaque cursor so its encoding can change without a
// client mistaking an old cursor for a new one. Clients must treat the cursor
// as opaque; nothing here is part of the wire contract.
const cursorPrefix = "skills.v1:"

// ErrInvalidCursor reports a cursor that this server did not issue, or issued
// under an encoding it no longer accepts.
var ErrInvalidCursor = errors.New("invalid pagination cursor")

// encodeCursor renders an offset as an opaque cursor.
func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursorPrefix + strconv.Itoa(offset)))
}

// decodeCursor recovers the offset from a cursor this server issued.
func decodeCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidCursor, cursor)
	}
	rest, ok := strings.CutPrefix(string(raw), cursorPrefix)
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrInvalidCursor, cursor)
	}
	offset, err := strconv.Atoi(rest)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidCursor, cursor)
	}
	return offset, nil
}

// paginate slices one page out of an ordered collection and returns the cursor
// for the next page, empty when the page is the last.
//
// Slicing is by whole elements, so a skill entry is never split across pages:
// a page carries an entry's complete resource manifest or does not carry the
// entry at all.
//
// An offset past the end yields an empty final page rather than an error. The
// collection may have shrunk since the cursor was issued, and a client that
// walks off the end of a changed catalog should see the walk finish, not fail.
func paginate[T any](items []T, cursor string, pageSize int) (page []T, nextCursor string, err error) {
	offset := 0
	if cursor != "" {
		offset, err = decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if offset >= len(items) {
		return []T{}, "", nil
	}
	end := offset + pageSize
	if end >= len(items) {
		return items[offset:], "", nil
	}
	return items[offset:end], encodeCursor(end), nil
}
