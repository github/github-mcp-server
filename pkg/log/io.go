package log

import (
	"io"

	"log/slog"
)

// IOLogger is a wrapper around io.Reader and io.Writer that can be used
// to log the data being read and written from the underlying streams
type IOLogger struct {
	io.ReadWriteCloser

	reader io.Reader
	writer io.Writer
	logger *slog.Logger
}

// NewIOLogger creates a new IOLogger instance
func NewIOLogger(r io.Reader, w io.Writer, logger *slog.Logger) *IOLogger {
	return &IOLogger{
		reader: r,
		writer: w,
		logger: logger,
	}
}

// Read reads data from the underlying io.Reader and logs it.
func (l *IOLogger) Read(p []byte) (n int, err error) {
	if l.reader == nil {
		return 0, io.EOF
	}
	n, err = l.reader.Read(p)
	if n > 0 {
		l.logger.Info("[stdin]: received bytes", "count", n, "data", string(p[:n]))
	}
	return n, err
}

// Write writes data to the underlying io.Writer and logs it.
func (l *IOLogger) Write(p []byte) (n int, err error) {
	if l.writer == nil {
		return 0, io.ErrClosedPipe
	}
	l.logger.Info("[stdout]: sending bytes", "count", len(p), "data", string(p))
	return l.writer.Write(p)
}

func (l *IOLogger) Close() error {
	var errReader, errWriter error
	if closer, ok := l.reader.(io.Closer); ok {
		errReader = closer.Close()
	}
	if closer, ok := l.writer.(io.Closer); ok {
		errWriter = closer.Close()
	}
	if errReader != nil {
		return errReader
	}
	return errWriter
}
