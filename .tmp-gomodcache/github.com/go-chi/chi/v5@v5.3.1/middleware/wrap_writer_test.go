package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// readerFromRecorder is a ResponseRecorder that also satisfies io.ReaderFrom,
// mirroring the real net/http response writer (which implements ReadFrom and
// is what httpFancyWriter.ReadFrom fast-paths to). Bytes copied via ReadFrom
// are recorded to the body so a test can observe whether they reached the
// original writer.
type readerFromRecorder struct {
	*httptest.ResponseRecorder
}

func (r *readerFromRecorder) ReadFrom(src io.Reader) (int64, error) {
	return io.Copy(r.ResponseRecorder.Body, src)
}

func TestHttpFancyWriterRemembersWroteHeaderWhenFlushed(t *testing.T) {
	f := &httpFancyWriter{basicWriter: basicWriter{ResponseWriter: httptest.NewRecorder()}}
	f.Flush()

	if !f.wroteHeader {
		t.Fatal("want Flush to have set wroteHeader=true")
	}
}

func TestHttp2FancyWriterRemembersWroteHeaderWhenFlushed(t *testing.T) {
	f := &http2FancyWriter{basicWriter{ResponseWriter: httptest.NewRecorder()}}
	f.Flush()

	if !f.wroteHeader {
		t.Fatal("want Flush to have set wroteHeader=true")
	}
}

func TestBasicWritesTeesWritesWithoutDiscard(t *testing.T) {
	// explicitly create the struct instead of NewRecorder to control the value of Code
	original := &httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
	wrap := &basicWriter{ResponseWriter: original}

	var buf bytes.Buffer
	wrap.Tee(&buf)

	_, err := wrap.Write([]byte("hello world"))
	assertNoError(t, err)

	assertEqual(t, 200, original.Code)
	assertEqual(t, []byte("hello world"), original.Body.Bytes())
	assertEqual(t, []byte("hello world"), buf.Bytes())
	assertEqual(t, 11, wrap.BytesWritten())
}

func TestBasicWriterDiscardsWritesToOriginalResponseWriter(t *testing.T) {
	t.Run("With Tee", func(t *testing.T) {
		// explicitly create the struct instead of NewRecorder to control the value of Code
		original := &httptest.ResponseRecorder{
			HeaderMap: make(http.Header),
			Body:      new(bytes.Buffer),
		}
		wrap := &basicWriter{ResponseWriter: original}

		var buf bytes.Buffer
		wrap.Tee(&buf)
		wrap.Discard()

		_, err := wrap.Write([]byte("hello world"))
		assertNoError(t, err)

		assertEqual(t, 0, original.Code) // wrapper shouldn't call WriteHeader implicitly
		assertEqual(t, 0, original.Body.Len())
		assertEqual(t, []byte("hello world"), buf.Bytes())
		assertEqual(t, 11, wrap.BytesWritten())
	})

	t.Run("Without Tee", func(t *testing.T) {
		// explicitly create the struct instead of NewRecorder to control the value of Code
		original := &httptest.ResponseRecorder{
			HeaderMap: make(http.Header),
			Body:      new(bytes.Buffer),
		}
		wrap := &basicWriter{ResponseWriter: original}
		wrap.Discard()

		_, err := wrap.Write([]byte("hello world"))
		assertNoError(t, err)

		assertEqual(t, 0, original.Code) // wrapper shouldn't call WriteHeader implicitly
		assertEqual(t, 0, original.Body.Len())
		assertEqual(t, 11, wrap.BytesWritten())
	})
}

// TestHttpFancyWriterReadFromByteCountWithTee is a regression test for
// https://github.com/go-chi/chi/issues/1067.
// httpFancyWriter.ReadFrom was adding n to basicWriter.bytes even when the
// write went through basicWriter.Write (which already increments the counter),
// resulting in double-counting the bytes when a Tee writer was set.
func TestHttpFancyWriterReadFromByteCountWithTee(t *testing.T) {
	original := &httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
	f := &httpFancyWriter{basicWriter: basicWriter{ResponseWriter: original}}

	var teeBuf bytes.Buffer
	f.Tee(&teeBuf)

	const input = "hello world"
	n, err := f.ReadFrom(strings.NewReader(input))
	assertNoError(t, err)
	assertEqual(t, int64(len(input)), n)
	// Before the fix, BytesWritten() returned 22 (double-counted).
	assertEqual(t, len(input), f.BytesWritten())
	assertEqual(t, []byte(input), teeBuf.Bytes())
}

// TestHttpFancyWriterReadFromHonorsDiscard verifies that Discard() suppresses
// the body even when it is written via ReadFrom/io.Copy. Before the fix, the
// non-tee ReadFrom path streamed straight to the original ResponseWriter's
// ReaderFrom, bypassing the discard flag entirely.
func TestHttpFancyWriterReadFromHonorsDiscard(t *testing.T) {
	original := &readerFromRecorder{&httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}}
	f := &httpFancyWriter{basicWriter: basicWriter{ResponseWriter: original}}
	f.Discard()

	const input = "hello world"
	n, err := f.ReadFrom(strings.NewReader(input))
	assertNoError(t, err)
	assertEqual(t, int64(len(input)), n)
	// The data must NOT reach the original writer once Discard() is set.
	assertEqual(t, 0, original.Body.Len())
	// BytesWritten still reflects the bytes consumed.
	assertEqual(t, len(input), f.BytesWritten())
}
