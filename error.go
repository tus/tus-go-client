package tusgo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	// ErrUnsupportedFeature is returned when an action requires a TUS extension the server does not support.
	ErrUnsupportedFeature = TusError{msg: "unsupported feature"}

	// ErrUploadTooLarge is returned when creating an upload that is larger than the server accepts.
	ErrUploadTooLarge = TusError{msg: "upload is too large"}

	// ErrUploadDoesNotExist is returned when the upload is not found on the server or the access to it is denied.
	ErrUploadDoesNotExist = TusError{msg: "upload does not exist"}

	// ErrOffsetsNotSynced is returned when the server expects the data at an offset other than the local one.
	// Call UploadStream.Sync to adopt the server offset.
	ErrOffsetsNotSynced = TusError{msg: "client stream and server offsets are not synced"}

	// ErrChecksumMismatch is returned when the server has detected a corruption of the transferred data.
	ErrChecksumMismatch = TusError{msg: "checksum mismatch"}

	// ErrProtocol is returned when an otherwise successful server response contains malformed data.
	// This error may indicate that the server does not support the requested TUS version, or server misbehaves.
	ErrProtocol = TusError{msg: "protocol error"}

	// ErrZeroProgress is returned by UploadStream in chunked mode, indicating that the server did not
	// accept any of the data and did not return an error. This may happen when client is sending data too fast and the
	// server is overloaded, or if the server has a bug or misconfiguration.
	ErrZeroProgress = TusError{msg: "zero progress"}

	// ErrCannotUpload is returned when the server explicitly refuses to accept the data for an existing upload.
	// This error may occur if the server returned 403 Forbidden when trying to upload data.
	ErrCannotUpload = TusError{msg: "can not upload"}

	// ErrUnexpectedResponse is returned when the server has responded with an unexpected status code.
	// This error may indicate that the server does not support the TUS protocol, or the response is originated
	// by a proxy server or CDN.
	ErrUnexpectedResponse = TusError{msg: "unexpected HTTP response code"}
)

// bodyReadLimit is the maximum number of the response body bytes included in an error message
const bodyReadLimit = 2 * 1024 // 2 KiB

// TusError represents an error occurred during a TUS operation.
type TusError struct {
	inner error
	resp  *http.Response
	msg   string
}

func (te TusError) Error() string {
	var b strings.Builder

	b.WriteString(te.msg)
	if te.resp != nil {
		fmt.Fprintf(&b, " (HTTP %d; Content-Length=%d)", te.resp.StatusCode, te.resp.ContentLength)
	}
	if te.inner != nil {
		fmt.Fprintf(&b, ": %s", te.inner.Error())
	}
	return b.String()
}

func (te TusError) Unwrap() error {
	return te.inner
}

// Is reports whether the error is equal to the target error.
func (te TusError) Is(e error) bool {
	v, ok := e.(TusError)
	return ok && v.msg == te.msg || errors.Is(te.inner, e)
}

// Response returns the copy of the HTTP response that caused the error, or nil if no response has been made.
// The response body is already closed and contains up to first 2KiB of the original response body if it was available.
func (te TusError) Response() *http.Response {
	return te.resp
}

func newTusError(te TusError, r *http.Response, e error) TusError {
	// Read the response body up to bodyReadLimit byte
	if r != nil {
		r2 := *r
		te.resp = &r2

		b := make([]byte, bodyReadLimit)
		if l, err := io.ReadFull(r2.Body, b); err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			r2.Body = io.NopCloser(bytes.NewReader(b[:l]))
		} else {
			te.inner = fmt.Errorf("read response body: %w", err)
			r2.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}

	if e != nil {
		te.inner = errors.Join(te.inner, e)
	}

	return te
}
