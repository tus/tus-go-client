package tusgo

import (
	"errors"
	"fmt"
	"io"
)

const maxConsecutiveZeroWrites = 3

// NewChunkedWriter returns a ChunkedWriter that writes to w in chunks of at most chunkSize bytes. The internal
// buffer of chunkSize bytes is allocated once and reused for the lifetime of the writer.
//
// w is typically a [*Stream]. Any other [io.Writer] must follow the [Stream.Write] contract: return the number of bytes
// the server has accepted, and [ErrIncompleteWrite] if it is less than len(p).
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

// ChunkedWriter writes data to an underlying stream, typically a [Stream], in fixed-size chunks.
// It's optimized for reliable and resumable writing the data to the underlying stream.
//
// [ChunkedWriter.ReadFrom] stages each chunk in an internal buffer and keeps the unaccepted data there on error,
// so a later call with the same reader resumes the process without rewinding it. [ChunkedWriter.Write] sends
// the caller's slice in chunks, without buffering, but returns count of bytes accepted by the stream.
// In both cases, if the stream accepts only part of a chunk, ChunkedWriter sends the rest before it moves on.
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

// ReadFrom reads data from r and uploads it in chunks until r returns [io.EOF] or the upload is full. It
// implements [io.ReaderFrom] and returns the number of bytes read from r. ReadFrom sends the data until r returns
// [io.EOF] or the server rejects a chunk.
//
// If the server accepts only part of a chunk, ReadFrom resends the rest. If the server accepts no data in
// several consecutive requests, ReadFrom returns [ErrIncompleteWrite].
//
// On error, ReadFrom returns immediately and keeps the data the server has not accepted in the internal buffer
// (see [ChunkedWriter.Buffer]). A later call to ReadFrom with the same r sends the buffered data first, so r does
// not need to be rewound.
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

// Write uploads p in chunks, issuing one Stream.Write call per chunk. It always returns the number of bytes
// accepted by the server and possible error. Write sends p until the server confirms all chunks or rejects a chunk.
//
// If the server accepts only part of a chunk, Write resends the rest. If the server accepts no data in
// several consecutive requests, Write returns [ErrIncompleteWrite].
//
// On error, Write returns immediately and does not keep the unaccepted data. To continue the upload, call Write
// again with p[n:].
func (us *ChunkedWriter) Write(p []byte) (int, error) {
	var n int
	n0 := len(p)
	var zeroAttempts int

	for len(p) > 0 {
		nn := min(len(p), len(us.buf))
		advance, err := us.w.Write(p[:nn])
		if err == nil || errors.Is(err, ErrIncompleteWrite) || errors.Is(err, ErrTooLarge) {
			p = p[advance:]
			n += advance
		}
		if err != nil && !errors.Is(err, ErrIncompleteWrite) {
			return n, fmt.Errorf("stream write: %w", err)
		}
		if advance == 0 {
			zeroAttempts++
			if zeroAttempts >= maxConsecutiveZeroWrites {
				return n, newTusError(ErrIncompleteWrite, nil, fmt.Errorf("server did not receive data too many times: %d", zeroAttempts))
			}
		} else {
			zeroAttempts = 0
		}
	}

	if n < n0 {
		return n, newTusError(ErrTooLarge, nil, fmt.Errorf("upload is full, %d bytes of %d were sent", n, n0))
	}
	return n, nil
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
	if advance < 0 {
		advance = 0
	}
	if err == nil || errors.Is(err, ErrIncompleteWrite) || errors.Is(err, ErrTooLarge) {
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
