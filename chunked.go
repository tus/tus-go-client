package tusgo

import (
	"errors"
	"fmt"
	"io"
)

// NewChunkedWriter returns a ChunkedWriter that writes to w in chunks of at most chunkSize bytes. The internal
// buffer of chunkSize bytes is allocated once and reused for the lifetime of the writer.
//
// NewChunkedWriter panics if w is nil or chunkSize is not positive.
func NewChunkedWriter(w io.Writer, chunkSize int) *ChunkedWriter {
	if chunkSize <= 0 {
		panic(fmt.Sprintf("invalid chunk size %d, must be positive", chunkSize))
	}
	if w == nil {
		panic("stream is nil")
	}
	return &ChunkedWriter{
		w:   w,
		buf: make([]byte, chunkSize),
	}
}

// ChunkedWriter is a buffered io.Writer, that writes data to an underlying writer in fixed-size chunks.
// It's optimized for reliable and resumable writing the data to the underlying writer.
//
// [ChunkedWriter.ReadFrom] stages each chunk in an internal buffer and keeps the unaccepted data there on error,
// so a later call with the same reader resumes the process without rewinding it. [ChunkedWriter.Write] sends
// the caller's slice in chunks, without buffering, but returns count of bytes accepted by the writer.
// In both cases, if the writer accepts only part of a chunk during the transmission, ChunkedWriter stops immediately
// with possibility to resume transmission.
//
// A ChunkedWriter is not safe for concurrent use.
type ChunkedWriter struct {
	w io.Writer

	// buf is the intermediate buffer where len(buf) equal to the chunk size.
	buf []byte
	// bufPtr is a pointer to the data part in buf unconfirmed by the server, because the server may confirm
	// only a part of the data sent in a request.
	// len(bufPtr) equal to 0 means the all data is confirmed by the server.
	bufPtr []byte
}

// ReadFrom implements [io.ReaderFrom]. It reads r in chunks through the internal buffer and writes each chunk
// to the underlying writer until r returns [io.EOF]. It returns the number of bytes read from r and the first
// error encountered (if any).
//
// On a write error, the unaccepted data stays in the buffer (see [ChunkedWriter.Buffer]) and is written first
// on the next call, so r does not need to be rewound.
func (us *ChunkedWriter) ReadFrom(r io.Reader) (int64, error) {
	counterRd := &counterReader{Rd: r}

	for {
		if len(us.bufPtr) == 0 {
			n, err := io.ReadFull(counterRd, us.buf)
			us.bufPtr = us.buf[:n]
			switch {
			case errors.Is(err, io.EOF): // Reader is empty
				return counterRd.BytesRead, nil
			case errors.Is(err, io.ErrUnexpectedEOF): // Reader has ended early, upload the last chunk
			case err != nil:
				return counterRd.BytesRead, fmt.Errorf("read: %w", err)
			}
		}

		// Here always len(us.bufPtr) > 0. Write returns only the confirmed bytes, so the rest stays in the buffer.
		n, err := us.Write(us.bufPtr)
		us.bufPtr = us.bufPtr[n:]
		if err != nil {
			return counterRd.BytesRead, fmt.Errorf("write: %w", err)
		}
	}
}

// Write writes p to the underlying writer in chunks, one call per chunk, and returns
// the total number of bytes accepted and the first error encountered (if any).
// Write does not use the internal buffer.
//
// On error, Write returns immediately; to resume, call Write again with p[n:].
//
// If the underlying writer misbehaves and accepts only part of a chunk without error, Write returns immediately with
// the total number of bytes accepted (including a faulty write) and [io.ErrShortWrite] error.
func (us *ChunkedWriter) Write(p []byte) (int, error) {
	var n int

	for len(p) > 0 {
		nn := min(len(p), len(us.buf))
		advance, err := us.w.Write(p[:nn])
		if advance > 0 {
			p = p[advance:]
			n += advance
		}
		if err != nil {
			return n, fmt.Errorf("write: %w", err)
		}
		if advance < nn {
			return n, fmt.Errorf("write: %w: expected %d, got %d", io.ErrShortWrite, nn, advance)
		}
	}

	return n, nil
}

// Advance discards the first n bytes of the unconfirmed data in the internal buffer (see [ChunkedWriter.Buffer]).
// It reports whether n is within the buffered data, that is, 0 <= n <= len(Buffer()). Otherwise, Advance empties
// the buffer and returns false.
//
// Advance is intended for recovering from [ErrOffsetsNotSynced] while using ReadFrom. That error means either
// the server rejected the request because the client offset was not equal to server offset, or the server stored
// the data but the response confirming it was lost. In the latter case the buffer still holds data the server
// already has, and the next write would send it again, appending a duplicate at the following offset.
//
// To prevent that, pass the delta returned by [Stream.Sync] to Advance before resuming the transfer. If Advance
// returns false, the buffered write is broken: the server offset lies outside the buffered data. The source
// reader must be repositioned to [Stream.Tell]:
//
//	d, err := stream.Sync()
//	if err != nil {
//		return fmt.Errorf("sync: %w", err)
//	}
//	if !cw.Advance(d) {
//		// The buffer is now empty; seek the source to stream.Tell() before calling cw.ReadFrom again.
//	}
func (us *ChunkedWriter) Advance(n int64) bool {
	if n < 0 || n > int64(len(us.bufPtr)) {
		us.bufPtr = nil
		return false
	}
	us.bufPtr = us.bufPtr[n:]
	return true
}

// Flush sends the unconfirmed data in the buffer to the server. It returns the number of bytes acknowledged by the
// server and an error if the upload failed.
//
// Flush returns all errors returned by the underlying writer. Call Flush again to resend the unconfirmed data
// if the error occurred.
func (us *ChunkedWriter) Flush() (int, error) {
	if len(us.bufPtr) == 0 {
		return 0, nil
	}

	advance, err := us.w.Write(us.bufPtr)
	if advance > 0 {
		us.bufPtr = us.bufPtr[advance:]
	}
	if err != nil {
		return advance, fmt.Errorf("write: %w", err)
	}

	return advance, nil
}

// Buffer returns the unconfirmed data in internal buffer. The returned slice is valid until the next call
// to ReadFrom or Flush.
// If its length is 0, the buffer is empty and all data sent to the server is confirmed.
func (us *ChunkedWriter) Buffer() []byte {
	return us.bufPtr
}

// Reset clears the buffer.
func (us *ChunkedWriter) Reset() {
	us.bufPtr = nil
}
