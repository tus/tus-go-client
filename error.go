package tusgo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

// TusError represents an error occurred during a TUS operation. It also may contain the HTTP response that caused the
// error, if any.
type TusError struct {
	inner error
	resp  *http.Response
	msg   string
}

func (te TusError) Error() string {
	var b strings.Builder

	b.WriteString(te.msg)
	if te.resp != nil {
		fmt.Fprintf(&b, " (HTTP %d; Content-Length=%d", te.resp.StatusCode, te.resp.ContentLength)
		if te.resp.Header.Get("Upload-Offset") != "" {
			fmt.Fprintf(&b, "; Upload-Offset=%s", te.resp.Header.Get("Upload-Offset"))
		}
		b.WriteString(")")
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

// StatusCode is a convenience method that returns the HTTP status code of the response that caused the error,
// or 0 if no response has been made.
//
// Typical usage:
//
//	var te tusgo.TusError
//	if errors.As(err, &te) && te.StatusCode() == http.StatusInternalServerError {
//	    log.Printf("internal server error: %v", te)
//	}
func (te TusError) StatusCode() int {
	if te.resp != nil {
		return te.resp.StatusCode
	}
	return 0
}

// IsErrorRetryable reports whether the operation that returned err may succeed if repeated.
//
// The following errors are retryable:
//
//   - [ErrIncompleteWrite], [ErrChecksumMismatch] and [ErrOffsetsNotSynced]. The latter requires calling
//     [Stream.Sync] before repeating
//   - [ErrUnexpectedResponse] with status 408, 423, 429, 500, 502, 503 or 504
//   - network errors, such as connection failures, timeouts and connections closed by the server
//
// Other errors, including [context.Canceled] and an expired [context.Context] deadline, are not retryable.
func IsErrorRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}

	var te TusError
	if errors.As(err, &te) {
		switch te.msg {
		case ErrIncompleteWrite.msg, ErrChecksumMismatch.msg, ErrOffsetsNotSynced.msg:
			return true
		case ErrUnexpectedResponse.msg:
			switch te.StatusCode() {
			case http.StatusRequestTimeout, http.StatusLocked, http.StatusTooManyRequests, http.StatusInternalServerError,
				http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return true
			}
		}
		return false
	}

	// url.Error, returned by http.Client, implements net.Error itself, so check the error it wraps
	var ue *url.Error
	if errors.As(err, &ue) {
		if errors.Is(ue.Err, io.EOF) || errors.Is(ue.Err, io.ErrUnexpectedEOF) { // Connection closed by the server
			return true
		}
		err = ue.Err
	}
	var de *net.DNSError
	if errors.As(err, &de) {
		return !de.IsNotFound
	}
	var ne net.Error
	if errors.As(err, &ne) {
		// context.DeadlineExceeded is a net.Error too, but it means the caller's deadline has passed.
		// http.Client.Timeout is reported with a different error.
		return ne != context.DeadlineExceeded
	}
	return false
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
