package tusgo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"
)

// bodyReadLimit is the maximum amount of the response body bytes that is included to an error message
const bodyReadLimit = 256

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
	ErrUnsupportedFeature = TusError{msg: "unsupported feature"}
	ErrUploadTooLarge     = TusError{msg: "upload is too large"}
	ErrUploadDoesNotExist = TusError{msg: "upload does not exist"}
	ErrOffsetsNotSynced   = TusError{msg: "client stream and server offsets are not synced"}
	ErrChecksumMismatch   = TusError{msg: "checksum mismatch"}
	ErrProtocol           = TusError{msg: "protocol error"}
	ErrCannotUpload       = TusError{msg: "can not upload"}
	ErrUnexpectedResponse = TusError{msg: "unexpected HTTP response code"}
)
