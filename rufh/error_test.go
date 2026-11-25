package rufh

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// foreignError is a stub of an error type from another package (e.g. tusgo.TusError) that has the same shape
// as RUFHError and also matches by message in its Is method
type foreignError struct {
	msg string
}

func (fe foreignError) Error() string {
	return fe.msg
}

func (fe foreignError) Is(e error) bool {
	v, ok := e.(foreignError)
	return ok && v.msg == fe.msg
}

var (
	foreignErrProtocol     = foreignError{msg: "protocol error"}
	foreignErrCannotUpload = foreignError{msg: "can not upload"}
)

// Test-only sentinels, since the package does not define its own yet
var (
	testErrTooLarge           = RUFHError{msg: "data is too large"}
	testErrUploadDoesNotExist = RUFHError{msg: "upload does not exist"}
	testErrOffsetsNotSynced   = RUFHError{msg: "client stream and server offsets are not synced"}
	testErrChecksumMismatch   = RUFHError{msg: "checksum mismatch"}
	testErrIncompleteWrite    = RUFHError{msg: "incomplete write"}
	testErrCannotUpload       = RUFHError{msg: "can not upload"}
	testErrProtocol           = RUFHError{msg: "protocol error"}
	testErrUnexpectedResponse = RUFHError{msg: "unexpected HTTP response code"}
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

var _ = Describe("RUFHError", func() {
	Context("Error", func() {
		It("should join the sentinel message and the inner error", func() {
			err := newRUFHError(testErrProtocol, nil, errors.New("boom"))
			Ω(err.Error()).Should(Equal("protocol error: boom"))
		})
		It("should keep the inner error message built by fmt.Errorf", func() {
			err := newRUFHError(testErrProtocol, nil, fmt.Errorf("cannot parse %q: %w", "abc", errors.New("bad syntax")))
			Ω(err.Error()).Should(Equal(`protocol error: cannot parse "abc": bad syntax`))
		})
		It("should render a message when a bare sentinel is used", func() {
			Ω(testErrProtocol.Error()).Should(Equal("protocol error"))
		})
		It("should render a message when neither response nor inner error is given", func() {
			Ω(newRUFHError(testErrProtocol, nil, nil).Error()).Should(Equal("protocol error"))
		})
		DescribeTable("should include the status code and the content length of the response, but not the Upload-Offset header",
			func(re RUFHError, status int, contentLength int64, header http.Header, inner error, expect string) {
				resp := testResponse(status, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, contentLength)
				resp.Header = header

				Ω(newRUFHError(re, resp, inner).Error()).Should(Equal(expect))
			},
			Entry("no headers", testErrUploadDoesNotExist, http.StatusNotFound, int64(14), nil, nil,
				"upload does not exist: (HTTP 404: Content-Length=14)"),
			Entry("unknown content length", testErrTooLarge, http.StatusRequestEntityTooLarge, int64(-1), nil, nil,
				"data is too large: (HTTP 413: Content-Length=-1)"),
			Entry("Upload-Offset", testErrOffsetsNotSynced, http.StatusConflict, int64(14), http.Header{"Upload-Offset": {"512"}}, nil,
				"client stream and server offsets are not synced: (HTTP 409: Content-Length=14)"),
			Entry("zero Upload-Offset", testErrIncompleteWrite, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"0"}}, nil,
				"incomplete write: (HTTP 204: Content-Length=0)"),
			Entry("malformed Upload-Offset", testErrProtocol, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"abc"}}, nil,
				"protocol error: (HTTP 204: Content-Length=0)"),
			Entry("empty Upload-Offset", testErrProtocol, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {""}}, nil,
				"protocol error: (HTTP 204: Content-Length=0)"),
			Entry("other headers only", testErrProtocol, http.StatusOK, int64(0), http.Header{"Upload-Length": {"1024"}, "X-Foo": {"bar"}}, nil,
				"protocol error: (HTTP 200: Content-Length=0)"),
			Entry("inner error", testErrProtocol, http.StatusOK, int64(0), nil, errors.New("boom"),
				"protocol error: (HTTP 200: Content-Length=0): boom"),
			Entry("Upload-Offset and inner error", testErrIncompleteWrite, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"100"}}, errors.New("boom"),
				"incomplete write: (HTTP 204: Content-Length=0): boom"),
		)
		It("should include the method and the URL of the request", func() {
			resp := testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			resp.Request = httptest.NewRequest(http.MethodPatch, "http://example.com/files/abc?x=1", nil)

			Ω(newRUFHError(testErrUploadDoesNotExist, resp, errors.New("boom")).Error()).
				Should(Equal("upload does not exist: PATCH http://example.com/files/abc?x=1 (HTTP 404: Content-Length=0): boom"))
		})
		It("should omit the method and the URL if the request has no URL", func() {
			resp := testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			resp.Request = &http.Request{Method: http.MethodPatch}

			Ω(newRUFHError(testErrUploadDoesNotExist, resp, nil).Error()).
				Should(Equal("upload does not exist: (HTTP 404: Content-Length=0)"))
		})
		It("should not include the response body", func() {
			err := newRUFHError(testErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), nil)
			Ω(err.Error()).ShouldNot(ContainSubstring("no such upload"))
		})
		It("should not include the Upload-Offset header of a real response", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("upload-offset", "256")
				w.WriteHeader(http.StatusConflict)
			}))
			defer srv.Close()
			resp, err := http.Get(srv.URL)
			Ω(err).ShouldNot(HaveOccurred())
			defer resp.Body.Close()

			Ω(newRUFHError(testErrOffsetsNotSynced, resp, nil).Error()).
				Should(Equal("client stream and server offsets are not synced: GET " + srv.URL + " (HTTP 409: Content-Length=0)"))
		})
		It("should be usable as a wrapped error message", func() {
			err := fmt.Errorf("upload failed: %w", newRUFHError(testErrCannotUpload, nil, errors.New("boom")))
			Ω(err.Error()).Should(Equal("upload failed: can not upload: boom"))
		})
	})

	Context("Unwrap", func() {
		It("should return the inner error", func() {
			inner := errors.New("boom")
			unwrapped := newRUFHError(testErrProtocol, nil, inner).Unwrap()
			Ω(unwrapped).Should(MatchError(inner))
			Ω(unwrapped.Error()).Should(Equal("boom"))
		})
		It("should return nil for a bare sentinel", func() {
			Ω(testErrProtocol.Unwrap()).Should(BeNil())
		})
		It("should return nil if neither response nor inner error is given", func() {
			Ω(newRUFHError(testErrProtocol, nil, nil).Unwrap()).Should(BeNil())
		})
		It("should return nil if the response body has been read successfully", func() {
			err := newRUFHError(testErrProtocol, testResponse(http.StatusOK, &closeTrackingBody{Reader: strings.NewReader("body")}, 4), nil)
			Ω(err.Unwrap()).Should(BeNil())
		})
	})

	Context("errors.Is", func() {
		It("should match the sentinel it was built from", func() {
			var err error = newRUFHError(testErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, testErrProtocol)).Should(BeTrue())
		})
		It("should not match a different sentinel", func() {
			var err error = newRUFHError(testErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, testErrCannotUpload)).Should(BeFalse())
		})
		It("should not match an error of another type with the same message", func() {
			var err error = newRUFHError(testErrProtocol, nil, errors.New("boom"))
			Ω(foreignErrProtocol.Error()).Should(Equal(testErrProtocol.Error()))
			Ω(errors.Is(err, foreignErrProtocol)).Should(BeFalse())
			Ω(errors.Is(testErrProtocol, foreignErrProtocol)).Should(BeFalse())
			Ω(errors.Is(foreignErrProtocol, testErrProtocol)).Should(BeFalse())
		})
		It("should match regardless of the inner error the sentinel carries", func() {
			a := newRUFHError(testErrTooLarge, nil, errors.New("first"))
			b := newRUFHError(testErrTooLarge, nil, errors.New("second"))
			Ω(errors.Is(a, b)).Should(BeTrue())
			Ω(errors.Is(b, a)).Should(BeTrue())
		})
		It("should match regardless of the response the sentinel carries", func() {
			var err error = newRUFHError(testErrOffsetsNotSynced, testResponse(http.StatusConflict, &closeTrackingBody{Reader: strings.NewReader("offset mismatch")}, 15), nil)
			Ω(errors.Is(err, testErrOffsetsNotSynced)).Should(BeTrue())
			Ω(errors.Is(err, testErrCannotUpload)).Should(BeFalse())
		})
		It("should match a bare sentinel against itself", func() {
			Ω(errors.Is(testErrProtocol, testErrProtocol)).Should(BeTrue())
		})
		It("should not match a bare sentinel against another one", func() {
			Ω(errors.Is(testErrProtocol, testErrChecksumMismatch)).Should(BeFalse())
		})
		It("should match the inner error", func() {
			inner := errors.New("boom")
			var err error = newRUFHError(testErrProtocol, nil, inner)
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match an error wrapped deeper in the inner chain", func() {
			inner := errors.New("boom")
			var err error = newRUFHError(testErrProtocol, nil, fmt.Errorf("context: %w", inner))
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match both sentinels when one RUFHError carries another", func() {
			var err error = newRUFHError(testErrProtocol, nil, testErrCannotUpload)
			Ω(errors.Is(err, testErrProtocol)).Should(BeTrue())
			Ω(errors.Is(err, testErrCannotUpload)).Should(BeTrue())
		})
		It("should match an error of another type carried as the inner error", func() {
			var err error = newRUFHError(testErrProtocol, nil, foreignErrCannotUpload)
			Ω(errors.Is(err, foreignErrCannotUpload)).Should(BeTrue())
			Ω(errors.Is(err, foreignErrProtocol)).Should(BeFalse())
		})
		It("should survive being wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newRUFHError(testErrUploadDoesNotExist, nil, errors.New("boom")))
			Ω(errors.Is(err, testErrUploadDoesNotExist)).Should(BeTrue())
			Ω(errors.Is(err, testErrProtocol)).Should(BeFalse())
		})
		It("should not match an unrelated error", func() {
			var err error = newRUFHError(testErrProtocol, nil, errors.New("boom"))
			Ω(errors.Is(err, io.EOF)).Should(BeFalse())
		})
	})

	Context("errors.As", func() {
		It("should extract a RUFHError with its message, inner error and response", func() {
			inner := errors.New("boom")
			resp := testResponse(http.StatusOK, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			var err error = newRUFHError(testErrProtocol, resp, inner)

			var target RUFHError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
			Ω(target.inner).Should(MatchError(inner))
			Ω(target.Response()).Should(HaveHTTPStatus(http.StatusOK))
		})
		It("should extract a RUFHError wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newRUFHError(testErrChecksumMismatch, nil, errors.New("boom")))

			var target RUFHError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("checksum mismatch"))
		})
		It("should extract the outermost RUFHError of a nested chain", func() {
			var err error = newRUFHError(testErrProtocol, nil, testErrCannotUpload)

			var target RUFHError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
		})
		It("should fail for an error that is not a RUFHError", func() {
			var target RUFHError
			Ω(errors.As(errors.New("boom"), &target)).Should(BeFalse())
		})
		It("should fail for an error of another type with the same message", func() {
			var target RUFHError
			Ω(errors.As(foreignErrProtocol, &target)).Should(BeFalse())
		})
	})

	Context("newRUFHError", func() {
		It("should not modify the sentinel it was built from", func() {
			_ = newRUFHError(testErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), errors.New("boom"))
			Ω(testErrUploadDoesNotExist.inner).Should(BeNil())
			Ω(testErrUploadDoesNotExist.resp).Should(BeNil())
		})
		It("should join the inner error to the inner error of an already contextualized error", func() {
			first := newRUFHError(testErrProtocol, nil, errors.New("first"))
			second := newRUFHError(first, nil, errors.New("second"))

			Ω(second.Error()).Should(Equal("protocol error: first\nsecond"))
			Ω(first.Error()).Should(Equal("protocol error: first"))
		})
	})

	Context("Response", func() {
		It("should return nil if no response is given", func() {
			Ω(newRUFHError(testErrProtocol, nil, errors.New("boom")).Response()).Should(BeNil())
		})
		It("should return nil for a bare sentinel", func() {
			Ω(testErrProtocol.Response()).Should(BeNil())
		})
		It("should return a copy of the given response with the body read into a buffer", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("no such upload")}
			resp := testResponse(http.StatusNotFound, body, 14)
			resp.Header = http.Header{"X-Foo": {"bar"}}

			err := newRUFHError(testErrUploadDoesNotExist, resp, nil)
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

			_ = newRUFHError(testErrUploadDoesNotExist, resp, nil)
			Ω(resp.Body).Should(BeIdenticalTo(body))
			Ω(body.closed).Should(BeFalse())
		})
		It("should keep an empty body", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("")}
			resp := testResponse(http.StatusNotFound, body, 0)

			err := newRUFHError(testErrUploadDoesNotExist, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte{}))
			Ω(body.closed).Should(BeFalse())
		})
		It("should keep a body up to the read limit untruncated", func() {
			data := strings.Repeat("x", bodyReadLimit)
			body := &closeTrackingBody{Reader: strings.NewReader(data)}
			resp := testResponse(http.StatusRequestEntityTooLarge, body, int64(len(data)))

			err := newRUFHError(testErrTooLarge, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte(data)))
		})
		It("should truncate a body longer than the read limit and leave the rest in the original body", func() {
			body := &closeTrackingBody{Reader: strings.NewReader(strings.Repeat("x", bodyReadLimit) + strings.Repeat("y", 100))}
			resp := testResponse(http.StatusRequestEntityTooLarge, body, -1)

			err := newRUFHError(testErrTooLarge, resp, nil)
			Ω(err.Unwrap()).Should(BeNil())
			Ω(err.Response()).Should(HaveHTTPBody([]byte(strings.Repeat("x", bodyReadLimit))))
			Ω(body.closed).Should(BeFalse())
			Ω(resp).Should(HaveHTTPBody([]byte(strings.Repeat("y", 100))))
		})
		It("should only keep the part of the body that has not been read yet", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("no such upload")}
			resp := testResponse(http.StatusNotFound, body, 14)
			Ω(io.ReadFull(resp.Body, make([]byte, len("no such ")))).Should(Equal(len("no such ")))

			err := newRUFHError(testErrUploadDoesNotExist, resp, nil)
			Ω(err.Response()).Should(HaveHTTPBody([]byte("upload")))
		})
		It("should report a body that cannot be read", func() {
			body := &closeTrackingBody{Reader: strings.NewReader("boom")}
			Ω(body.Close()).Should(Succeed())
			resp := testResponse(http.StatusInternalServerError, body, 4)

			err := newRUFHError(testErrUnexpectedResponse, resp, nil)
			Ω(err).Should(And(
				MatchError(testErrUnexpectedResponse),
				MatchError("unexpected HTTP response code: (HTTP 500: Content-Length=4): read response body: read on closed body"),
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

			err := newRUFHError(testErrProtocol, resp, inner)
			Ω(err).Should(And(
				MatchError(testErrProtocol),
				MatchError(inner),
				MatchError("protocol error: (HTTP 200: Content-Length=4): read response body: read on closed body\nbad header"),
			))
		})
	})

	Context("StatusCode", func() {
		It("should return the HTTP status code if response is not nil", func() {
			resp := testResponse(http.StatusInternalServerError, &closeTrackingBody{Reader: strings.NewReader("boom")}, 4)
			err := newRUFHError(testErrUnexpectedResponse, resp, nil)
			Ω(err.StatusCode()).Should(Equal(http.StatusInternalServerError))
		})
		It("should return 0 if response is nil", func() {
			err := newRUFHError(testErrUnexpectedResponse, nil, nil)
			Ω(err.StatusCode()).Should(Equal(0))
		})
		It("should return 0 for a bare sentinel", func() {
			Ω(testErrProtocol.StatusCode()).Should(Equal(0))
		})
	})
})
