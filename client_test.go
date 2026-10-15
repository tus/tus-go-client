package tusgo

import (
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

func tRequest(method, location string, emptyHeaders []string) *mocha.MockBuilder {
	b := mocha.Request().
		URL(expect.URLPath(location)).Method(method).
		Header("Tus-Resumable", expect.ToEqual("1.0.0"))
	for _, h := range emptyHeaders {
		b = b.Header(h, expect.ToBeEmpty())
	}
	return b
}

func tReply(startReply *reply.StdReply) *reply.StdReply {
	return startReply.Header("Tus-Resumable", "1.0.0")
}

// tErrResponse returns the response kept by the TusError found in err chain, or nil
func tErrResponse(err error) *http.Response {
	var te TusError
	if errors.As(err, &te) {
		return te.Response()
	}
	return nil
}

// countingTransport counts the requests made through it
type countingTransport struct {
	requests int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests++
	return http.DefaultTransport.RoundTrip(req)
}

type doRequestCall struct {
	method string
	url    string
}

// doRequestSpy provides a DoRequestFunc that records its calls and sets the X-Custom header in every request
type doRequestSpy struct {
	calls []doRequestCall
}

func (s *doRequestSpy) DoRequest(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, doRequestCall{method: req.Method, url: req.URL.String()})
	req.Header.Set("X-Custom", "custom")
	return http.DefaultClient.Do(req)
}

// fakeResponder provides a DoRequestFunc that replies with the given status, headers and body without making a real
// request, and records the response bodies to check whether they have been closed
type fakeResponder struct {
	status int
	header http.Header
	body   string
	bodies []*closeTrackingBody
}

func (f *fakeResponder) DoRequest(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}
	b := &closeTrackingBody{Reader: strings.NewReader(f.body)}
	f.bodies = append(f.bodies, b)
	return &http.Response{StatusCode: f.status, Header: f.header.Clone(), Body: b, ContentLength: int64(len(f.body)), Request: req}, nil
}

// tExpectBodyKeptInError makes the client reply to the request made in call with a fake response with body, and checks
// that the returned error is expectErr, the response kept by the error contains the body, and the original body is closed
func tExpectBodyKeptInError(c *Client, status int, header http.Header, expectErr error, call func() error) {
	GinkgoHelper()
	responder := &fakeResponder{status: status, header: header, body: "server says"}
	c.DoRequest = responder.DoRequest

	err := call()
	Ω(err).Should(MatchError(expectErr))
	Ω(err).ShouldNot(MatchError(ContainSubstring("read response body")))
	Ω(tErrResponse(err)).Should(And(HaveHTTPStatus(status), HaveHTTPBody([]byte("server says"))))
	Ω(responder.bodies).Should(HaveLen(1))
	Ω(responder.bodies[0].closed).Should(BeTrue())
}

// tExpectBodyClosed makes the client reply to the request made in call with a fake successful response with body, and
// checks that call succeeds and the body is closed
func tExpectBodyClosed(c *Client, status int, header http.Header, call func() error) {
	GinkgoHelper()
	responder := &fakeResponder{status: status, header: header, body: "server says"}
	c.DoRequest = responder.DoRequest

	Ω(call()).Should(Succeed())
	Ω(responder.bodies).Should(HaveLen(1))
	Ω(responder.bodies[0].closed).Should(BeTrue())
}

var _ = Describe("Client", func() {
	var testClient *Client
	var testURL *url.URL
	var srvMock *mocha.Mocha
	var tusHeaders []string
	var spy *doRequestSpy
	var extraHeaders http.Header

	BeforeEach(func() {
		srvMock = mocha.New(GinkgoT())
		srvMock.Start()
		testURL, _ = url.Parse(srvMock.URL())
		testClient = NewClient(testURL)
		testClient.Capabilities = &ServerCapabilities{
			ProtocolVersions: []string{"1.0.0"},
		}
		tusHeaders = []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
		spy = &doRequestSpy{}
		extraHeaders = http.Header{"X-Extra": {"extra"}}
	})
	AfterEach(func() {
		if srvMock != nil {
			Ω(srvMock.Close()).Should(Succeed())
			srvMock.AssertCalled(GinkgoT())
		}
	})
	Context("NewClient", func() {
		It("should correct set initial values", func() {
			Ω(testClient.ProtocolVersion).Should(Equal("1.0.0"))
			Ω(testClient.DoRequest).Should(BeNil())
			Ω(testClient.Client).Should(BeNil())
			Ω(testClient.ExtraHeaders).Should(Equal(http.Header{}))
			Ω(testClient.endpoint).Should(Equal(testURL))
		})
		It("should panic if endpoint is nil", func() {
			Ω(func() { NewClient(nil) }).Should(PanicWith("endpoint is nil"))
		})
	})
	Context("tusRequest", func() {
		Context("happy path", func() {
			It("should make a request, return response", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK()).Body([]byte("test"))))
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())
				resp, err := testClient.tusRequest(req)
				Ω(err).Should(Succeed())
				Ω(resp).ShouldNot(BeNil())
				Ω(resp).Should(HaveHTTPBody([]byte("test")))
			})
			When("OPTIONS request", func() {
				It("should not set Tus-Resumable header", func() {
					srvMock.AddMocks(mocha.Request().
						URL(expect.URLPath("/foo")).Method(http.MethodOptions).
						Header("Tus-Resumable", expect.ToBeEmpty()). // OPTIONS request should not contain this header
						Reply(tReply(reply.OK())),
					)
					req, err := http.NewRequest(http.MethodOptions, srvMock.URL()+"/foo", nil)
					Ω(err).Should(Succeed())

					Ω(testClient.tusRequest(req)).ShouldNot(BeNil())
				})
			})
			It("should keep Tus-Resumable header if it is already set", func() {
				srvMock.AddMocks(mocha.Request().
					URL(expect.URLPath("/foo")).Method(http.MethodGet).
					Header("Tus-Resumable", expect.ToEqual("0.2.2")).
					Reply(tReply(reply.OK())),
				)
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())
				req.Header.Set("Tus-Resumable", "0.2.2")

				Ω(testClient.tusRequest(req)).ShouldNot(BeNil())
			})
			It("should use http.DefaultClient if neither DoRequest nor Client is set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK())))
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
			})
			It("should use Client if DoRequest is not set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK())))
				transport := &countingTransport{}
				testClient.Client = &http.Client{Transport: transport}
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
				Ω(transport.requests).Should(Equal(1))
			})
			It("should use DoRequest and ignore Client if both are set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Reply(tReply(reply.OK())),
				)
				transport := &countingTransport{}
				testClient.Client = &http.Client{Transport: transport}
				testClient.DoRequest = spy.DoRequest
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodGet, srvMock.URL() + "/foo"}}))
				Ω(transport.requests).Should(Equal(0))
			})
		})
		Context("error path", func() {
			It("should process http 412 unknown versions", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).
					Reply(reply.Status(http.StatusPreconditionFailed).
						Header("Tus-Version", "1.0.1,0.9.0").
						Body([]byte("test"))),
				)
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				_, err = testClient.tusRequest(req)
				Ω(err).Should(And(
					MatchError(ErrProtocol),
					MatchError(ContainSubstring("protocol error (HTTP 412;")),
					MatchError(ContainSubstring("server does not support protocol version \"1.0.0\", supported versions: 1.0.1,0.9.0")),
				))
				Ω(tErrResponse(err)).Should(And(HaveHTTPStatus(http.StatusPreconditionFailed), HaveHTTPBody([]byte("test"))))
			})
			It("should return error returned by DoRequest", func() {
				doErr := errors.New("boom")
				testClient.DoRequest = func(*http.Request) (*http.Response, error) { return nil, doErr }
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				resp, err := testClient.tusRequest(req)
				Ω(err).Should(MatchError(doErr))
				Ω(resp).Should(BeNil())
			})
		})
	})
	Context("GetUpload", func() {
		Context("happy path", func() {
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusOK, http.Header{"Upload-Offset": {"0"}}, func() error {
					_, err := testClient.GetUpload("/foo/bar")
					return err
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.OK()).
						Header("Upload-Offset", "64")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.GetUpload("/foo/bar")
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodHead, srvMock.URL() + "/foo/bar"}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar", RemoteOffset: 64}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.OK()).
						Header("Upload-Offset", "64")),
				)
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.GetUpload("/foo/bar")
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar", RemoteOffset: 64}))
			})
			When("ordinary upload", func() {
				It("should get upload info", func() {
					srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
						Reply(tReply(reply.OK()).
							Header("Upload-Offset", "64")),
					)

					f, err := testClient.GetUpload("/foo/bar")
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						Location:     "/foo/bar",
						RemoteOffset: 64,
					}))
				})
			})
			When("upload with metadata", func() {
				DescribeTable("should parse metadata",
					func(metadata string, expectMetadata map[string]string) {
						srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
							Reply(tReply(reply.OK()).
								Header("Upload-Offset", "64").
								Header("Upload-Metadata", metadata)),
						)

						f, err := testClient.GetUpload("/foo/bar")
						Ω(err).Should(Succeed())
						Ω(f).Should(Equal(Upload{
							Location:     "/foo/bar",
							RemoteOffset: 64,
							Metadata:     expectMetadata,
						}))
					},
					Entry("items with values", "key1 dmFsdWUx,key2 Jl4lJCIJ",
						map[string]string{"key1": "value1", "key2": "&^%$\"\t"}),
					Entry("items with empty value, with or without the separating space", "key1 dmFsdWUx,key2 ,key3",
						map[string]string{"key1": "value1", "key2": "", "key3": ""}),
				)
			})
			When("partial upload", func() {
				It("should get upload info", func() {
					srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
						Reply(tReply(reply.OK()).
							Header("Upload-Concat", "partial").
							Header("Upload-Offset", "64")),
					)

					f, err := testClient.GetUpload("/foo/bar")
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						Location:     "/foo/bar",
						RemoteOffset: 64,
						Partial:      true,
					}))
				})
			})
			When("final upload", func() {
				It("should get upload info", func() {
					srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
						Reply(tReply(reply.OK()).
							Header("Upload-Concat", "final").
							Header("Upload-Offset", "64").
							Header("Upload-Length", "1024")),
					)

					f, err := testClient.GetUpload("/foo/bar")
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						Location:     "/foo/bar",
						RemoteOffset: 64,
						Partial:      false,
						RemoteSize:   1024,
					}))
				})
				When("concatenated upload is still in progress", func() {
					It("should get upload info with unknown offset", func() {
						srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
							Reply(tReply(reply.OK()).
								Header("Upload-Concat", "final").
								Header("Upload-Length", "1024")),
						)

						f, err := testClient.GetUpload("/foo/bar")
						Ω(err).Should(Succeed())
						Ω(f).Should(Equal(Upload{
							Location:     "/foo/bar",
							Partial:      false,
							RemoteSize:   1024,
							RemoteOffset: OffsetUnknown,
						}))
					})
				})
			})
		})
		Context("error path", func() {
			DescribeTable("should keep the response body in the error and close the original body",
				func(status int, header http.Header, expectErr error) {
					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						_, err := testClient.GetUpload("/foo/bar")
						return err
					})
				},
				Entry("http error", http.StatusNotFound, nil, ErrUploadDoesNotExist),
				Entry("malformed header", http.StatusOK, http.Header{"Upload-Offset": {"asdf"}}, ErrProtocol),
			)
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectMsg string) {
						srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).Reply(reply.Status(status)))

						_, err := testClient.GetUpload("/foo/bar")
						Ω(err).Should(And(
							MatchError(expectErr),
							MatchError(ContainSubstring(expectMsg)),
						))
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(status))
					},
					Entry("404", http.StatusNotFound, ErrUploadDoesNotExist, "upload does not exist (HTTP 404;"),
					Entry("410", http.StatusGone, ErrUploadDoesNotExist, "upload does not exist (HTTP 410;"),
					Entry("403", http.StatusForbidden, ErrUploadDoesNotExist, "upload does not exist (HTTP 403;"),
					Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
					Entry("201", http.StatusCreated, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 201;"),
				)
			})
			When("corrupted numeric header value", func() {
				DescribeTable("should return protocol error",
					func(testHeader string, headers map[string]string) {
						r := tReply(reply.OK())
						for h, v := range headers {
							r = r.Header(h, v)
						}
						srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).Reply(r))

						_, err := testClient.GetUpload("/foo/bar")
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusOK))
						Ω(err).Should(And(
							MatchError(ErrProtocol),
							MatchError(ContainSubstring(": parse "+testHeader+" header \"asdf\": strconv.ParseInt: parsing \"asdf\": invalid syntax")),
						))
					},
					Entry("Upload-Offset", "Upload-Offset", map[string]string{"Upload-Offset": "asdf"}),
					Entry("Upload-Length", "Upload-Length", map[string]string{"Upload-Length": "asdf", "Upload-Offset": "123"}),
				)
			})
			When("corrupted metadata header value", func() {
				DescribeTable("should return protocol error",
					func(metadata, expectMsg string) {
						srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
							Reply(tReply(reply.OK()).
								Header("Upload-Offset", "64").
								Header("Upload-Length", "1024").
								Header("Upload-Metadata", metadata)),
						)

						_, err := testClient.GetUpload("/foo/bar")
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusOK))
						Ω(err).Should(And(
							MatchError(ErrProtocol),
							MatchError(ContainSubstring(": parse Upload-Metadata header \""+metadata+"\": "+expectMsg)),
						))
					},
					Entry("item without key", "key1 dmFsdWUx, dmFsdWUy", "metadata item \" dmFsdWUy\" has bad format"),
					Entry("invalid base64 value", "key1 dmFsdWUx,key2 !!!!", "illegal base64 data at input byte 0"),
				)
			})
		})
	})
	Context("CreateUpload", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
			})
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusCreated, http.Header{"Location": {"/foo/bar"}}, func() error {
					_, err := testClient.CreateUpload(1024, false, nil)
					return err
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Content-Length", expect.ToEqual("0")).
					Header("Upload-Length", expect.ToEqual("1024")).
					Reply(tReply(reply.Created()).
						Header("Location", "/foo/bar")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodPost, srvMock.URL()}}))
				Ω(f).Should(Equal(Upload{RemoteSize: 1024, Location: "/foo/bar"}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Content-Length", expect.ToEqual("0")).
					Header("Upload-Length", expect.ToEqual("1024")).
					Reply(tReply(reply.Created()).
						Header("Location", "/foo/bar")),
				)
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(f).Should(Equal(Upload{RemoteSize: 1024, Location: "/foo/bar"}))
			})
			When("upload with size, without metadata", func() {
				It("should create upload", func() {
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("0")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar")),
					)

					f, err := testClient.CreateUpload(1024, false, nil)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						RemoteSize: 1024,
						Location:   "/foo/bar",
					}))
				})
			})
			When("upload with size, with metadata", func() {
				It("should encode metadata and create upload", func() {
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Checksum", "Upload-Offset"}
					md := map[string]string{
						"key1": "value1",
						"key2": "&^%$\"\t",
					}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("0")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Upload-Metadata", expect.Func(func(v any, _ expect.Args) (bool, error) {
							m, e := DecodeMetadata(v.(string))
							return reflect.DeepEqual(m, md), e
						})).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar")),
					)

					f, err := testClient.CreateUpload(1024, false, md)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						RemoteSize: 1024,
						Location:   "/foo/bar",
						Metadata:   md,
					}))
				})
			})
			When("partial upload with size, with metadata", func() {
				It("should encode metadata and create upload", func() {
					eh := []string{"Upload-Defer-Length", "Upload-Checksum", "Upload-Offset"}
					md := map[string]string{
						"key1": "value1",
						"key2": "&^%$\"\t",
					}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Upload-Concat", expect.ToEqual("partial")).
						Header("Content-Length", expect.ToEqual("0")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Upload-Metadata", expect.Func(func(v any, _ expect.Args) (bool, error) {
							m, e := DecodeMetadata(v.(string))
							return reflect.DeepEqual(m, md), e
						})).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar")),
					)

					f, err := testClient.CreateUpload(1024, true, md)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						RemoteSize: 1024,
						Location:   "/foo/bar",
						Metadata:   md,
						Partial:    true,
					}))
				})
			})
			When("partial upload with defer size, with metadata", func() {
				It("should encode metadata and create upload", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation-defer-length")
					eh := []string{"Upload-Length", "Upload-Checksum", "Upload-Offset"}
					md := map[string]string{
						"key1": "value1",
						"key2": "&^%$\"\t",
					}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Upload-Concat", expect.ToEqual("partial")).
						Header("Content-Length", expect.ToEqual("0")).
						Header("Upload-Defer-Length", expect.ToEqual("1")).
						Header("Upload-Metadata", expect.Func(func(v any, _ expect.Args) (bool, error) {
							m, e := DecodeMetadata(v.(string))
							return reflect.DeepEqual(m, md), e
						})).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar")),
					)

					f, err := testClient.CreateUpload(SizeUnknown, true, md)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						RemoteSize: SizeUnknown,
						Location:   "/foo/bar",
						Metadata:   md,
						Partial:    true,
					}))
				})
			})
		})
		Context("error path", func() {
			DescribeTable("should keep the response body in the error and close the original body",
				func(status int, expectErr error) {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
					tExpectBodyKeptInError(testClient, status, nil, expectErr, func() error {
						_, err := testClient.CreateUpload(1024, false, nil)
						return err
					})
				},
				Entry("http error", http.StatusRequestEntityTooLarge, ErrUploadTooLarge),
				Entry("unsupported protocol version", http.StatusPreconditionFailed, ErrProtocol),
			)
			Specify("no creation extension", func() {
				_, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation")),
				))
			})
			Specify("no creation-defer-length extension and trying to create defer size upload", func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
				_, err := testClient.CreateUpload(SizeUnknown, false, nil)
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation-defer-length")),
				))
			})
			When("upload size is negative", func() {
				It("should panic", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
					Ω(func() { _, _ = testClient.CreateUpload(-2, false, nil) }).Should(Panic())
				})
			})
			Specify("metadata key contains a space", func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
				md := map[string]string{
					"key 1": "value1",
					"key2":  "&^%$\"\t",
				}
				_, err := testClient.CreateUpload(1024, false, md)
				Ω(err).Should(MatchError(ContainSubstring("key \"key 1\" contains spaces")))
			})
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectErrMsg string) {
						eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
						testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
						srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
							Header("Content-Length", expect.ToEqual("0")).
							Header("Upload-Length", expect.ToEqual("1024")).
							Reply(reply.Status(status)))
						_, err := testClient.CreateUpload(1024, false, nil)
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(status))
						Ω(err).Should(And(
							MatchError(expectErr),
							MatchError(ContainSubstring(expectErrMsg)),
						))
					},
					Entry("413", http.StatusRequestEntityTooLarge, ErrUploadTooLarge, "upload is too large (HTTP 413;"),
					Entry("404", http.StatusNotFound, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 404;"),
					Entry("410", http.StatusGone, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 410;"),
					Entry("403", http.StatusForbidden, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 403;"),
					Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
					Entry("200", http.StatusOK, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 200;"),
				)
			})
		})
	})
	Context("CreateUploadWithData", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation", "creation-with-upload")
			})
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusCreated, http.Header{"Location": {"/foo/bar"}, "Upload-Offset": {"10"}}, func() error {
					_, _, err := testClient.CreateUploadWithData(make([]byte, 10), 1024, false, nil)
					return err
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 512))
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Content-Length", expect.ToEqual("512")).
					Header("Upload-Length", expect.ToEqual("1024")).
					Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
					Body(expect.ToEqual(d)).
					Reply(tReply(reply.Created()).
						Header("Location", "/foo/bar").
						Header("Upload-Offset", "512")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders

				bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
				Ω(bytes).Should(BeEquivalentTo(512))
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodPost, srvMock.URL()}}))
				Ω(u).Should(Equal(Upload{RemoteSize: 1024, Location: "/foo/bar", RemoteOffset: 512}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 512))
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Content-Length", expect.ToEqual("512")).
					Header("Upload-Length", expect.ToEqual("1024")).
					Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
					Body(expect.ToEqual(d)).
					Reply(tReply(reply.Created()).
						Header("Location", "/foo/bar").
						Header("Upload-Offset", "512")),
				)
				testClient.ExtraHeaders = extraHeaders

				bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
				Ω(bytes).Should(BeEquivalentTo(512))
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(u).Should(Equal(Upload{RemoteSize: 1024, Location: "/foo/bar", RemoteOffset: 512}))
			})
			When("upload without metadata", func() {
				DescribeTable("should upload data in one request",
					func(dataLen int) {
						eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
						d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(dataLen)))
						srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
							Header("Content-Length", expect.ToEqual(strconv.Itoa(dataLen))).
							Header("Upload-Length", expect.ToEqual("1024")).
							Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
							Body(expect.ToEqual(d)).
							Reply(tReply(reply.Created()).
								Header("Location", "/foo/bar").
								Header("Upload-Offset", strconv.Itoa(dataLen))),
						)

						bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
						Ω(bytes).Should(BeEquivalentTo(dataLen))
						Ω(err).Should(Succeed())
						Ω(u).Should(Equal(Upload{
							RemoteSize:   1024,
							Location:     "/foo/bar",
							RemoteOffset: int64(dataLen),
						}))
					},
					Entry("part of upload length", 512),
					Entry("full upload length", 1024),
				)
			})
			When("upload all data with metadata", func() {
				It("should upload data in one request and add metadata", func() {
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Checksum", "Upload-Offset"}
					d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 512))
					md := map[string]string{"key1": "value1", "key2": "&^%$\"\t"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("512")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
						Header("Upload-Metadata", expect.Func(func(v any, _ expect.Args) (bool, error) {
							m, e := DecodeMetadata(v.(string))
							return reflect.DeepEqual(m, md), e
						})).
						Body(expect.ToEqual(d)).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar").
							Header("Upload-Offset", "512")),
					)

					bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, md)
					Ω(bytes).Should(BeEquivalentTo(512))
					Ω(err).Should(Succeed())
					Ω(u).Should(Equal(Upload{
						RemoteSize:   1024,
						Location:     "/foo/bar",
						RemoteOffset: 512,
						Metadata:     md,
					}))
				})
			})
			When("create partial upload and upload all data without metadata", func() {
				It("should upload data in one request", func() {
					eh := []string{"Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("1024")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Upload-Concat", expect.ToEqual("partial")).
						Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
						Body(expect.ToEqual(d)).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar").
							Header("Upload-Offset", "1024")),
					)

					bytes, u, err := testClient.CreateUploadWithData(d, 1024, true, nil)
					Ω(bytes).Should(BeEquivalentTo(1024))
					Ω(err).Should(Succeed())
					Ω(u).Should(Equal(Upload{
						RemoteSize:   1024,
						Location:     "/foo/bar",
						RemoteOffset: 1024,
						Partial:      true,
					}))
				})
			})
		})
		Context("error path", func() {
			DescribeTable("should keep the response body in the error and close the original body",
				func(status int, header http.Header, expectErr error) {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation", "creation-with-upload")
					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						_, _, err := testClient.CreateUploadWithData(make([]byte, 10), 1024, false, nil)
						return err
					})
				},
				Entry("http error", http.StatusConflict, nil, ErrOffsetsNotSynced),
				Entry("zero progress", http.StatusCreated, http.Header{"Upload-Offset": {"0"}}, ErrZeroProgress),
				Entry("offset advanced too far", http.StatusCreated, http.Header{"Upload-Offset": {"50"}}, ErrProtocol),
			)
			Specify("no 'creation-with-upload' extension", func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation")
				bytes, _, err := testClient.CreateUploadWithData(make([]byte, 10), 1024, false, nil)
				Ω(bytes).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(BeNil())
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation-with-upload")),
				))
			})
			DescribeTable("http errors handling",
				func(expectStatus int, expectErr error, expectErrMsg string) {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation", "creation-with-upload")
					d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
					eh := []string{"Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("1024")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Upload-Concat", expect.ToEqual("partial")).
						Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
						Body(expect.ToEqual(d)).
						Reply(tReply(reply.Status(expectStatus))),
					)

					bytes, _, err := testClient.CreateUploadWithData(d, 1024, true, nil)
					Ω(bytes).Should(BeEquivalentTo(0))
					Ω(tErrResponse(err).StatusCode).Should(Equal(expectStatus))
					Ω(err).Should(And(
						MatchError(expectErr),
						MatchError(ContainSubstring(expectErrMsg)),
					))
				},
				Entry("409", http.StatusConflict, ErrOffsetsNotSynced, "client stream and server offsets are not synced (HTTP 409;"),
				Entry("403", http.StatusForbidden, ErrCannotUpload, "can not upload (HTTP 403;"),
				Entry("410", http.StatusGone, ErrUploadDoesNotExist, "upload does not exist (HTTP 410;"),
				Entry("404", http.StatusNotFound, ErrUploadDoesNotExist, "upload does not exist (HTTP 404;"),
				Entry("413", http.StatusRequestEntityTooLarge, ErrUploadTooLarge, "upload is too large (HTTP 413;"),
				Entry("460", 460, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 460;"),
				Entry("401", http.StatusUnauthorized, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 401;"),
				Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
				Entry("200", http.StatusOK, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 200;"),
			)
			When("server creates upload but does not advance the offset", func() {
				It("should return ErrZeroProgress and the created upload", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation", "creation-with-upload")
					d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 512))
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Content-Length", expect.ToEqual("512")).
						Header("Upload-Length", expect.ToEqual("1024")).
						Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
						Body(expect.ToEqual(d)).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar").
							Header("Upload-Offset", "0")),
					)

					bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
					Ω(bytes).Should(BeEquivalentTo(0))
					Ω(tErrResponse(err).StatusCode).Should(Equal(http.StatusCreated))
					Ω(err).Should(And(
						MatchError(ErrZeroProgress),
						MatchError(ContainSubstring("offset=0")),
					))
					// Upload is created anyway, so the caller can continue the transfer with UploadStream
					Ω(u).Should(Equal(Upload{
						RemoteSize:   1024,
						Location:     "/foo/bar",
						RemoteOffset: 0,
					}))
				})
			})
			When("server advances the offset to an invalid value", func() {
				DescribeTable("should return protocol error",
					func(offset int64, expectMsg string) {
						testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "creation", "creation-with-upload")
						d, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 512))
						eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
						srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
							Header("Content-Length", expect.ToEqual("512")).
							Header("Upload-Length", expect.ToEqual("1024")).
							Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
							Body(expect.ToEqual(d)).
							Reply(tReply(reply.Created()).
								Header("Location", "/foo/bar").
								Header("Upload-Offset", strconv.FormatInt(offset, 10))),
						)

						bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
						Ω(bytes).Should(Equal(offset))
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusCreated))
						Ω(err).Should(And(
							MatchError(ErrProtocol),
							MatchError(ContainSubstring(expectMsg)),
						))
						Ω(u.Location).Should(Equal("/foo/bar"))
					},
					Entry("below zero", int64(-1), "server offset has gone backwards, offset=-1, transferredDataSize=512"),
					Entry("more than data size", int64(513), "server offset has been advanced too far: offset=513, transferredDataSize=512"),
					Entry("more than data size, but within upload size", int64(1024), "server offset has been advanced too far: offset=1024, transferredDataSize=512"),
				)
			})
		})
	})
	Context("DeleteUpload", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "termination")
			})
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusNoContent, nil, func() error {
					return testClient.DeleteUpload("/foo/bar")
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				srvMock.AddMocks(tRequest(http.MethodDelete, "/foo/bar", tusHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.NoContent())))
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders

				Ω(testClient.DeleteUpload("/foo/bar")).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodDelete, srvMock.URL() + "/foo/bar"}}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				srvMock.AddMocks(tRequest(http.MethodDelete, "/foo/bar", tusHeaders).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.NoContent())))
				testClient.ExtraHeaders = extraHeaders

				Ω(testClient.DeleteUpload("/foo/bar")).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
			})
			Specify("make a request", func() {
				srvMock.AddMocks(
					tRequest(http.MethodDelete, "/foo/bar", tusHeaders).
						Reply(tReply(reply.NoContent())))
				Ω(testClient.DeleteUpload("/foo/bar")).Should(Succeed())
			})
		})
		Context("error path", func() {
			It("should keep the response body in the error and close the original body", func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "termination")
				tExpectBodyKeptInError(testClient, http.StatusNotFound, nil, ErrUploadDoesNotExist, func() error {
					return testClient.DeleteUpload("/foo/bar")
				})
			})
			Specify("no termination extension", func() {
				err := testClient.DeleteUpload("/foo/bar")
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: termination")),
				))
			})
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectErrMsg string) {
						testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "termination")
						srvMock.AddMocks(tRequest(http.MethodDelete, "/foo/bar", tusHeaders).Reply(reply.Status(status)))

						err := testClient.DeleteUpload("/foo/bar")
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(status))
						Ω(err).Should(And(
							MatchError(expectErr), MatchError(ContainSubstring(expectErrMsg)),
						))
					},
					Entry("413", http.StatusRequestEntityTooLarge, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 413;"),
					Entry("404", http.StatusNotFound, ErrUploadDoesNotExist, "upload does not exist (HTTP 404;"),
					Entry("410", http.StatusGone, ErrUploadDoesNotExist, "upload does not exist (HTTP 410;"),
					Entry("403", http.StatusForbidden, ErrUploadDoesNotExist, "upload does not exist (HTTP 403;"),
					Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
					Entry("200", http.StatusOK, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 200;"),
				)
			})
		})
	})
	Context("ConcatenateUploads", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")
			})
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusCreated, http.Header{"Location": {"/foo/baz"}}, func() error {
					_, err := testClient.ConcatenateUploads([]Upload{{Location: "/foo/bar", Partial: true}}, nil)
					return err
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

				f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, nil)
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodPost, srvMock.URL()}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar/baz"}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				testClient.ExtraHeaders = extraHeaders
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

				f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, nil)
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar/baz"}))
			})
			When("send several uploads, no metadata", func() {
				It("should make a request", func() {
					eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
						Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
					)
					f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
					f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

					f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, nil)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						Location: "/foo/bar/baz",
						Partial:  false,
					}))
				})
			})
			When("send several uploads, with metadata", func() {
				It("should make a request", func() {
					eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Checksum", "Upload-Offset"}
					md := map[string]string{
						"key1": "value1",
						"key2": "&^%$\"\t",
					}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
						Header("Upload-Metadata", expect.Func(func(v any, _ expect.Args) (bool, error) {
							m, e := DecodeMetadata(v.(string))
							return reflect.DeepEqual(m, md), e
						})).
						Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
					)
					f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
					f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

					f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, md)
					Ω(err).Should(Succeed())
					Ω(f).Should(Equal(Upload{
						Location: "/foo/bar/baz",
						Partial:  false,
						Metadata: md,
					}))
				})
			})
		})
		Context("error path", func() {
			It("should keep the response body in the error and close the original body", func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")
				tExpectBodyKeptInError(testClient, http.StatusNotFound, nil, ErrUploadDoesNotExist, func() error {
					_, err := testClient.ConcatenateUploads([]Upload{{Location: "/foo/bar", Partial: true}}, nil)
					return err
				})
			})
			When("uploads list is empty", func() {
				It("should return error", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")

					f, err := testClient.ConcatenateUploads(nil, nil)
					Ω(err).Should(MatchError("partial uploads list is empty"))
					Ω(f).Should(Equal(Upload{}))
				})
			})
			When("no concatenation extension", func() {
				It("should return error", func() {
					f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
					f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

					f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, nil)
					Ω(tErrResponse(err)).Should(BeNil())
					Ω(err).Should(MatchError(ContainSubstring("unsupported feature: concatenation")))
					Ω(f).Should(Equal(Upload{}))
				})
			})
			When("some uploads are not partial", func() {
				It("should return error", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")
					f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
					f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: false}
					f3 := Upload{Location: "/foo/baa", RemoteSize: 512, RemoteOffset: 512, Partial: true}

					f, err := testClient.ConcatenateUploads([]Upload{f1, f2, f3}, nil)
					Ω(tErrResponse(err)).Should(BeNil())
					Ω(err).Should(MatchError(ContainSubstring("upload \"/foo/baz\" is not partial")))
					Ω(f).Should(Equal(Upload{}))
				})
			})
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectErrMsg string) {
						eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
						testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")
						srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
							Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
							Reply(reply.Status(status)))
						f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
						f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}

						f, err := testClient.ConcatenateUploads([]Upload{f1, f2}, nil)
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(status))
						Ω(err).Should(And(
							MatchError(expectErr),
							MatchError(ContainSubstring(expectErrMsg)),
						))
						Ω(f).Should(Equal(Upload{}))
					},
					Entry("404", http.StatusNotFound, ErrUploadDoesNotExist, "upload does not exist (HTTP 404;"),
					Entry("410", http.StatusGone, ErrUploadDoesNotExist, "upload does not exist (HTTP 410;"),
					Entry("403", http.StatusForbidden, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 403;"),
					Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
					Entry("200", http.StatusOK, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 200;"),
				)
			})
		})
	})
	Context("ConcatenateStreams", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation", "concatenation-unfinished")
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
				s1 := NewUploadStream(testClient, &f1)
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}
				s2 := NewUploadStream(testClient, &f2)

				f, err := testClient.ConcatenateStreams([]*UploadStream{s1, s2}, nil)
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodPost, srvMock.URL()}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar/baz"}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				testClient.ExtraHeaders = extraHeaders
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
				s1 := NewUploadStream(testClient, &f1)
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}
				s2 := NewUploadStream(testClient, &f2)

				f, err := testClient.ConcatenateStreams([]*UploadStream{s1, s2}, nil)
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(f).Should(Equal(Upload{Location: "/foo/bar/baz"}))
			})
			Specify("all streams are finished", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 256, Partial: true}
				s1 := NewUploadStream(testClient, &f1)
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 512, Partial: true}
				s2 := NewUploadStream(testClient, &f2)

				f, err := testClient.ConcatenateStreams([]*UploadStream{s1, s2}, nil)
				Ω(err).Should(Succeed())
				Ω(f).Should(Equal(Upload{
					Location: "/foo/bar/baz",
					Partial:  false,
				}))
			})
			Specify("some streams are not finished", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 64, Partial: true}
				s1 := NewUploadStream(testClient, &f1)
				f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 128, Partial: true}
				s2 := NewUploadStream(testClient, &f2)

				f, err := testClient.ConcatenateStreams([]*UploadStream{s1, s2}, nil)
				Ω(err).Should(Succeed())
				Ω(f).Should(Equal(Upload{
					Location: "/foo/bar/baz",
					Partial:  false,
				}))
			})
		})
		Context("error path", func() {
			When("streams list is empty", func() {
				DescribeTable("should return error without making a request",
					func(streams []*UploadStream) {
						testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation", "concatenation-unfinished")
						testClient.DoRequest = spy.DoRequest

						f, err := testClient.ConcatenateStreams(streams, nil)
						Ω(err).Should(MatchError("streams list is empty"))
						Ω(f).Should(Equal(Upload{}))
						Ω(spy.calls).Should(BeEmpty())
					},
					Entry("nil", nil),
					Entry("empty slice", []*UploadStream{}),
				)
			})
			When("some streams are not finished and no 'concatenation-unfinished' extension", func() {
				It("should return error", func() {
					testClient.Capabilities.Extensions = append(testClient.Capabilities.Extensions, "concatenation")
					f1 := Upload{Location: "/foo/bar", RemoteSize: 256, RemoteOffset: 64, Partial: true}
					s1 := NewUploadStream(testClient, &f1)
					f2 := Upload{Location: "/foo/baz", RemoteSize: 512, RemoteOffset: 128, Partial: true}
					s2 := NewUploadStream(testClient, &f2)

					_, err := testClient.ConcatenateStreams([]*UploadStream{s1, s2}, nil)
					Ω(tErrResponse(err)).Should(BeNil())
					Ω(err).Should(And(
						MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: concatenation-unfinished")),
					))
				})
			})
		})
	})
	Context("UpdateCapabilities", func() {
		Context("happy path", func() {
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusNoContent, nil, func() error {
					_, err := testClient.UpdateCapabilities()
					return err
				})
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				srvMock.AddMocks(
					mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
						Header("X-Custom", expect.ToEqual("custom")).
						Header("X-Extra", expect.ToEqual("extra")).
						Reply(tReply(reply.NoContent()).
							Header("Tus-Version", "1.0.0").
							Header("Tus-Extension", "creation")),
				)
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = extraHeaders

				Ω(testClient.UpdateCapabilities()).ShouldNot(BeNil())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodOptions, srvMock.URL()}}))
				Ω(*testClient.Capabilities).Should(Equal(ServerCapabilities{
					Extensions:       []string{"creation"},
					ProtocolVersions: []string{"1.0.0"},
				}))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				srvMock.AddMocks(
					mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
						Header("X-Extra", expect.ToEqual("extra")).
						Reply(tReply(reply.NoContent()).
							Header("Tus-Version", "1.0.0").
							Header("Tus-Extension", "creation")),
				)
				testClient.ExtraHeaders = extraHeaders

				Ω(testClient.UpdateCapabilities()).ShouldNot(BeNil())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(*testClient.Capabilities).Should(Equal(ServerCapabilities{
					Extensions:       []string{"creation"},
					ProtocolVersions: []string{"1.0.0"},
				}))
			})
			DescribeTable("should fill client capabilities",
				func(status int) {
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.Status(status)).
								Header("Tus-Version", "1.0.0,0.2.2,0.2.1").
								Header("Tus-Max-Size", "1073741824").
								Header("Tus-Extension", "creation,expiration,checksum").
								Header("Tus-Checksum-Algorithm", "sha1,md5")),
					)
					Ω(testClient.UpdateCapabilities()).ShouldNot(BeNil())
					Ω(*testClient.Capabilities).Should(Equal(ServerCapabilities{
						Extensions:         []string{"creation", "expiration", "checksum"},
						MaxSize:            1073741824,
						ProtocolVersions:   []string{"1.0.0", "0.2.2", "0.2.1"},
						ChecksumAlgorithms: []string{"sha1", "md5"},
					}))
				},
				Entry("200", http.StatusOK),
				Entry("204", http.StatusNoContent),
			)
		})
		Context("error path", func() {
			It("should keep the response body in the error and close the original body", func() {
				tExpectBodyKeptInError(testClient, http.StatusInternalServerError, nil, ErrUnexpectedResponse, func() error {
					_, err := testClient.UpdateCapabilities()
					return err
				})
			})
			When("corrupted number in Tus-Max-Size", func() {
				It("should return error", func() {
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.OK()).
								Header("Tus-Version", "1.0.0,0.2.2,0.2.1").
								Header("Tus-Max-Size", "fdsa107374182dw4").
								Header("Tus-Extension", "creation,expiration,checksum").
								Header("Tus-Checksum-Algorithm", "sha1,md5")),
					)
					resp, err := testClient.UpdateCapabilities()
					Ω(resp).ShouldNot(BeNil())
					Ω(err).Should(MatchError(ContainSubstring("parse Tus-Max-Size integer value \"fdsa107374182dw4\"")))
				})
			})
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectErrMsg string) {
						srvMock.AddMocks(mocha.Request().
							URL(expect.URLPath("/")).Method(http.MethodOptions).
							Header("Tus-Resumable", expect.ToBeEmpty()). // OPTIONS request should not contain this header
							Reply(tReply(reply.Status(status))),
						)

						resp, err := testClient.UpdateCapabilities()
						Ω(resp).ShouldNot(BeNil())
						Ω(err).Should(And(
							MatchError(expectErr),
							MatchError(ContainSubstring(expectErrMsg)),
						))
					},
					Entry("404", http.StatusNotFound, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 404;"),
					Entry("410", http.StatusGone, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 410;"),
					Entry("403", http.StatusForbidden, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 403;"),
					Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
					Entry("201", http.StatusCreated, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 201;"),
				)
			})
		})
	})
	Context("ensureExtension", func() {
		When("extension exists", func() {
			When("capabilities are empty", func() {
				It("should request from server and return no error", func() {
					testClient.Capabilities = nil
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.OK()).
								Header("Tus-Version", "1.0.0,0.2.2,0.2.1").
								Header("Tus-Max-Size", "1073741824").
								Header("Tus-Extension", "creation,expiration,checksum").
								Header("Tus-Checksum-Algorithm", "sha1,md5")),
					)
					Ω(testClient.ensureExtension("creation")).Should(Succeed())
				})
			})
			When("capabilities are not empty", func() {
				It("should use cache and return no error", func() {
					testClient.Capabilities.Extensions = []string{"creation", "expiration"}
					Ω(testClient.ensureExtension("creation")).Should(Succeed())
				})
			})
		})
		When("no such extension", func() {
			It("should return error", func() {
				Ω(testClient.ensureExtension("creation")).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: creation")),
				))
			})
		})
	})
})

var _ = Describe("Metadata", func() {
	Context("EncodeMetadata", func() {
		// Map iteration order is random, so the items are compared regardless of their order
		encodedItems := func(metadata map[string]string) []string {
			s, err := EncodeMetadata(metadata)
			Ω(err).Should(Succeed())
			return strings.Split(s, ",")
		}

		DescribeTable("should encode metadata",
			func(metadata map[string]string, expectItems []string) {
				Ω(encodedItems(metadata)).Should(ConsistOf(expectItems))
			},
			Entry("single item", map[string]string{"key": "value"}, []string{"key dmFsdWU="}),
			Entry("multiple items separated by commas",
				map[string]string{"key1": "value1", "key2": "&^%$\"\t"}, []string{"key1 dmFsdWUx", "key2 Jl4lJCIJ"}),
			Entry("empty value as an empty base64 string", map[string]string{"key": ""}, []string{"key "}),
			Entry("value containing spaces and commas", map[string]string{"key": "a b,c"}, []string{"key YSBiLGM="}),
			Entry("non-ASCII value", map[string]string{"filename": "файл.txt"}, []string{"filename 0YTQsNC50LsudHh0"}),
		)
		DescribeTable("should return an empty string for empty metadata",
			func(metadata map[string]string) {
				Ω(EncodeMetadata(metadata)).Should(BeEmpty())
			},
			Entry("nil map", nil),
			Entry("empty map", map[string]string{}),
		)
		DescribeTable("should return error if a key is invalid",
			func(key, expectMsg string) {
				s, err := EncodeMetadata(map[string]string{key: "value"})
				Ω(err).Should(MatchError(expectMsg))
				Ω(s).Should(BeEmpty())
			},
			Entry("space in the middle", "my key", `key "my key" contains spaces`),
			Entry("leading space", " key", `key " key" contains spaces`),
			Entry("trailing space", "key ", `key "key " contains spaces`),
			Entry("comma in the middle", "my,key", `key "my,key" contains commas`),
			Entry("leading comma", ",key", `key ",key" contains commas`),
			Entry("trailing comma", "key,", `key "key," contains commas`),
			Entry("empty key", "", `empty key`),
		)
		It("should return error if an invalid key is among valid ones", func() {
			s, err := EncodeMetadata(map[string]string{"key1": "value1", "bad,key": "value2", "key3": "value3"})
			Ω(err).Should(MatchError(`key "bad,key" contains commas`))
			Ω(s).Should(BeEmpty())
		})
	})

	Context("DecodeMetadata", func() {
		DescribeTable("should decode metadata",
			func(raw string, expectMetadata map[string]string) {
				Ω(DecodeMetadata(raw)).Should(Equal(expectMetadata))
			},
			Entry("single item", "key dmFsdWU=", map[string]string{"key": "value"}),
			Entry("multiple items", "key1 dmFsdWUx,key2 Jl4lJCIJ", map[string]string{"key1": "value1", "key2": "&^%$\"\t"}),
			Entry("empty value", "key ", map[string]string{"key": ""}),
			Entry("empty value without separating space", "key", map[string]string{"key": ""}),
			Entry("empty value without separating space among other items", "key1 dmFsdWUx,key2,key3 dmFsdWUz",
				map[string]string{"key1": "value1", "key2": "", "key3": "value3"}),
			Entry("value containing spaces and commas", "key YSBiLGM=", map[string]string{"key": "a b,c"}),
			Entry("non-ASCII value", "filename 0YTQsNC50LsudHh0", map[string]string{"filename": "файл.txt"}),
			Entry("duplicated key, keeping the last value", "key dmFsdWUx,key dmFsdWUy", map[string]string{"key": "value2"}),
		)
		DescribeTable("should return error",
			func(raw, expectMsg string) {
				_, err := DecodeMetadata(raw)
				Ω(err).Should(MatchError(expectMsg))
			},
			Entry("empty string", "", `metadata item "" has bad format`),
			Entry("item without key", "key1 dmFsdWUx, dmFsdWUy", `metadata item " dmFsdWUy" has bad format`),
			Entry("trailing comma", "key dmFsdWU=,", `metadata item "" has bad format`),
			Entry("empty item in the middle", "key1 dmFsdWUx,,key2 dmFsdWUy", `metadata item "" has bad format`),
			Entry("illegal base64 characters", "key !!!!", "illegal base64 data at input byte 0"),
			Entry("missing padding", "key dmFsdWU", "illegal base64 data at input byte 4"),
			Entry("URL-safe alphabet", "key Pz8_", "illegal base64 data at input byte 3"),
			Entry("extra space before the value", "key  dmFsdWU=", "illegal base64 data at input byte 0"),
		)
		It("should return the items decoded before the error", func() {
			m, err := DecodeMetadata("key1 dmFsdWUx,key2 !!!!,key3 dmFsdWUz")
			Ω(err).Should(HaveOccurred())
			Ω(m).Should(Equal(map[string]string{"key1": "value1"}))
		})
	})

	Context("round trip", func() {
		DescribeTable("should decode the encoded metadata to the same map",
			func(metadata map[string]string) {
				s, err := EncodeMetadata(metadata)
				Ω(err).Should(Succeed())
				Ω(DecodeMetadata(s)).Should(Equal(metadata))
			},
			Entry("single item", map[string]string{"key": "value"}),
			Entry("multiple items", map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}),
			Entry("empty value", map[string]string{"key1": "", "key2": "value2"}),
			Entry("special characters in values", map[string]string{"key": "a b,c\x00\n\t\"'"}),
			Entry("non-ASCII key and value", map[string]string{"ключ": "значение"}),
		)
	})
})
