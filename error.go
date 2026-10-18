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

	// ErrTooLarge is returned when creating an upload that is larger than the server accepts, or when sending more
	// data than the upload can contain.
	ErrTooLarge = TusError{msg: "data is too large"}

	// ErrUploadDoesNotExist is returned when the upload is not found on the server or the access to it is denied.
	ErrUploadDoesNotExist = TusError{msg: "upload does not exist"}

	// ErrOffsetsNotSynced indicates that stream's offset does not match the remote offset on the server.
	// This error may occur when sending the data from the start to the upload that already contains some data.
	// Or when the server has accepted data, but its response was lost.
	//
	// To fix this error, call stream's Sync method and try again.
	ErrOffsetsNotSynced = TusError{msg: "client stream and server offsets are not synced"}

	// ErrChecksumMismatch is returned when the server has detected a corruption of the transferred data.
	ErrChecksumMismatch = TusError{msg: "checksum mismatch"}

	// ErrIncompleteWrite is returned when the server responds successfully but rejects some or all sent data.
	// This may indicate the server-side backpressure, which means the server is overloaded. Retry after a short delay.
	ErrIncompleteWrite = TusError{msg: "incomplete write"}

	// ErrCannotUpload is returned when the server explicitly refuses to accept any data for an existing upload.
	// For example, this may happen while trying to transfer data to a "final" upload that has been concatenated from
	// partial uploads, which is forbidden by the TUS protocol.
	ErrCannotUpload = TusError{msg: "can not upload"}

	// ErrProtocol is a general error that is returned when an otherwise successful server response contains malformed data.
	// This error may indicate that the server does not support the requested TUS version, or server misbehaves.
	ErrProtocol = TusError{msg: "protocol error"}

	// ErrUnexpectedResponse is a general error that returned when the server has responded with an unexpected status code.
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
