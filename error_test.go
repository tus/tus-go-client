package tusgo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"time"

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
		DescribeTable("should include the status code, the content length and the Upload-Offset header of the response",
			func(te TusError, status int, contentLength int64, header http.Header, inner error, expect string) {
				resp := testResponse(status, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, contentLength)
				resp.Header = header

				Ω(newTusError(te, resp, inner).Error()).Should(Equal(expect))
			},
			Entry("no headers", ErrUploadDoesNotExist, http.StatusNotFound, int64(14), nil, nil,
				"upload does not exist: (HTTP 404: Content-Length=14)"),
			Entry("unknown content length", ErrTooLarge, http.StatusRequestEntityTooLarge, int64(-1), nil, nil,
				"data is too large: (HTTP 413: Content-Length=-1)"),
			Entry("Upload-Offset", ErrOffsetsNotSynced, http.StatusConflict, int64(14), http.Header{"Upload-Offset": {"512"}}, nil,
				"client stream and server offsets are not synced: (HTTP 409: Content-Length=14; Upload-Offset=512)"),
			Entry("zero Upload-Offset", ErrIncompleteWrite, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"0"}}, nil,
				"incomplete write: (HTTP 204: Content-Length=0; Upload-Offset=0)"),
			Entry("Upload-Offset is passed as is", ErrProtocol, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"abc"}}, nil,
				"protocol error: (HTTP 204: Content-Length=0; Upload-Offset=abc)"),
			Entry("empty Upload-Offset", ErrProtocol, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {""}}, nil,
				"protocol error: (HTTP 204: Content-Length=0)"),
			Entry("other headers only", ErrProtocol, http.StatusOK, int64(0), http.Header{"Upload-Length": {"1024"}, "X-Foo": {"bar"}}, nil,
				"protocol error: (HTTP 200: Content-Length=0)"),
			Entry("inner error", ErrProtocol, http.StatusOK, int64(0), nil, errors.New("boom"),
				"protocol error: (HTTP 200: Content-Length=0): boom"),
			Entry("Upload-Offset and inner error", ErrIncompleteWrite, http.StatusNoContent, int64(0), http.Header{"Upload-Offset": {"100"}}, errors.New("boom"),
				"incomplete write: (HTTP 204: Content-Length=0; Upload-Offset=100): boom"),
		)
		It("should include the method and the URL of the request", func() {
			resp := testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			resp.Request = httptest.NewRequest(http.MethodPatch, "http://example.com/files/abc?x=1", nil)

			Ω(newTusError(ErrUploadDoesNotExist, resp, errors.New("boom")).Error()).
				Should(Equal("upload does not exist: PATCH http://example.com/files/abc?x=1 (HTTP 404: Content-Length=0): boom"))
		})
		It("should omit the method and the URL if the request has no URL", func() {
			resp := testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("")}, 0)
			resp.Request = &http.Request{Method: http.MethodPatch}

			Ω(newTusError(ErrUploadDoesNotExist, resp, nil).Error()).
				Should(Equal("upload does not exist: (HTTP 404: Content-Length=0)"))
		})
		It("should not include the response body", func() {
			err := newTusError(ErrUploadDoesNotExist, testResponse(http.StatusNotFound, &closeTrackingBody{Reader: strings.NewReader("no such upload")}, 14), nil)
			Ω(err.Error()).ShouldNot(ContainSubstring("no such upload"))
		})
		It("should include the Upload-Offset header of a real response", func() {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("upload-offset", "256")
				w.WriteHeader(http.StatusConflict)
			}))
			defer srv.Close()
			resp, err := http.Get(srv.URL)
			Ω(err).ShouldNot(HaveOccurred())
			defer resp.Body.Close()

			Ω(newTusError(ErrOffsetsNotSynced, resp, nil).Error()).
				Should(Equal("client stream and server offsets are not synced: GET " + srv.URL + " (HTTP 409: Content-Length=0; Upload-Offset=256)"))
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

			err := newTusError(ErrProtocol, resp, inner)
			Ω(err).Should(And(
				MatchError(ErrProtocol),
				MatchError(inner),
				MatchError("protocol error: (HTTP 200: Content-Length=4): read response body: read on closed body\nbad header"),
			))
		})
	})

	Context("StatusCode", func() {
		It("should return the HTTP status code if response is not nil", func() {
			resp := testResponse(http.StatusInternalServerError, &closeTrackingBody{Reader: strings.NewReader("boom")}, 4)
			err := newTusError(ErrUnexpectedResponse, resp, nil)
			Ω(err.StatusCode()).Should(Equal(http.StatusInternalServerError))
		})
		It("should return 0 if response is nil", func() {
			err := newTusError(ErrUnexpectedResponse, nil, nil)
			Ω(err.StatusCode()).Should(Equal(0))
		})
		It("should return 0 for a bare sentinel", func() {
			Ω(ErrProtocol.StatusCode()).Should(Equal(0))
		})
	})
})

var _ = Describe("IsErrorRetryable", func() {
	// withStatus returns the error built from the sentinel with a response of the given status
	withStatus := func(te TusError, status int) TusError {
		return newTusError(te, testResponse(status, &closeTrackingBody{Reader: strings.NewReader("")}, 0), nil)
	}

	DescribeTable("should classify errors",
		func(err error, expect bool) {
			Ω(IsErrorRetryable(err)).Should(Equal(expect))
		},
		Entry("nil", nil, false),
		Entry("unrelated error", errors.New("boom"), false),

		Entry("ErrIncompleteWrite", newTusError(ErrIncompleteWrite, nil, errors.New("boom")), true),
		Entry("ErrChecksumMismatch", withStatus(ErrChecksumMismatch, 460), true),
		Entry("ErrOffsetsNotSynced", withStatus(ErrOffsetsNotSynced, http.StatusConflict), true),
		Entry("bare sentinel", ErrOffsetsNotSynced, true),
		Entry("TusError wrapped by fmt.Errorf", fmt.Errorf("write: %w", withStatus(ErrOffsetsNotSynced, http.StatusConflict)), true),

		Entry("ErrTooLarge", withStatus(ErrTooLarge, http.StatusRequestEntityTooLarge), false),
		Entry("ErrUploadDoesNotExist", withStatus(ErrUploadDoesNotExist, http.StatusNotFound), false),
		Entry("ErrCannotUpload", withStatus(ErrCannotUpload, http.StatusForbidden), false),
		Entry("ErrUnsupportedFeature", newTusError(ErrUnsupportedFeature, nil, errors.New("checksum")), false),
		Entry("ErrProtocol", withStatus(ErrProtocol, http.StatusPreconditionFailed), false),
		Entry("non-retryable TusError carrying a retryable one", newTusError(ErrProtocol, nil, ErrIncompleteWrite), false),

		Entry("ErrUnexpectedResponse 408", withStatus(ErrUnexpectedResponse, http.StatusRequestTimeout), true),
		Entry("ErrUnexpectedResponse 423", withStatus(ErrUnexpectedResponse, http.StatusLocked), true),
		Entry("ErrUnexpectedResponse 429", withStatus(ErrUnexpectedResponse, http.StatusTooManyRequests), true),
		Entry("ErrUnexpectedResponse 500", withStatus(ErrUnexpectedResponse, http.StatusInternalServerError), true),
		Entry("ErrUnexpectedResponse 502", withStatus(ErrUnexpectedResponse, http.StatusBadGateway), true),
		Entry("ErrUnexpectedResponse 503", withStatus(ErrUnexpectedResponse, http.StatusServiceUnavailable), true),
		Entry("ErrUnexpectedResponse 504", withStatus(ErrUnexpectedResponse, http.StatusGatewayTimeout), true),
		Entry("ErrUnexpectedResponse 200", withStatus(ErrUnexpectedResponse, http.StatusOK), false),
		Entry("ErrUnexpectedResponse 400", withStatus(ErrUnexpectedResponse, http.StatusBadRequest), false),
		Entry("ErrUnexpectedResponse 401", withStatus(ErrUnexpectedResponse, http.StatusUnauthorized), false),
		Entry("ErrUnexpectedResponse 501", withStatus(ErrUnexpectedResponse, http.StatusNotImplemented), false),
		Entry("ErrUnexpectedResponse without response", ErrUnexpectedResponse, false),

		Entry("context.Canceled", fmt.Errorf("do request: %w", context.Canceled), false),
		Entry("context.DeadlineExceeded", fmt.Errorf("do request: %w", context.DeadlineExceeded), false),
		Entry("context.DeadlineExceeded in url.Error",
			&url.Error{Op: "Patch", URL: "http://example.com", Err: context.DeadlineExceeded}, false),
		Entry("connection refused",
			&url.Error{Op: "Patch", URL: "http://example.com", Err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}}, true),
		Entry("net.OpError without url.Error", fmt.Errorf("do request: %w", &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}), true),
		Entry("connection closed by the server", &url.Error{Op: "Patch", URL: "http://example.com", Err: io.EOF}, true),
		Entry("connection closed unexpectedly", &url.Error{Op: "Patch", URL: "http://example.com", Err: io.ErrUnexpectedEOF}, true),
		Entry("io.ErrUnexpectedEOF outside url.Error", fmt.Errorf("read: %w", io.ErrUnexpectedEOF), false),
		Entry("temporary DNS failure",
			&url.Error{Op: "Patch", URL: "http://example.com", Err: &net.DNSError{Err: "server misbehaving", IsTemporary: true}}, true),
		Entry("host not found",
			&url.Error{Op: "Patch", URL: "http://example.com", Err: &net.DNSError{Err: "no such host", IsNotFound: true}}, false),
		Entry("url.Error with non-network error",
			&url.Error{Op: "Patch", URL: "foo://example.com", Err: errors.New(`unsupported protocol scheme "foo"`)}, false),
	)

	Context("errors returned by http.Client", func() {
		var srv *httptest.Server
		var handler http.HandlerFunc

		BeforeEach(func() {
			handler = nil
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler(w, r) }))
		})
		AfterEach(func() {
			srv.Close()
		})
		// withContext makes c send requests with ctx
		withContext := func(c *Client, ctx context.Context) *Client {
			c.DoRequest = func(req *http.Request) (*http.Response, error) {
				return http.DefaultClient.Do(req.WithContext(ctx))
			}
			return c
		}
		// getUpload requests the upload from srv through c and returns the error
		getUpload := func(c *Client) error {
			GinkgoHelper()
			_, err := c.GetUpload(srv.URL + "/foo/bar")
			Ω(err).Should(HaveOccurred())
			return err
		}

		It("should be retryable on http.Client.Timeout", func() {
			handler = func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) }
			u, _ := url.Parse(srv.URL)
			c := NewClient(u)
			c.Client = &http.Client{Timeout: 20 * time.Millisecond}

			Ω(IsErrorRetryable(getUpload(c))).Should(BeTrue())
		})
		It("should not be retryable when the context deadline has expired", func() {
			handler = func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) }
			u, _ := url.Parse(srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			c := withContext(NewClient(u), ctx)

			Ω(IsErrorRetryable(getUpload(c))).Should(BeFalse())
		})
		It("should not be retryable when the context is canceled", func() {
			u, _ := url.Parse(srv.URL)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			c := withContext(NewClient(u), ctx)

			Ω(IsErrorRetryable(getUpload(c))).Should(BeFalse())
		})
		It("should be retryable when the server closes the connection", func() {
			handler = func(w http.ResponseWriter, r *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(conn.Close()).Should(Succeed())
			}
			u, _ := url.Parse(srv.URL)

			Ω(IsErrorRetryable(getUpload(NewClient(u)))).Should(BeTrue())
		})
		It("should be retryable when the connection is refused", func() {
			u, _ := url.Parse(srv.URL)
			srv.Close()

			Ω(IsErrorRetryable(getUpload(NewClient(u)))).Should(BeTrue())
		})
		It("should not be retryable for an unsupported URL scheme", func() {
			u, _ := url.Parse("foo://example.com")
			_, err := NewClient(u).GetUpload("foo://example.com/foo/bar")
			Ω(err).Should(HaveOccurred())

			Ω(IsErrorRetryable(err)).Should(BeFalse())
		})
		It("should be retryable on 503 response", func() {
			handler = func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }
			u, _ := url.Parse(srv.URL)

			Ω(IsErrorRetryable(getUpload(NewClient(u)))).Should(BeTrue())
		})
		It("should not be retryable on 404 response", func() {
			handler = func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }
			u, _ := url.Parse(srv.URL)

			Ω(IsErrorRetryable(getUpload(NewClient(u)))).Should(BeFalse())
		})
	})
})
