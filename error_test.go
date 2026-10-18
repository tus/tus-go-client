package tusgo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// closeTrackingBody wraps a response body and records whether it was closed
type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Read(p []byte) (int, error) {
	if b.closed {
		return 0, errors.New("read on closed body")
	}
	return b.Reader.Read(p)
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

// testResponse returns a response with the given status code and body; contentLength -1 means unknown length
func testResponse(status int, body *closeTrackingBody, contentLength int64) *http.Response {
	return &http.Response{StatusCode: status, Body: body, ContentLength: contentLength}
}

var _ = Describe("TusError", func() {
	Context("Error", func() {
		It("should join the sentinel message and the inner error", func() {
			err := newTusError(ErrProtocol, nil, errors.New("boom"))
			Ω(err.Error()).Should(Equal("protocol error: boom"))
		})
		It("should keep the inner error message built by fmt.Errorf", func() {
			err := newTusError(ErrProtocol, nil, fmt.Errorf("cannot parse %q: %w", "abc", errors.New("bad syntax")))
			Ω(err.Error()).Should(Equal(`protocol error: cannot parse "abc": bad syntax`))
		})
		It("should render a message when a bare sentinel is used", func() {
			Ω(ErrProtocol.Error()).Should(Equal("protocol error"))
		})
		It("should render a message when neither response nor inner error is given", func() {
			Ω(newTusError(ErrProtocol, nil, nil).Error()).Should(Equal("protocol error"))
		})
		It("should include the status code and the content length of the response", func() {
			err := newTusError(ErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), nil)
			Ω(err.Error()).Should(Equal("upload does not exist (HTTP 404; Content-Length=14)"))
		})
		It("should include the unknown content length of the response", func() {
			err := newTusError(ErrTooLarge, testResponse(http.StatusRequestEntityTooLarge, &closeTrackingBody{Reader: strings.NewReader("too large")}, -1), nil)
			Ω(err.Error()).Should(Equal("data is too large (HTTP 413; Content-Length=-1)"))
		})
		It("should not include the response body", func() {
			err := newTusError(ErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), nil)
			Ω(err.Error()).ShouldNot(ContainSubstring("no such upload"))
		})
		It("should include both the response and the inner error", func() {
			err := newTusError(ErrProtocol, testResponse(http.StatusOK, &closeTrackingBody{Reader: strings.NewReader("")}, 0), errors.New("boom"))
			Ω(err.Error()).Should(Equal("protocol error (HTTP 200; Content-Length=0): boom"))
		})
		It("should be usable as a wrapped error message", func() {
			err := fmt.Errorf("upload failed: %w", newTusError(ErrCannotUpload, nil, errors.New("boom")))
			Ω(err.Error()).Should(Equal("upload failed: can not upload: boom"))
		})
	})

	Context("Unwrap", func() {
		It("should return the inner error", func() {
			inner := errors.New("boom")
			unwrapped := newTusError(ErrProtocol, nil, inner).Unwrap()
			Ω(unwrapped).Should(MatchError(inner))
			Ω(unwrapped.Error()).Should(Equal("boom"))
		})
		It("should return nil for a bare sentinel", func() {
			Ω(ErrProtocol.Unwrap()).Should(BeNil())
		})
		It("should return nil if neither response nor inner error is given", func() {
			Ω(newTusError(ErrProtocol, nil, nil).Unwrap()).Should(BeNil())
		})
		It("should return nil if the response body has been read successfully", func() {
			err := newTusError(ErrProtocol, testResponse(http.StatusOK, &closeTrackingBody{Reader: strings.NewReader("body")}, 4), nil)
			Ω(err.Unwrap()).Should(BeNil())
		})
	})

	Context("errors.Is", func() {
		It("should match the sentinel it was built from", func() {
			var err error = newTusError(ErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, ErrProtocol)).Should(BeTrue())
		})
		It("should not match a different sentinel", func() {
			var err error = newTusError(ErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeFalse())
		})
		It("should match regardless of the inner error the sentinel carries", func() {
			a := newTusError(ErrTooLarge, nil, errors.New("first"))
			b := newTusError(ErrTooLarge, nil, errors.New("second"))
			Ω(errors.Is(a, b)).Should(BeTrue())
			Ω(errors.Is(b, a)).Should(BeTrue())
		})
		It("should match regardless of the response the sentinel carries", func() {
			var err error = newTusError(ErrOffsetsNotSynced, testResponse(http.StatusConflict, &closeTrackingBody{Reader: strings.NewReader("offset mismatch")}, 15), nil)
			Ω(errors.Is(err, ErrOffsetsNotSynced)).Should(BeTrue())
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeFalse())
		})
		It("should match a bare sentinel against itself", func() {
			Ω(errors.Is(ErrProtocol, ErrProtocol)).Should(BeTrue())
		})
		It("should not match a bare sentinel against another one", func() {
			Ω(errors.Is(ErrProtocol, ErrChecksumMismatch)).Should(BeFalse())
		})
		It("should match the inner error", func() {
			inner := errors.New("boom")
			var err error = newTusError(ErrProtocol, nil, inner)
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match an error wrapped deeper in the inner chain", func() {
			inner := errors.New("boom")
			var err error = newTusError(ErrProtocol, nil, fmt.Errorf("context: %w", inner))
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match both sentinels when one TusError carries another", func() {
			var err error = newTusError(ErrProtocol, nil, ErrCannotUpload)
			Ω(errors.Is(err, ErrProtocol)).Should(BeTrue())
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeTrue())
		})
		It("should survive being wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newTusError(ErrUploadDoesNotExist, nil, errors.New("boom")))
			Ω(errors.Is(err, ErrUploadDoesNotExist)).Should(BeTrue())
			Ω(errors.Is(err, ErrProtocol)).Should(BeFalse())
		})
		It("should not match an unrelated error", func() {
			var err error = newTusError(ErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, io.EOF)).Should(BeFalse())
		})
	})

	Context("errors.As", func() {
		It("should extract a TusError with its message, inner error and response", func() {
			inner := errors.New("boom")
			resp := testResponse(http.StatusOK, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			var err error = newTusError(ErrProtocol, resp, inner)

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
			Ω(target.inner).Should(MatchError(inner))
			Ω(target.Response()).Should(HaveHTTPStatus(http.StatusOK))
		})
		It("should extract a TusError wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newTusError(ErrChecksumMismatch, nil, errors.New("boom")))

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("checksum mismatch"))
		})
		It("should extract the outermost TusError of a nested chain", func() {
			var err error = newTusError(ErrProtocol, nil, ErrCannotUpload)

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
		})
		It("should fail for an error that is not a TusError", func() {
			var target TusError
			Ω(errors.As(errors.New("boom"), &target)).Should(BeFalse())
		})
	})

	Context("newTusError", func() {
		It("should not modify the sentinel it was built from", func() {
			_ = newTusError(ErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), errors.New("boom"))
			Ω(ErrUploadDoesNotExist.inner).Should(BeNil())
			Ω(ErrUploadDoesNotExist.resp).Should(BeNil())
		})
		It("should join the inner error to the inner error of an already contextualized error", func() {
			first := newTusError(ErrProtocol, nil, errors.New("first"))
			second := newTusError(first, nil, errors.New("second"))

			Ω(second.Error()).Should(Equal("protocol error: first\nsecond"))
			Ω(first.Error()).Should(Equal("protocol error: first"))
		})
	})

	Context("Response", func() {
		It("should return nil if no response is given", func() {
			Ω(newTusError(ErrProtocol, nil, errors.New("boom")).Response()).Should(BeNil())
		})
		It("should return nil for a bare sentinel", func() {
			Ω(ErrProtocol.Response()).Should(BeNil())
		})
		It("should return a copy of the given response with the body read into a buffer", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("no such upload")}
			resp := testResponse(http.StatusNotFound, body, 14)
			resp.Header = http.Header{"X-Foo": {"bar"}}

			err := newTusError(ErrUploadDoesNotExist, resp, nil)
			Ω(err.Response()).ShouldNot(BeIdenticalTo(resp))
			Ω(err.Response()).Should(And(
				HaveHTTPStatus(http.StatusNotFound),
				HaveHTTPHeaderWithValue("X-Foo", "bar"),
				HaveHTTPBody([]byte("no such upload")),
			))
			Ω(err.Response().ContentLength).Should(BeEquivalentTo(14))
		})
		It("should leave the original response body to the caller", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("no such upload")}
			resp := testResponse(http.StatusNotFound, body, 14)

			_ = newTusError(ErrUploadDoesNotExist, resp, nil)
			Ω(resp.Body).Should(BeIdenticalTo(body))
			Ω(body.closed).Should(BeFalse())
		})
		It("should keep an empty body", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("")}
			resp := testResponse(http.StatusNotFound, body, 0)

			err := newTusError(ErrUploadDoesNotExist, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte{}))
			Ω(body.closed).Should(BeFalse())
		})
		It("should keep a body up to the read limit untruncated", func() {
			data := strings.Repeat("x", bodyReadLimit)
			body := &closeTrackingBody{Reader: strings.NewReader(data)}
			resp := testResponse(http.StatusRequestEntityTooLarge, body, int64(len(data)))

			err := newTusError(ErrTooLarge, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte(data)))
		})
		It("should truncate a body longer than the read limit and leave the rest in the original body", func() {
			body := &closeTrackingBody{Reader: strings.NewReader(strings.Repeat("x", bodyReadLimit) + strings.Repeat("y", 100))}
			resp := testResponse(http.StatusRequestEntityTooLarge, body, -1)

			err := newTusError(ErrTooLarge, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte(strings.Repeat("x", bodyReadLimit))))
			Ω(body.closed).Should(BeFalse())
			Ω(resp).Should(HaveHTTPBody([]byte(strings.Repeat("y", 100))))
		})
		It("should only keep the part of the body that has not been read yet", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("no such upload")}
			resp := testResponse(http.StatusNotFound, body, 14)
			Ω(io.ReadFull(resp.Body, make([]byte, len("no such ")))).Should(Equal(len("no such ")))

			err := newTusError(ErrUploadDoesNotExist, resp, nil)
			Ω(err.Response()).Should(HaveHTTPBody([]byte("upload")))
		})
		It("should report a body that cannot be read", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("boom")}
			Ω(body.Close()).Should(Succeed())
			resp := testResponse(http.StatusInternalServerError, body, 4)

			err := newTusError(ErrUnexpectedResponse, resp, nil)
			Ω(err).Should(And(
				MatchError(ErrUnexpectedResponse),
				MatchError("unexpected HTTP response code (HTTP 500; Content-Length=4): read response body: read on closed body"),
			))
			Ω(err.Response()).ShouldNot(BeIdenticalTo(resp))
			Ω(err.Response()).Should(HaveHTTPStatus(http.StatusInternalServerError))
			Ω(err.Response()).Should(HaveHTTPBody([]byte{}))
			Ω(resp.Body).Should(BeIdenticalTo(body))
		})
		It("should report a body that cannot be read along with the inner error", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("boom")}
			Ω(body.Close()).Should(Succeed())
			resp := testResponse(http.StatusOK, body, 4)
			inner := errors.New("bad header")

			err := newTusError(ErrProtocol, resp, inner)
			Ω(err).Should(And(
				MatchError(ErrProtocol),
				MatchError(inner),
				MatchError("protocol error (HTTP 200; Content-Length=4): read response body: read on closed body\nbad header"),
			))
		})
	})
})
