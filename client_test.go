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
	"sync"
	"sync/atomic"
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

// http.Client must be usable as Client.Client
var _ HTTPClient = (*http.Client)(nil)

// countingClient is an HTTPClient that counts its calls, safe for concurrent use. Its Do method may also be used as
// a DoRequestFunc.
type countingClient struct {
	requests atomic.Int32
}

func (c *countingClient) Do(req *http.Request) (*http.Response, error) {
	c.requests.Add(1)
	return http.DefaultClient.Do(req)
}

type requestCall struct {
	method string
	url    string
}

// spyClient is an HTTPClient that records its calls and sets the X-Custom header in every request. Its Do method may
// also be used as a DoRequestFunc.
type spyClient struct {
	calls []requestCall
}

func (s *spyClient) Do(req *http.Request) (*http.Response, error) {
	s.calls = append(s.calls, requestCall{method: req.Method, url: req.URL.String()})
	req.Header.Set("X-Custom", "custom")
	return http.DefaultClient.Do(req)
}

// failingClient is an HTTPClient that fails every request with err
type failingClient struct {
	err error
}

func (f *failingClient) Do(*http.Request) (*http.Response, error) {
	return nil, f.err
}

// fakeResponder is an HTTPClient that replies with the given status, headers and body without making a real
// request, and records the response bodies to check whether they have been closed
type fakeResponder struct {
	status int
	header http.Header
	body   string
	bodies []*closeTrackingBody
}

func (f *fakeResponder) Do(req *http.Request) (*http.Response, error) {
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
	c.DoRequest = responder.Do

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
	c.DoRequest = responder.Do

	Ω(call()).Should(Succeed())
	Ω(responder.bodies).Should(HaveLen(1))
	Ω(responder.bodies[0].closed).Should(BeTrue())
}

var _ = Describe("Client", func() {
	var testClient *Client
	var testURL *url.URL
	var srvMock *mocha.Mocha
	var tusHeaders []string
	var spy *spyClient
	var extraHeaders http.Header

	BeforeEach(func() {
		srvMock = mocha.New(GinkgoT())
		srvMock.Start()
		testURL, _ = url.Parse(srvMock.URL())
		testClient = NewClient(testURL)
		testClient.caps = &ServerCapabilities{Extensions: []string{}, ProtocolVersions: []string{"1.0.0"}}
		testClient.protocolVersion = "1.0.0"
		tusHeaders = []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
		spy = &spyClient{}
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
			cl := NewClient(testURL)
			Ω(cl.protocolVersion).Should(BeEmpty()) // negotiated with the server on the first request
			Ω(cl.caps).Should(BeNil())              // fetched from the server on the first need
			Ω(cl.DoRequest).Should(BeNil())
			Ω(cl.Client).Should(BeNil())
			Ω(cl.ExtraHeaders).Should(Equal(http.Header{}))
			Ω(cl.endpoint).Should(Equal(testURL))
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
				It("should not set Tus-Resumable header nor negotiate the protocol version", func() {
					testClient.protocolVersion = ""
					srvMock.AddMocks(mocha.Request().
						URL(expect.URLPath("/foo")).Method(http.MethodOptions).
						Header("Tus-Resumable", expect.ToBeEmpty()). // OPTIONS request should not contain this header
						Reply(tReply(reply.OK())),
					)
					req, err := http.NewRequest(http.MethodOptions, srvMock.URL()+"/foo", nil)
					Ω(err).Should(Succeed())

					Ω(testClient.tusRequest(req)).ShouldNot(BeNil())
					Ω(testClient.protocolVersion).Should(BeEmpty())
				})
			})
			It("should override Tus-Resumable header with the negotiated protocol version", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK())))
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())
				req.Header.Set("Tus-Resumable", "0.2.2")

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
			})
			It("should use http.DefaultClient if neither DoRequest nor Client is set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK())))
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
			})
			It("should use Client if DoRequest is not set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Reply(tReply(reply.OK())),
				)
				testClient.Client = spy
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodGet, srvMock.URL() + "/foo"}}))
			})
			It("should use DoRequest and ignore Client if both are set", func() {
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Reply(tReply(reply.OK())),
				)
				cl := &countingClient{}
				testClient.Client = cl
				testClient.DoRequest = spy.Do
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				Ω(testClient.tusRequest(req)).Should(HaveHTTPStatus(http.StatusOK))
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodGet, srvMock.URL() + "/foo"}}))
				Ω(cl.requests.Load()).Should(BeZero())
			})
			It("should return error returned by Client", func() {
				doErr := errors.New("boom")
				testClient.Client = &failingClient{err: doErr}
				req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
				Ω(err).Should(Succeed())

				resp, err := testClient.tusRequest(req)
				Ω(err).Should(MatchError(doErr))
				Ω(resp).Should(BeNil())
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
	Context("protocol version negotiation", func() {
		var counter *countingClient

		BeforeEach(func() {
			testClient.protocolVersion = ""
			testClient.caps = nil
			counter = &countingClient{}
			testClient.DoRequest = counter.Do
		})
		mockOptions := func(tusVersion string) {
			r := tReply(reply.NoContent())
			if tusVersion != "" {
				r = r.Header("Tus-Version", tusVersion)
			}
			srvMock.AddMocks(mocha.Request().
				URL(expect.URLPath("/")).Method(http.MethodOptions).
				Header("Tus-Resumable", expect.ToBeEmpty()).
				Reply(r),
			)
		}
		newRequest := func() *http.Request {
			req, err := http.NewRequest(http.MethodGet, srvMock.URL()+"/foo", nil)
			Ω(err).Should(Succeed())
			return req
		}

		Context("happy path", func() {
			DescribeTable("should pick the most recent supported version advertised by server",
				func(tusVersion, expectVersion string) {
					mockOptions(tusVersion)
					srvMock.AddMocks(mocha.Request().
						URL(expect.URLPath("/foo")).Method(http.MethodGet).
						Header("Tus-Resumable", expect.ToEqual(expectVersion)).
						Reply(tReply(reply.OK())),
					)

					Ω(testClient.tusRequest(newRequest())).Should(HaveHTTPStatus(http.StatusOK))
					Ω(testClient.protocolVersion).Should(Equal(expectVersion))
				},
				Entry("single version", "1.0.0", "1.0.0"),
				Entry("among older versions", "1.0.0,0.2.2,0.2.1", "1.0.0"),
				Entry("most recent patch version", "1.0.1,1.0.3,1.0.0,1.0.2", "1.0.3"),
				Entry("ignoring the newer major and minor versions", "2.0.0,1.1.0,1.0.1,0.2.2", "1.0.1"),
			)
			It("should negotiate the version only once", func() {
				mockOptions("1.0.0")
				srvMock.AddMocks(tRequest(http.MethodGet, "/foo", tusHeaders).Reply(tReply(reply.OK())).Repeat(2))

				Ω(testClient.tusRequest(newRequest())).Should(HaveHTTPStatus(http.StatusOK))
				Ω(testClient.tusRequest(newRequest())).Should(HaveHTTPStatus(http.StatusOK))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(3))
			})
			It("should negotiate the version on the first request made by a public method", func() {
				mockOptions("1.0.0,0.2.2")
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", tusHeaders).
					Reply(tReply(reply.OK()).Header("Upload-Offset", "64")),
				)

				Ω(testClient.GetUpload("/foo/bar")).Should(Equal(Upload{Location: "/foo/bar", RemoteOffset: 64}))
				Ω(testClient.protocolVersion).Should(Equal("1.0.0"))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(2))
			})
			It("should share the fetched capabilities with the extension check", func() {
				srvMock.AddMocks(mocha.Request().
					URL(expect.URLPath("/")).Method(http.MethodOptions).
					Header("Tus-Resumable", expect.ToBeEmpty()).
					Reply(tReply(reply.NoContent()).
						Header("Tus-Version", "1.0.0").
						Header("Tus-Extension", "creation")),
				)
				srvMock.AddMocks(tRequest(http.MethodPost, "/", []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata"}).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar")),
				)

				Ω(testClient.CreateUpload(1024, false, nil)).Should(Equal(Upload{Location: "/foo/bar", RemoteSize: 1024}))
				Ω(testClient.protocolVersion).Should(Equal("1.0.0"))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(2)) // single OPTIONS and POST
			})
			It("should use the cached capabilities without requesting the server", func() {
				testClient.caps = &ServerCapabilities{ProtocolVersions: []string{"0.2.2", "1.0.1"}}
				srvMock.AddMocks(mocha.Request().
					URL(expect.URLPath("/foo")).Method(http.MethodGet).
					Header("Tus-Resumable", expect.ToEqual("1.0.1")).
					Reply(tReply(reply.OK())),
				)

				Ω(testClient.tusRequest(newRequest())).Should(HaveHTTPStatus(http.StatusOK))
				Ω(testClient.protocolVersion).Should(Equal("1.0.1"))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
			})
		})
		Context("error path", func() {
			DescribeTable("should return error, not send the request and not cache the version",
				func(tusVersion, expectErrMsg string) {
					mockOptions(tusVersion)

					resp, err := testClient.tusRequest(newRequest())
					Ω(err).Should(And(
						MatchError(ErrUnsupportedFeature),
						MatchError(ContainSubstring("get supported protocol version")),
						MatchError(ContainSubstring(expectErrMsg)),
					))
					Ω(resp).Should(BeNil())
					Ω(testClient.protocolVersion).Should(BeEmpty())
					Ω(counter.requests.Load()).Should(BeEquivalentTo(1))

					// Capabilities are cached anyway, so the server is not requested again
					_, err = testClient.tusRequest(newRequest())
					Ω(err).Should(MatchError(ErrUnsupportedFeature))
					Ω(testClient.caps).ShouldNot(BeNil())
					Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
				},
				Entry("no versions advertised", "", "client supports protocol version 1.0...., but server advertises:"),
				Entry("only older versions advertised", "0.2.2,0.2.1",
					"client supports protocol version 1.0...., but server advertises: 0.2.2, 0.2.1"),
				Entry("only newer versions advertised", "2.0.0,1.1.0",
					"client supports protocol version 1.0...., but server advertises: 2.0.0, 1.1.0"),
			)
			It("should return error and not cache anything if fetching capabilities fails", func() {
				srvMock.AddMocks(mocha.Request().
					URL(expect.URLPath("/")).Method(http.MethodOptions).
					Reply(reply.Status(http.StatusInternalServerError)),
				)

				resp, err := testClient.tusRequest(newRequest())
				Ω(err).Should(And(
					MatchError(ErrUnexpectedResponse),
					MatchError(ContainSubstring("get supported protocol version: fetch capabilities")),
				))
				Ω(resp).Should(BeNil())
				Ω(testClient.protocolVersion).Should(BeEmpty())
				Ω(testClient.caps).Should(BeNil())
				Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
			})
			It("should return error from a public method", func() {
				mockOptions("0.2.2")

				_, err := testClient.GetUpload("/foo/bar")
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("server advertises: 0.2.2")),
				))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.GetUpload("/foo/bar")
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodHead, srvMock.URL() + "/foo/bar"}}))
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
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				f, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodPost, srvMock.URL()}}))
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
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
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
			When("server returns Upload-Expires header", func() {
				It("should parse the expiration time", func() {
					srvMock.AddMocks(tRequest(http.MethodPost, "/", nil).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar").
							Header("Upload-Expires", "Wed, 25 Jun 2014 16:00:00 GMT")),
					)

					f, err := testClient.CreateUpload(1024, false, nil)
					Ω(err).Should(Succeed())
					Ω(f.Expires).ShouldNot(BeNil())
					Ω(*f.Expires).Should(BeTemporally("==", time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)))
					f.Expires = nil
					Ω(f).Should(Equal(Upload{RemoteSize: 1024, Location: "/foo/bar"}))
				})
			})
			When("server does not return Upload-Expires header", func() {
				It("should leave the expiration time nil", func() {
					srvMock.AddMocks(tRequest(http.MethodPost, "/", nil).
						Reply(tReply(reply.Created()).
							Header("Location", "/foo/bar")),
					)

					f, err := testClient.CreateUpload(1024, false, nil)
					Ω(err).Should(Succeed())
					Ω(f.Expires).Should(BeNil())
				})
			})
		})
		Context("error path", func() {
			DescribeTable("should keep the response body in the error and close the original body",
				func(status int, header http.Header, expectErr error) {
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						_, err := testClient.CreateUpload(1024, false, nil)
						return err
					})
				},
				Entry("http error", http.StatusRequestEntityTooLarge, nil, ErrTooLarge),
				Entry("unsupported protocol version", http.StatusPreconditionFailed, nil, ErrProtocol),
				Entry("malformed Upload-Expires", http.StatusCreated, http.Header{"Location": {"/foo/bar"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
			)
			It("should return ErrProtocol if Upload-Expires header is not in RFC1123 format", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
				srvMock.AddMocks(tRequest(http.MethodPost, "/", nil).
					Reply(tReply(reply.Created()).
						Header("Location", "/foo/bar").
						Header("Upload-Expires", "2014-06-25T16:00:00Z")),
				)

				_, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(And(
					MatchError(ErrProtocol),
					MatchError(ContainSubstring(`parse RFC1123 datetime in Upload-Expires header "2014-06-25T16:00:00Z"`)),
				))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusCreated))
			})
			Specify("no creation extension", func() {
				_, err := testClient.CreateUpload(1024, false, nil)
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation")),
				))
			})
			Specify("no creation-defer-length extension and trying to create defer size upload", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
				_, err := testClient.CreateUpload(SizeUnknown, false, nil)
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation-defer-length")),
				))
			})
			When("upload size is negative", func() {
				It("should return an error", func() {
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
					_, err := testClient.CreateUpload(-2, false, nil)
					Ω(err).Should(MatchError(ContainSubstring("remoteSize is negative: -2")))
				})
			})
			Specify("metadata key contains a space", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
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
						testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
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
					Entry("413", http.StatusRequestEntityTooLarge, ErrTooLarge, "data is too large (HTTP 413;"),
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
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation", "creation-with-upload")
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				bytes, u, err := testClient.CreateUploadWithData(d, 1024, false, nil)
				Ω(bytes).Should(BeEquivalentTo(512))
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodPost, srvMock.URL()}}))
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
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation", "creation-with-upload")
					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						_, _, err := testClient.CreateUploadWithData(make([]byte, 10), 1024, false, nil)
						return err
					})
				},
				Entry("http error", http.StatusConflict, nil, ErrOffsetsNotSynced),
				Entry("zero progress", http.StatusCreated, http.Header{"Upload-Offset": {"0"}}, ErrIncompleteWrite),
				Entry("offset advanced too far", http.StatusCreated, http.Header{"Upload-Offset": {"50"}}, ErrProtocol),
			)
			Specify("no 'creation-with-upload' extension", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation")
				bytes, _, err := testClient.CreateUploadWithData(make([]byte, 10), 1024, false, nil)
				Ω(bytes).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(BeNil())
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature), MatchError(ContainSubstring("unsupported feature: creation-with-upload")),
				))
			})
			DescribeTable("http errors handling",
				func(expectStatus int, expectErr error, expectErrMsg string) {
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation", "creation-with-upload")
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
				Entry("413", http.StatusRequestEntityTooLarge, ErrTooLarge, "data is too large (HTTP 413;"),
				Entry("460", 460, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 460;"),
				Entry("401", http.StatusUnauthorized, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 401;"),
				Entry("400", http.StatusBadRequest, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 400;"),
				Entry("200", http.StatusOK, ErrUnexpectedResponse, "unexpected HTTP response code (HTTP 200;"),
			)
			When("server creates upload but accepts only a part of the data", func() {
				DescribeTable("should return ErrIncompleteWrite and the created upload",
					func(offset int64) {
						testClient.caps.Extensions = append(testClient.caps.Extensions, "creation", "creation-with-upload")
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
						Ω(tErrResponse(err).StatusCode).Should(Equal(http.StatusCreated))
						Ω(err).Should(And(
							MatchError(ErrIncompleteWrite),
							MatchError(ContainSubstring("offset="+strconv.FormatInt(offset, 10))),
						))
						// Upload is created anyway, so the caller can continue the transfer with ChunkedWriter or Stream
						Ω(u).Should(Equal(Upload{
							RemoteSize:   1024,
							Location:     "/foo/bar",
							RemoteOffset: offset,
						}))
					},
					Entry("nothing", int64(0)),
					Entry("partially", int64(100)),
					Entry("all except the last byte", int64(511)),
				)
			})
			When("server advances the offset to an invalid value", func() {
				DescribeTable("should return protocol error",
					func(offset int64, expectMsg string) {
						testClient.caps.Extensions = append(testClient.caps.Extensions, "creation", "creation-with-upload")
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
					Entry("below zero", int64(-1), "server offset is negative, offset=-1, transferredDataSize=512"),
					Entry("more than data size", int64(513), "server offset has been advanced too far, offset=513, transferredDataSize=512"),
					Entry("more than data size, but within upload size", int64(1024), "server offset has been advanced too far, offset=1024, transferredDataSize=512"),
				)
			})
		})
	})
	Context("DeleteUpload", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "termination")
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				Ω(testClient.DeleteUpload("/foo/bar")).Should(Succeed())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodDelete, srvMock.URL() + "/foo/bar"}}))
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
				testClient.caps.Extensions = append(testClient.caps.Extensions, "termination")
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
						testClient.caps.Extensions = append(testClient.caps.Extensions, "termination")
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
	Context("ConcatenatePartialUploads", func() {
		Context("happy path", func() {
			BeforeEach(func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "concatenation")
			})
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusCreated, http.Header{"Location": {"/foo/baz"}}, func() error {
					_, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar"}, nil)
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
				Ω(err).Should(Succeed())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodPost, srvMock.URL()}}))
				Ω(loc).Should(Equal("/foo/bar/baz"))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)
				testClient.ExtraHeaders = extraHeaders

				loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
				Ω(err).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(loc).Should(Equal("/foo/bar/baz"))
			})
			When("send several uploads, no metadata", func() {
				It("should make a request", func() {
					eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
						Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
						Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
					)

					loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
					Ω(err).Should(Succeed())
					Ω(loc).Should(Equal("/foo/bar/baz"))
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

					loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, md)
					Ω(err).Should(Succeed())
					Ω(loc).Should(Equal("/foo/bar/baz"))
				})
			})
		})
		DescribeTable("should accept any of concatenation extensions",
			func(extensions []string) {
				testClient.caps.Extensions = extensions
				eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
					Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
					Reply(tReply(reply.Created()).Header("Location", "/foo/bar/baz")),
				)

				loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
				Ω(err).Should(Succeed())
				Ω(loc).Should(Equal("/foo/bar/baz"))
			},
			Entry("concatenation", []string{"concatenation"}),
			Entry("concatenation-unfinished", []string{"concatenation-unfinished"}),
			Entry("both", []string{"concatenation", "concatenation-unfinished"}),
		)
		Context("error path", func() {
			It("should keep the response body in the error and close the original body", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "concatenation")
				tExpectBodyKeptInError(testClient, http.StatusNotFound, nil, ErrUploadDoesNotExist, func() error {
					_, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar"}, nil)
					return err
				})
			})
			When("locations list is empty", func() {
				DescribeTable("should return error without making a request",
					func(locations []string) {
						testClient.caps.Extensions = append(testClient.caps.Extensions, "concatenation")
						testClient.DoRequest = spy.Do

						loc, err := testClient.ConcatenatePartialUploads(locations, nil)
						Ω(err).Should(MatchError("locations list is empty"))
						Ω(loc).Should(BeEmpty())
						Ω(spy.calls).Should(BeEmpty())
					},
					Entry("nil", nil),
					Entry("empty slice", []string{}),
				)
			})
			When("no concatenation extensions", func() {
				It("should return error", func() {
					loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
					Ω(tErrResponse(err)).Should(BeNil())
					Ω(err).Should(And(
						MatchError(ErrUnsupportedFeature),
						MatchError(ContainSubstring("either concatenation or concatenation-unfinished")),
					))
					Ω(loc).Should(BeEmpty())
				})
			})
			When("http error or unexpected code", func() {
				DescribeTable("should return error",
					func(status int, expectErr error, expectErrMsg string) {
						eh := []string{"Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
						testClient.caps.Extensions = append(testClient.caps.Extensions, "concatenation")
						srvMock.AddMocks(tRequest(http.MethodPost, "/", eh).
							Header("Upload-Concat", expect.ToEqual("final;/foo/bar /foo/baz")).
							Reply(reply.Status(status)))

						loc, err := testClient.ConcatenatePartialUploads([]string{"/foo/bar", "/foo/baz"}, nil)
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(status))
						Ω(err).Should(And(
							MatchError(expectErr),
							MatchError(ContainSubstring(expectErrMsg)),
						))
						Ω(loc).Should(BeEmpty())
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
	Context("FetchCapabilities", func() {
		Context("happy path", func() {
			It("should close the response body", func() {
				tExpectBodyClosed(testClient, http.StatusNoContent, nil, func() error {
					_, err := testClient.FetchCapabilities()
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
				testClient.DoRequest = spy.Do
				testClient.ExtraHeaders = extraHeaders

				caps, err := testClient.FetchCapabilities()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(spy.calls).Should(Equal([]requestCall{{http.MethodOptions, srvMock.URL()}}))
				Ω(caps).Should(Equal(ServerCapabilities{
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

				caps, err := testClient.FetchCapabilities()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(caps).Should(Equal(ServerCapabilities{
					Extensions:       []string{"creation"},
					ProtocolVersions: []string{"1.0.0"},
				}))
			})
			It("should not affect the client state", func() {
				srvMock.AddMocks(
					mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
						Reply(tReply(reply.NoContent()).
							Header("Tus-Version", "1.0.0").
							Header("Tus-Extension", "creation,termination")),
				)
				testClient.caps = nil
				testClient.protocolVersion = ""

				caps, err := testClient.FetchCapabilities()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(caps.Extensions).Should(Equal([]string{"creation", "termination"}))
				Ω(testClient.caps).Should(BeNil())
				Ω(testClient.protocolVersion).Should(BeEmpty())
			})
			DescribeTable("should return the server capabilities",
				func(status int) {
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.Status(status)).
								Header("Tus-Version", "1.0.0,0.2.2,0.2.1").
								Header("Tus-Max-Size", "1073741824").
								Header("Tus-Extension", "creation,expiration,checksum").
								Header("Tus-Checksum-Algorithm", "sha1,md5")),
					)
					caps, err := testClient.FetchCapabilities()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(caps).Should(Equal(ServerCapabilities{
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
					_, err := testClient.FetchCapabilities()
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
					_, err := testClient.FetchCapabilities()
					Ω(err).Should(MatchError(ErrProtocol))
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

						_, err := testClient.FetchCapabilities()
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
		var counter *countingClient

		BeforeEach(func() {
			counter = &countingClient{}
			testClient.DoRequest = counter.Do
		})
		When("extension exists", func() {
			When("extensions have not been fetched yet", func() {
				BeforeEach(func() {
					testClient.caps = nil
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.OK()).
								Header("Tus-Version", "1.0.0,0.2.2,0.2.1").
								Header("Tus-Max-Size", "1073741824").
								Header("Tus-Extension", "creation,expiration,checksum").
								Header("Tus-Checksum-Algorithm", "sha1,md5")),
					)
				})
				It("should request from server, cache the extensions and return no error", func() {
					Ω(testClient.ensureExtension("creation")).Should(Succeed())
					Ω(testClient.caps).Should(Equal(&ServerCapabilities{
						Extensions:         []string{"creation", "expiration", "checksum"},
						MaxSize:            1073741824,
						ProtocolVersions:   []string{"1.0.0", "0.2.2", "0.2.1"},
						ChecksumAlgorithms: []string{"sha1", "md5"},
					}))
					Ω(testClient.ensureExtension("checksum")).Should(Succeed())
					Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
				})
				It("should request from server only once when called concurrently", func() {
					var wg sync.WaitGroup
					for range 10 {
						wg.Add(1)
						go func() {
							defer GinkgoRecover()
							defer wg.Done()
							Ω(testClient.ensureExtension("creation")).Should(Succeed())
						}()
					}
					wg.Wait()
					Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
					Ω(testClient.caps.Extensions).Should(Equal([]string{"creation", "expiration", "checksum"}))
				})
			})
			When("extensions have been fetched", func() {
				It("should use cache and return no error", func() {
					testClient.caps = &ServerCapabilities{Extensions: []string{"creation", "expiration"}}
					Ω(testClient.ensureExtension("creation")).Should(Succeed())
					Ω(counter.requests.Load()).Should(BeZero())
				})
			})
		})
		When("no such extension", func() {
			It("should return error", func() {
				Ω(testClient.ensureExtension("creation")).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: creation")),
				))
				Ω(counter.requests.Load()).Should(BeZero())
			})
			When("server advertises no extensions", func() {
				It("should cache the capabilities and return error", func() {
					testClient.caps = nil
					srvMock.AddMocks(
						mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
							Reply(tReply(reply.NoContent()).Header("Tus-Version", "1.0.0")),
					)
					Ω(testClient.ensureExtension("creation")).Should(MatchError(ErrUnsupportedFeature))
					Ω(testClient.caps).Should(Equal(&ServerCapabilities{ProtocolVersions: []string{"1.0.0"}}))
					Ω(testClient.ensureExtension("termination")).Should(MatchError(ErrUnsupportedFeature))
					Ω(counter.requests.Load()).Should(BeEquivalentTo(1))
				})
			})
		})
		When("fetching extensions fails", func() {
			It("should return error and not cache anything", func() {
				testClient.caps = nil
				srvMock.AddMocks(
					mocha.Request().URL(expect.URLPath("/")).Method(http.MethodOptions).
						Reply(reply.Status(http.StatusInternalServerError)),
				)
				Ω(testClient.ensureExtension("creation")).Should(And(
					MatchError(ErrUnexpectedResponse),
					MatchError(ContainSubstring("fetch capabilities")),
				))
				Ω(testClient.caps).Should(BeNil())
				Ω(testClient.ensureExtension("creation")).Should(MatchError(ErrUnexpectedResponse))
				Ω(counter.requests.Load()).Should(BeEquivalentTo(2))
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
