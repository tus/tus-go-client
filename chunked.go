package tusgo

import (
	"errors"
	"fmt"
	"io"
)

const maxConsecutiveZeroWrites = 3

type streamWriter interface {
	Write(p []byte) (n int, err error)
	Size() int64
	Tell() int64
}

// NewChunkedWriter returns a ChunkedWriter that uploads data through the stream s in chunks of chunkSize bytes.
func NewChunkedWriter(s streamWriter, chunkSize int) *ChunkedWriter {
	if chunkSize <= 0 {
		panic(fmt.Sprintf("invalid chunk size %d, must be positive", chunkSize))
	}
	if s == nil {
		panic("stream is nil")
	}
	return &ChunkedWriter{
		s:   s,
		buf: make([]byte, chunkSize),
	}
}

// ChunkedWriter is an io.ReaderFrom that uploads the data read from an [io.Reader] in chunks of a fixed size,
// sending each chunk with a single Write call on the underlying stream.
// ChunkedWriter is not safe for concurrent use.
//
// Each chunk is read into an internal buffer and kept there until the server confirms all of it. If the upload was
// interrupted, a repeated ReadFrom call with the same reader resends the unconfirmed data first and then continues
// reading, so the reader does not need to be rewound.
type ChunkedWriter struct {
	s streamWriter

	// buf is the intermediate buffer where len(buf) equal to the chunk size.
	buf []byte
	// bufPtr is a pointer to the data part in buf unconfirmed by the server, because the server may confirm
	// only a part of the data sent in a request.
	// len(bufPtr) equal to 0 means the all data is confirmed by the server.
	bufPtr []byte
}

// ReadFrom reads data from r and uploads it in chunks until r returns [io.EOF] or the upload is full. It
// implements [io.ReaderFrom] and returns the number of bytes read from r. Each chunk is uploaded with
// [ChunkedWriter.Write].
//
// If the upload size is known, ReadFrom never reads more from r than the space left in the upload, so any data
// beyond it stays unread. If the upload has a deferred size that has not been declared yet
// (see [Stream.DeclareSize]), ReadFrom reads until r returns [io.EOF] or the server rejects a chunk.
//
// If the server accepts only part of a chunk, ReadFrom resends the rest. If the server accepts no data in
// several consecutive requests, ReadFrom returns [ErrIncompleteWrite].
//
// On error, ReadFrom returns immediately and keeps the data the server has not accepted in the internal buffer
// (see [ChunkedWriter.Buffer]). A later call to ReadFrom with the same r sends the buffered data first, so r does
// not need to be rewound.
func (us *ChunkedWriter) ReadFrom(r io.Reader) (int64, error) {
	counterRd := &counterReader{Rd: r}
	r = counterRd

	// Preliminary limit the reader to read only the space left in the upload (if upload size is known).
	if us.s.Size() != SizeUnknown {
		freeSpace := us.s.Size() - us.s.Tell() - int64(len(us.bufPtr))
		r = io.LimitReader(r, freeSpace)
	}

	// Load the data in chunks until the upload becomes full or reader is exhausted, whatever happens first.
	for us.s.Size() == SizeUnknown || us.s.Tell() < us.s.Size() {
		if len(us.bufPtr) == 0 {
			n, err := io.ReadFull(r, us.buf)
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

	return counterRd.BytesRead, nil
}

// Write uploads p in chunks, issuing one Stream.Write call per chunk. It returns the number of bytes accepted by
// the server.
//
// If the upload size is known, Write never sends more than the space left in the upload. If p does not fit, Write
// uploads as much as fits and returns [ErrTooLarge]. If the upload has a deferred size that has not been declared
// yet (see [Stream.DeclareSize]), Write sends p until the server either accepts all of it or rejects a chunk.
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

	deferSize := us.s.Size() == SizeUnknown
	freeSpace := int(us.s.Size() - us.s.Tell())
	for len(p) > 0 && (deferSize || freeSpace > 0) {
		nn := min(len(p), len(us.buf))
		if nn > freeSpace && !deferSize {
			nn = freeSpace
		}

		advance, err := us.s.Write(p[:nn])
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
		freeSpace = int(us.s.Size() - us.s.Tell())
	}

	if n < n0 {
		return n, newTusError(ErrTooLarge, nil, fmt.Errorf("upload is full, %d bytes of %d were sent", n, n0))
	}
	return n, nil
}

// Flush sends the unconfirmed data in the buffer to the server. It returns the number of bytes acknowledged by the
// server and an error if the upload failed.
//
// Flush returns all errors returned by the underlying Stream.Write. Call Flush again to resend the unconfirmed data
// if the error occurred.
func (us *ChunkedWriter) Flush() (int, error) {
	if len(us.bufPtr) == 0 {
		return 0, nil
	}

	advance, err := us.s.Write(us.bufPtr)
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
