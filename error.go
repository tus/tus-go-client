package tusgo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"
)

// bodyReadLimit is the maximum number of the response body bytes included in an error message
const bodyReadLimit = 256

// TusError represents an error occurred during a TUS operation.
type TusError struct {
	inner error
	msg   string
}

func (te TusError) Error() string {
	if te.inner == nil {
		return te.msg
	}
	return fmt.Sprintf("%s: %s", te.msg, te.inner)
}

func (te TusError) Unwrap() error {
	return te.inner
}

func (te TusError) Is(e error) bool {
	v, ok := e.(TusError)
	return ok && v.msg == te.msg || errors.Is(te.inner, e)
}

func newTusErrorWithErr(te TusError, inner error) TusError {
	te.inner = inner
	return te
}

func newTusErrorWithResponse(te TusError, r *http.Response) TusError {
	if r == nil {
		te.inner = fmt.Errorf("response is nil")
		return te
	}

	b := make([]byte, bodyReadLimit)
	if l, err := io.ReadFull(r.Body, b); err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		size := r.Header.Get("Content-Length")
		if size == "" {
			size = "?"
		}
		var body string
		if !utf8.Valid(b[:l]) {
			body = fmt.Sprintf("%v", b[:l]) // if the body is not valid UTF-8, print it as a byte slice
		} else {
			body = string(b[:l])
		}
		switch {
		case err == nil:
			te.inner = fmt.Errorf("HTTP %d: body (%s bytes): %s... (truncated)", r.StatusCode, size, body)
		case l > 0:
			te.inner = fmt.Errorf("HTTP %d: body (%s bytes): %s", r.StatusCode, size, body)
		default:
			te.inner = fmt.Errorf("HTTP %d: empty body", r.StatusCode)
		}
	} else {
		te.inner = fmt.Errorf("HTTP %d: read body: %w", r.StatusCode, err)
	}
	return te
}

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
