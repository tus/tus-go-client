package rufh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	ErrIncompleteWrite = RUFHError{msg: "incomplete write"}
	ErrProtocolError   = RUFHError{msg: "protocol error"}
)

func newTusError() error { // TODO: this is placeholder for now
	panic("implement me")
}

// bodyReadLimit is the maximum number of the response body bytes included in an error message
const bodyReadLimit = 2 * 1024 // 2 KiB

// RUFHError represents an error occurred during a RUFH operation. It also may contain the HTTP response that caused the
// error, if any.
type RUFHError struct {
	inner error
	resp  *http.Response
	msg   string
}

func (re RUFHError) Error() string {
	var b strings.Builder

	b.WriteString(re.msg)
	if re.resp != nil {
		b.WriteRune(':')
		if re.resp.Request != nil && re.resp.Request.URL != nil {
			fmt.Fprintf(&b, " %s %s", re.resp.Request.Method, re.resp.Request.URL)
		}
		fmt.Fprintf(&b, " (HTTP %d: Content-Length=%d)", re.resp.StatusCode, re.resp.ContentLength)
	}
	if re.inner != nil {
		fmt.Fprintf(&b, ": %s", re.inner.Error())
	}
	return b.String()
}

func (re RUFHError) Unwrap() error {
	return re.inner
}

// Is reports whether the error is equal to the target error.
func (re RUFHError) Is(e error) bool {
	v, ok := e.(RUFHError)
	return ok && v.msg == re.msg || errors.Is(re.inner, e)
}

// Response returns the copy of the HTTP response that caused the error, or nil if no response has been made.
// The response body is already closed and contains up to first 2KiB of the original response body if it was available.
func (re RUFHError) Response() *http.Response {
	return re.resp
}

// StatusCode is a convenience method that returns the HTTP status code of the response that caused the error,
// or 0 if no response has been made.
//
// Typical usage:
//
//	var re rufh.RUFHError
//	if errors.As(err, &re) && re.StatusCode() == http.StatusInternalServerError {
//	    log.Printf("internal server error: %v", re)
//	}
func (re RUFHError) StatusCode() int {
	if re.resp != nil {
		return re.resp.StatusCode
	}
	return 0
}

func newRUFHError(re RUFHError, r *http.Response, e error) RUFHError {
	// Read the response body up to bodyReadLimit byte
	if r != nil {
		r2 := *r
		re.resp = &r2

		b := make([]byte, bodyReadLimit)
		if l, err := io.ReadFull(r2.Body, b); err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			r2.Body = io.NopCloser(bytes.NewReader(b[:l]))
		} else {
			re.inner = fmt.Errorf("read response body: %w", err)
			r2.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}

	if e != nil {
		re.inner = errors.Join(re.inner, e)
	}

	return re
}
