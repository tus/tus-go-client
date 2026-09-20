package tusgo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/reply"
)

var _ = Describe("TusError", func() {
	Context("Error", func() {
		It("should join the sentinel message and the inner error", func() {
			err := newTusErrorWithErr(ErrProtocol, errors.New("boom"))
			Ω(err.Error()).Should(Equal("protocol error: boom"))
		})
		It("should keep the inner error message built by fmt.Errorf", func() {
			err := newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse %q: %w", "abc", errors.New("bad syntax")))
			Ω(err.Error()).Should(Equal(`protocol error: cannot parse "abc": bad syntax`))
		})
		It("should render a message when a bare sentinel is used", func() {
			Ω(ErrProtocol.Error()).Should(Equal("protocol error"))
		})
		It("should be usable as a wrapped error message", func() {
			err := fmt.Errorf("upload failed: %w", newTusErrorWithErr(ErrCannotUpload, errors.New("boom")))
			Ω(err.Error()).Should(Equal("upload failed: can not upload: boom"))
		})
	})

	Context("Unwrap", func() {
		It("should return the inner error", func() {
			inner := errors.New("boom")
			Ω(newTusErrorWithErr(ErrProtocol, inner).Unwrap()).Should(BeIdenticalTo(inner))
		})
		It("should return nil for a bare sentinel", func() {
			Ω(ErrProtocol.Unwrap()).Should(BeNil())
		})
	})

	Context("errors.Is", func() {
		It("should match the sentinel it was built from", func() {
			var err error = newTusErrorWithErr(ErrProtocol, errors.New("boom"))
			Ω(errors.Is(err, ErrProtocol)).Should(BeTrue())
		})
		It("should not match a different sentinel", func() {
			var err error = newTusErrorWithErr(ErrProtocol, errors.New("boom"))
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeFalse())
		})
		It("should match regardless of the inner error the sentinel carries", func() {
			a := newTusErrorWithErr(ErrUploadTooLarge, errors.New("first"))
			b := newTusErrorWithErr(ErrUploadTooLarge, errors.New("second"))
			Ω(errors.Is(a, b)).Should(BeTrue())
			Ω(errors.Is(b, a)).Should(BeTrue())
		})
		It("should match a bare sentinel against itself", func() {
			Ω(errors.Is(ErrProtocol, ErrProtocol)).Should(BeTrue())
		})
		It("should not match a bare sentinel against another one", func() {
			Ω(errors.Is(ErrProtocol, ErrChecksumMismatch)).Should(BeFalse())
		})
		It("should match the inner error", func() {
			inner := errors.New("boom")
			var err error = newTusErrorWithErr(ErrProtocol, inner)
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match an error wrapped deeper in the inner chain", func() {
			inner := errors.New("boom")
			var err error = newTusErrorWithErr(ErrProtocol, fmt.Errorf("context: %w", inner))
			Ω(errors.Is(err, inner)).Should(BeTrue())
		})
		It("should match both sentinels when one TusError carries another", func() {
			var err error = newTusErrorWithErr(ErrProtocol, ErrCannotUpload)
			Ω(errors.Is(err, ErrProtocol)).Should(BeTrue())
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeTrue())
		})
		It("should survive being wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newTusErrorWithErr(ErrUploadDoesNotExist, errors.New("boom")))
			Ω(errors.Is(err, ErrUploadDoesNotExist)).Should(BeTrue())
			Ω(errors.Is(err, ErrProtocol)).Should(BeFalse())
		})
		It("should not match an unrelated error", func() {
			var err error = newTusErrorWithErr(ErrProtocol, errors.New("boom"))
			Ω(errors.Is(err, io.EOF)).Should(BeFalse())
		})
	})

	Context("errors.As", func() {
		It("should extract a TusError with its message and inner error", func() {
			inner := errors.New("boom")
			var err error = newTusErrorWithErr(ErrProtocol, inner)

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
			Ω(target.inner).Should(BeIdenticalTo(inner))
		})
		It("should extract a TusError wrapped by fmt.Errorf", func() {
			err := fmt.Errorf("upload failed: %w", newTusErrorWithErr(ErrChecksumMismatch, errors.New("boom")))

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("checksum mismatch"))
		})
		It("should extract the outermost TusError of a nested chain", func() {
			var err error = newTusErrorWithErr(ErrProtocol, ErrCannotUpload)

			var target TusError
			Ω(errors.As(err, &target)).Should(BeTrue())
			Ω(target.msg).Should(Equal("protocol error"))
		})
		It("should fail for an error that is not a TusError", func() {
			var target TusError
			Ω(errors.As(errors.New("boom"), &target)).Should(BeFalse())
		})
	})

	Context("newTusErrorWithErr", func() {
		It("should not modify the sentinel it was built from", func() {
			_ = newTusErrorWithErr(ErrProtocol, errors.New("boom"))
			Ω(ErrProtocol.inner).Should(BeNil())
		})
		It("should replace the inner error of an already contextualized error", func() {
			first := newTusErrorWithErr(ErrProtocol, errors.New("first"))
			second := newTusErrorWithErr(first, errors.New("second"))

			Ω(second.Error()).Should(Equal("protocol error: second"))
			Ω(first.Error()).Should(Equal("protocol error: first"))
		})
	})

	Context("newTusErrorWithResponse", func() {
		var testClient *Client
		var srvMock *mocha.Mocha

		// tusResponse makes a request to the mock server the same way the Client does and returns the response
		tusResponse := func(location string) *http.Response {
			req, err := http.NewRequest(http.MethodGet, srvMock.URL()+location, nil)
			Ω(err).Should(Succeed())
			resp, err := testClient.tusRequest(context.Background(), req)
			Ω(err).Should(Succeed())
			return resp
		}

		BeforeEach(func() {
			srvMock = mocha.New(GinkgoT())
			srvMock.Start()
			testURL, _ := url.Parse(srvMock.URL())
			testClient = NewClient(http.DefaultClient, testURL)
			testClient.Capabilities = &ServerCapabilities{ProtocolVersions: []string{"1.0.0"}}
		})
		AfterEach(func() {
			if srvMock != nil {
				Ω(srvMock.Close()).Should(Succeed())
				srvMock.AssertCalled(GinkgoT())
			}
		})

		It("should include the status code and the response body", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusNotFound)).Body([]byte("no such upload"))),
			)

			err := newTusErrorWithResponse(ErrUploadDoesNotExist, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload does not exist: HTTP 404: body (14 bytes): no such upload"))
		})
		It("should report an empty body", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusNotFound))),
			)

			err := newTusErrorWithResponse(ErrUploadDoesNotExist, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload does not exist: HTTP 404: empty body"))
		})
		It("should keep a body untruncated", func() {
			body := strings.Repeat("x", 255)
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusRequestEntityTooLarge)).Body([]byte(body))),
			)

			err := newTusErrorWithResponse(ErrUploadTooLarge, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload is too large: HTTP 413: body (255 bytes): " + body))
		})
		It("should keep a body untruncated if empty content-length", func() {
			body := strings.Repeat("x", 255)
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusRequestEntityTooLarge)).Body([]byte(body)).Header("Content-Length", "")),
			)

			err := newTusErrorWithResponse(ErrUploadTooLarge, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload is too large: HTTP 413: body (? bytes): " + body))
		})
		It("should truncate a body longer than the read limit", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusRequestEntityTooLarge)).Body([]byte(strings.Repeat("x", 300)))),
			)

			err := newTusErrorWithResponse(ErrUploadTooLarge, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload is too large: HTTP 413: body (300 bytes): " + strings.Repeat("x", 256) + "... (truncated)"))
		})
		It("should truncate a body longer than the read limit if empty content-length", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusRequestEntityTooLarge)).Body([]byte(strings.Repeat("x", 300))).Header("Content-Length", "")),
			)

			err := newTusErrorWithResponse(ErrUploadTooLarge, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload is too large: HTTP 413: body (? bytes): " + strings.Repeat("x", 256) + "... (truncated)"))
		})
		It("should write a binary body to the message as raw bytes", func() {
			// NUL, DEL, an invalid UTF-8 sequence and other non-printable bytes.
			body := []byte{0x89, 0x50, 0x1A, 0x00, 0xFF, 0xFE, 0x01, 0x7F}
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusRequestEntityTooLarge)).Body(body)),
			)

			err := newTusErrorWithResponse(ErrUploadTooLarge, tusResponse("/foo"))
			Ω(err.Error()).Should(Equal("upload is too large: HTTP 413: body (8 bytes): [137 80 26 0 255 254 1 127]"))
			Ω(utf8.ValidString(err.Error())).Should(BeTrue())
		})
		It("should only include the part of the body that has not been read yet", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusNotFound)).Body([]byte("no such upload"))),
			)
			resp := tusResponse("/foo")
			Ω(io.ReadFull(resp.Body, make([]byte, len("no such ")))).Should(Equal(len("no such ")))

			err := newTusErrorWithResponse(ErrUploadDoesNotExist, resp)
			Ω(err.Error()).Should(Equal("upload does not exist: HTTP 404: body (14 bytes): upload"))
		})
		It("should report a body that cannot be read", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusInternalServerError)).Body([]byte("boom"))),
			)
			// Take the raw response, so that the body is a real network stream rather than the buffer tusRequest makes
			req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
			Ω(err).Should(Succeed())
			req.Header.Set("Tus-Resumable", testClient.ProtocolVersion)
			resp, err := http.DefaultClient.Do(req)
			Ω(err).Should(Succeed())
			Ω(resp.Body.Close()).Should(Succeed())

			tusErr := newTusErrorWithResponse(ErrUnexpectedResponse, resp)
			Ω(tusErr).Should(And(
				MatchError(ErrUnexpectedResponse),
				MatchError(ContainSubstring("unexpected HTTP response code: HTTP 500: read body: ")),
				MatchError(ContainSubstring("read on closed response body")),
			))
			Ω(tusErr.Error()).ShouldNot(ContainSubstring("boom"))
		})
		It("should report a nil response", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).Reply(tReply(reply.OK())))
			tusResponse("/foo") // Keep the mock expectation satisfied

			err := newTusErrorWithResponse(ErrProtocol, nil)
			Ω(err.Error()).Should(Equal("protocol error: response is nil"))
		})
		It("should stay matchable by its sentinel", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusConflict)).Body([]byte("offset mismatch"))),
			)

			var err error = newTusErrorWithResponse(ErrOffsetsNotSynced, tusResponse("/foo"))
			Ω(errors.Is(err, ErrOffsetsNotSynced)).Should(BeTrue())
			Ω(errors.Is(err, ErrCannotUpload)).Should(BeFalse())
		})
		It("should not modify the sentinel it was built from", func() {
			srvMock.AddMocks(tRequest(http.MethodGet, "/foo", nil).
				Reply(tReply(reply.Status(http.StatusNotFound)).Body([]byte("no such upload"))),
			)

			_ = newTusErrorWithResponse(ErrUploadDoesNotExist, tusResponse("/foo"))
			Ω(ErrUploadDoesNotExist.inner).Should(BeNil())
		})
	})
})
