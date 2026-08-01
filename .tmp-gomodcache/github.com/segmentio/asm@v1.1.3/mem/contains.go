package mem

import "bytes"

func containsGeneric(haystack []byte, needle byte) bool {
	return bytes.IndexByte(haystack, needle) != -1
}
