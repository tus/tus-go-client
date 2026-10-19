package tusgo

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

var _ = Describe("Stream", func() {
	var testClient *Client
	var testURL *url.URL
	var srvMock *mocha.Mocha
	var emptyHeaders []string

	BeforeEach(func() {
		srvMock = mocha.New(GinkgoT())
		srvMock.Start()
		testURL, _ = url.Parse(srvMock.URL())
		testClient = NewClient(testURL)
		testClient.caps = &ServerCapabilities{Extensions: []string{}, ProtocolVersions: []string{"1.0.0"}}
		testClient.protocolVersion = "1.0.0"
		emptyHeaders = []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum"}
	})
	AfterEach(func() {
		if srvMock != nil {
			srvMock.AssertCalled(GinkgoT())
			Ω(srvMock.Close()).Should(Succeed())
		}
	})
	Context("happy path", func() {
		It("should close the response body", func() {
			u := Upload{Location: "/foo/bar", RemoteSize: 1024}
			s, err := NewStream(testClient, u)
			Ω(err).ShouldNot(HaveOccurred())

			tExpectBodyClosed(testClient, http.StatusNoContent, http.Header{"Upload-Offset": {"1024"}}, func() error {
				n, err := s.Write(make([]byte, 1024))
				Ω(n).Should(Equal(1024))
				return err
			})
		})
		Context("NewStream", func() {
			It("should correct set initial values", func() {
				expires := time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 64, Expires: &expires}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				loc, _ := url.Parse(srvMock.URL() + "/foo/bar")
				Ω(*s).Should(Equal(Stream{
					checksumHash:        nil,
					rawChecksumHashName: "",
					client:              testClient,
					location:            loc,
					uploadSize:          1024,
					uploadStateOffset:   64,
					uploadStateExpires:  &expires,
				}))
			})
		})
		DescribeTable("New* should resolve the location relative to the client endpoint",
			func(endpoint, location, expectLocation string) {
				e, err := url.Parse(endpoint)
				Ω(err).ShouldNot(HaveOccurred())
				cl := NewClient(e)
				u := Upload{Location: location, RemoteSize: 1024}

				s, err := NewStream(cl, u)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s.location.String()).Should(Equal(expectLocation))

				s, err = NewStreamWithChecksum(cl, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s.location.String()).Should(Equal(expectLocation))
			},
			Entry("absolute URL", "http://example.com/files/", "http://other.com/foo/bar", "http://other.com/foo/bar"),
			Entry("scheme-relative URL", "http://example.com/files/", "//other.com/foo/bar", "http://other.com/foo/bar"),
			Entry("absolute path", "http://example.com/files/", "/foo/bar", "http://example.com/foo/bar"),
			Entry("relative path, endpoint with trailing slash", "http://example.com/files/", "foo/bar", "http://example.com/files/foo/bar"),
			Entry("relative path, endpoint without trailing slash", "http://example.com/files", "foo/bar", "http://example.com/foo/bar"),
			Entry("relative path with dot segments", "http://example.com/files/sub/", "../foo/./bar", "http://example.com/files/foo/bar"),
			Entry("query is kept", "http://example.com/files/", "foo?token=abc", "http://example.com/files/foo?token=abc"),
		)
		Context("NewStreamWithChecksum", func() {
			DescribeTable("should set checksum algorithm",
				func(algorithm string) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewStreamWithChecksum(testClient, u, algorithm)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.checksumHash).Should(BeAssignableToTypeOf(sha1.New()))
					Ω(s.rawChecksumHashName).Should(Equal(algorithm))
					Ω(s.Size()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(0))
				},
				Entry("normalized name", "sha1"),
				Entry("name to be normalized", "SHA-1"),
			)
		})
		DescribeTable("should upload data in a single request",
			func(dataSize, uploadSize int) {
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: int64(uploadSize)}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(dataSize)))

				Ω(s.Write(data)).Should(Equal(dataSize))
				Ω(s.Size()).Should(BeEquivalentTo(uploadSize))
				Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
				Ω(s.uploadStateExpires).Should(BeNil()) // No Upload-Expires header means the upload does not expire
				Ω(up.requests).Should(HaveLen(1))
				Ω(up.requests[0].ContentLength).Should(BeEquivalentTo(dataSize))
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("data fills the upload", 1024, 1024),
			Entry("data is less than the upload", 100, 1024),
		)
		It("should continue from the current offset on subsequent Write calls", func() {
			replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent())}
			up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

			u := Upload{Location: "/foo/bar", RemoteSize: 1024}
			s, err := NewStream(testClient, u)
			Ω(err).ShouldNot(HaveOccurred())
			data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

			for _, part := range [][]byte{data[:1], data[1:500], data[500:]} {
				Ω(s.Write(part)).Should(Equal(len(part)))
			}
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(3))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		DescribeTable("should return confirmed bytes and ErrIncompleteWrite if server confirms a part of received data",
			func(acceptSize int) {
				replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{acceptSize}}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				// 1024@0 -> acceptSize confirmed
				n, err := s.Write(data)
				Ω(n).Should(Equal(acceptSize))
				Ω(err).Should(MatchError(ErrIncompleteWrite))
				Ω(s.Size()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(acceptSize))
				Ω(up.requests).Should(HaveLen(1))
				Ω(data[:acceptSize]).Should(Equal(up.buf.Bytes()))

				// The rest is sent by the next call
				Ω(s.Write(data[s.Tell():])).Should(Equal(1024 - acceptSize))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(up.requests).Should(HaveLen(2))
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("partially", 100),
			Entry("all except the last byte", 1023),
			Entry("nothing", 0),
		)
		Context("UploadFrom", func() {
			It("should upload data from the reader in a single request", func() {
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				Ω(s.UploadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(1024))
				Ω(s.Size()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(up.requests).Should(HaveLen(1))
				Ω(data).Should(Equal(up.buf.Bytes()))
			})
			DescribeTable("should return bytes read and ErrIncompleteWrite if server confirms a part of received data",
				func(acceptSize int) {
					replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.NoContent())}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{acceptSize}}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

					// 1024@0 -> acceptSize confirmed, the whole reader is consumed
					n, err := s.UploadFrom(bytes.NewReader(data))
					Ω(n).Should(BeEquivalentTo(1024))
					Ω(err).Should(And(
						MatchError(ErrIncompleteWrite),
						MatchError(ContainSubstring("accepted="+strconv.Itoa(acceptSize))),
					))
					Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusNoContent))
					Ω(s.Tell()).Should(BeEquivalentTo(acceptSize))
					Ω(up.requests).Should(HaveLen(1))
					Ω(data[:acceptSize]).Should(Equal(up.buf.Bytes()))

					// The rest is sent by the next call with a new reader
					Ω(s.UploadFrom(bytes.NewReader(data[s.Tell():]))).Should(BeEquivalentTo(1024 - acceptSize))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))
					Ω(up.requests).Should(HaveLen(2))
					Ω(data).Should(Equal(up.buf.Bytes()))
				},
				Entry("partially", 100),
				Entry("all except the last byte", 1023),
				Entry("nothing", 0),
			)
			It("should set checksum of the whole data in request trailer", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum-trailer")
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
				sum := sha1.Sum(data)
				b64sum := base64.StdEncoding.EncodeToString(sum[:])

				Ω(s.UploadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(1024))
				Ω(data).Should(Equal(up.buf.Bytes()))
				Ω(up.requests).Should(HaveLen(1))
				Ω(up.requests[0].Trailer.Get("Upload-Checksum")).Should(Equal("sha1 " + b64sum))
			})
		})
		It("should set checksum of the whole data in request header", func() {
			testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
			replies := []*reply.StdReply{tReply(reply.NoContent())}
			eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
			up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).ReplyFunction(up.handler()))

			u := Upload{Location: "/foo/bar", RemoteSize: 1024}
			s, err := NewStreamWithChecksum(testClient, u, "sha1")
			Ω(err).ShouldNot(HaveOccurred())
			data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
			sum := sha1.Sum(data)
			b64sum := base64.StdEncoding.EncodeToString(sum[:])

			Ω(s.Write(data)).Should(Equal(1024))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(data).Should(Equal(up.buf.Bytes()))
			Ω(up.requests).Should(HaveLen(1))
			Ω(up.requests[0].Header.Get("Upload-Checksum")).Should(Equal("sha1 " + b64sum))
			Ω(up.requests[0].Trailer.Get("Upload-Checksum")).Should(BeEmpty())
		})
		It("should set Expires", func() {
			testClient.caps.Extensions = append(testClient.caps.Extensions, "expiration")
			rpl := tReply(reply.NoContent()).Header("Upload-Expires", "Wed, 25 Jun 2014 16:00:00 GMT")
			up := mockTusUploader{replies: []*reply.StdReply{rpl}, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

			u := Upload{Location: "/foo/bar", RemoteSize: 1024}
			s, err := NewStream(testClient, u)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(s.Write(make([]byte, 1024))).Should(Equal(1024))
			dt := time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)
			Ω(s.uploadStateExpires).ShouldNot(BeNil())
			Ω(dt.Equal(*s.uploadStateExpires)).Should(BeTrue())
		})
		It("should send the request via DoRequest along with ExtraHeaders", func() {
			replies := []*reply.StdReply{tReply(reply.NoContent())}
			up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).
				Header("X-Custom", expect.ToEqual("custom")).
				Header("X-Extra", expect.ToEqual("extra")).
				ReplyFunction(up.handler()))
			spy := &spyClient{}
			testClient.DoRequest = spy.Do
			testClient.ExtraHeaders = http.Header{"X-Extra": {"extra"}}

			u := Upload{Location: "/foo/bar", RemoteSize: 1024}
			s, err := NewStream(testClient, u)
			Ω(err).ShouldNot(HaveOccurred())
			data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

			Ω(s.Write(data)).Should(Equal(1024))
			Ω(data).Should(Equal(up.buf.Bytes()))
			Ω(spy.calls).Should(Equal([]requestCall{{http.MethodPatch, srvMock.URL() + "/foo/bar"}}))
			Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
		})
		Context("DeclareSize", func() {
			It("should send the size without data and keep the offset", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 256))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).
					Header("Upload-Length", expect.ToEqual("1024")).
					Header("Content-Length", expect.ToEqual("0")).
					ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown, RemoteOffset: 256}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s.Size()).Should(BeEquivalentTo(SizeUnknown))

				Ω(s.DeclareSize(1024)).Should(Succeed())
				Ω(s.Size()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(256))
				Ω(up.requests).Should(HaveLen(1))
				Ω(up.buf.Len()).Should(Equal(256))
			})
			It("should not send checksum", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum", "creation-defer-length")
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).
					Header("Content-Length", expect.ToEqual("0")).
					ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
				s, err := NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.DeclareSize(1024)).Should(Succeed())
				Ω(s.Size()).Should(BeEquivalentTo(1024))
			})
			It("should set Expires", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
				rpl := tReply(reply.NoContent()).Header("Upload-Expires", "Wed, 25 Jun 2014 16:00:00 GMT")
				up := mockTusUploader{replies: []*reply.StdReply{rpl}, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).
					Header("Content-Length", expect.ToEqual("0")).
					ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.DeclareSize(1024)).Should(Succeed())
				dt := time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)
				Ω(s.uploadStateExpires).ShouldNot(BeNil())
				Ω(dt.Equal(*s.uploadStateExpires)).Should(BeTrue())
			})
			It("should close the response body", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				tExpectBodyClosed(testClient, http.StatusNoContent, http.Header{"Upload-Offset": {"0"}}, func() error {
					return s.DeclareSize(1024)
				})
			})
		})
		Context("Sync", func() {
			DescribeTable("should sync local offset and size with remote ones and return the offset delta",
				func(localOffset, remoteOffset, expectDelta int64) {
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
					srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).
						Reply(tReply(reply.Status(http.StatusOK)).
							Header("Upload-Offset", strconv.FormatInt(remoteOffset, 10)).
							Header("Upload-Length", "2048")),
					)
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: localOffset}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.Sync()).Should(Equal(expectDelta))
					Ω(s.Size()).Should(BeEquivalentTo(2048))
					Ω(s.Tell()).Should(Equal(remoteOffset))
				},
				Entry("remote offset is ahead", int64(8), int64(512), int64(504)),
				Entry("remote offset is behind", int64(512), int64(8), int64(-504)),
				Entry("offsets are equal", int64(512), int64(512), int64(0)),
			)
			It("should close the response body", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				tExpectBodyClosed(testClient, http.StatusOK, http.Header{"Upload-Offset": {"512"}, "Upload-Length": {"1024"}}, func() error {
					delta, err := s.Sync()
					Ω(delta).Should(BeEquivalentTo(512))
					return err
				})
			})
		})
		Context("Seek", func() {
			DescribeTable("should move local offset",
				func(offset int64, whence int, expectOffset int64) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 100}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.Seek(offset, whence)).Should(Equal(expectOffset))
					Ω(s.Size()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(Equal(expectOffset))
				},
				Entry("io.SeekStart", int64(512), io.SeekStart, int64(512)),
				Entry("io.SeekCurrent", int64(50), io.SeekCurrent, int64(150)),
				Entry("io.SeekEnd", int64(-24), io.SeekEnd, int64(1000)),
				Entry("clamped to upload size", int64(1025), io.SeekStart, int64(1024)),
			)
			DescribeTable("seeking on upload with unknown size without clamping",
				func(offset int64, whence int, expectOffset int64) {
					u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown, RemoteOffset: 100}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.Seek(offset, whence)).Should(Equal(expectOffset))
					Ω(s.Size()).Should(BeEquivalentTo(SizeUnknown))
					Ω(s.Tell()).Should(Equal(expectOffset))
				},
				Entry("io.SeekStart", int64(512), io.SeekStart, int64(512)),
				Entry("io.SeekCurrent", int64(50), io.SeekCurrent, int64(150)),
			)
		})
	})
	Context("error path", func() {
		DescribeTable("should keep the response body in the error and close the original body",
			func(remoteOffset int64, status int, header http.Header, expectErr error) {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: remoteOffset}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
					_, err := s.Write(make([]byte, 512))
					return err
				})
				Ω(s.Tell()).Should(Equal(remoteOffset))
			},
			Entry("http error", int64(0), http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("offset advanced too far", int64(0), http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("offset has gone backwards", int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"100"}}, ErrProtocol),
			Entry("malformed Upload-Offset", int64(0), http.StatusNoContent, http.Header{"Upload-Offset": {"abc"}}, ErrProtocol),
			Entry("malformed Upload-Expires", int64(0), http.StatusNoContent, http.Header{"Upload-Offset": {"512"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
		)
		DescribeTable("http errors handling",
			func(expectStatus int, expectErr error) {
				replies := []*reply.StdReply{tReply(reply.Status(expectStatus))}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				n, err := s.Write(make([]byte, 1024))
				Ω(n).Should(Equal(0))
				Ω(err).Should(MatchError(expectErr))
				Ω(s.Size()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(expectStatus))
				Ω(up.buf.Len()).Should(Equal(0))
			},
			Entry("409", http.StatusConflict, ErrOffsetsNotSynced),
			Entry("403", http.StatusForbidden, ErrCannotUpload),
			Entry("410", http.StatusGone, ErrUploadDoesNotExist),
			Entry("404", http.StatusNotFound, ErrUploadDoesNotExist),
			Entry("413", http.StatusRequestEntityTooLarge, ErrTooLarge),
			Entry("460", 460, ErrUnexpectedResponse),
			Entry("401", http.StatusUnauthorized, ErrUnexpectedResponse),
			Entry("200", http.StatusOK, ErrUnexpectedResponse),
			Entry("201", http.StatusCreated, ErrUnexpectedResponse),
		)
		When("server returned 460 Checksum Mismatch and checksum is used", func() {
			It("should return ErrChecksumMismatch", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
				replies := []*reply.StdReply{tReply(reply.Status(460))}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())

				n, err := s.Write(make([]byte, 1024))
				Ω(n).Should(Equal(0))
				Ω(err).Should(MatchError(ErrChecksumMismatch))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(460))
				Ω(up.buf.Len()).Should(Equal(0))
			})
		})
		Context("DeclareSize", func() {
			It("should return error if creation-defer-length extension is not active", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				err = s.DeclareSize(2048)
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: creation-defer-length")),
				))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
			})
			It("should return error and send nothing if size is negative", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.DeclareSize(-1)).Should(MatchError(ContainSubstring("size is negative")))
				Ω(s.Size()).Should(BeEquivalentTo(SizeUnknown))
			})
			DescribeTable("should keep the response body in the error and keep the size unknown",
				func(status int, header http.Header, expectErr error) {
					testClient.caps.Extensions = append(testClient.caps.Extensions, "creation-defer-length")
					u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						return s.DeclareSize(1024)
					})
					Ω(s.Size()).Should(BeEquivalentTo(SizeUnknown))
					Ω(s.Tell()).Should(BeEquivalentTo(0))
				},
				Entry("http error", http.StatusConflict, nil, ErrOffsetsNotSynced),
				Entry("server has changed the offset", http.StatusNoContent, http.Header{"Upload-Offset": {"100"}}, ErrProtocol),
				Entry("malformed Upload-Expires", http.StatusNoContent, http.Header{"Upload-Offset": {"0"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
			)
		})
		Context("UploadFrom", func() {
			DescribeTable("should keep the response body in the error and close the original body",
				func(remoteOffset int64, status int, header http.Header, expectErr error) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: remoteOffset}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
						_, err := s.UploadFrom(bytes.NewReader(make([]byte, 512)))
						return err
					})
				},
				Entry("http error", int64(0), http.StatusConflict, nil, ErrOffsetsNotSynced),
				Entry("offset has gone backwards", int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"100"}}, ErrProtocol),
				Entry("malformed Upload-Offset", int64(0), http.StatusNoContent, http.Header{"Upload-Offset": {"abc"}}, ErrProtocol),
			)
			DescribeTable("http errors handling",
				func(expectStatus int, expectErr error) {
					replies := []*reply.StdReply{tReply(reply.Status(expectStatus))}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					n, err := s.UploadFrom(bytes.NewReader(make([]byte, 1024)))
					Ω(n).Should(BeEquivalentTo(0))
					Ω(err).Should(MatchError(expectErr))
					Ω(s.Tell()).Should(BeEquivalentTo(0))
					Ω(tErrResponse(err)).Should(HaveHTTPStatus(expectStatus))
					Ω(up.buf.Len()).Should(Equal(0))
				},
				Entry("409", http.StatusConflict, ErrOffsetsNotSynced),
				Entry("403", http.StatusForbidden, ErrCannotUpload),
				Entry("404", http.StatusNotFound, ErrUploadDoesNotExist),
				Entry("413", http.StatusRequestEntityTooLarge, ErrTooLarge),
			)
			It("should return error if checksum-trailer extension is not active", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())

				n, err := s.UploadFrom(bytes.NewReader(make([]byte, 1024)))
				Ω(n).Should(BeEquivalentTo(0))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: checksum-trailer")),
				))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
			})
		})
		Context("Sync", func() {
			It("should return zero delta and keep offset and size if the request fails", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).Reply(tReply(reply.Status(http.StatusNotFound))))
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 8}
				s, err := NewStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				delta, err := s.Sync()
				Ω(err).Should(MatchError(ErrUploadDoesNotExist))
				Ω(delta).Should(BeEquivalentTo(0))
				Ω(s.Size()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(8))
			})
		})
		Context("Seek", func() {
			When("seek to with invalid whence", func() {
				It("should return error and keep the offset", func() {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 100}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s.Seek(-1, 4)
					Ω(err).Should(MatchError(ContainSubstring("unknown whence value: 4")))
					Ω(s.Tell()).Should(BeEquivalentTo(100))
				})
			})
			When("seek on upload with unknown size", func() {
				It("should return error and keep the offset", func() {
					u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown, RemoteOffset: 100}
					s, err := NewStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s.Seek(0, io.SeekEnd)
					Ω(err).Should(MatchError(ContainSubstring("cannot seek from end of upload with unknown size")))
					Ω(s.Tell()).Should(BeEquivalentTo(100))
				})
			})
		})
		When("upload location is malformed", func() {
			It("should return error from constructors", func() {
				u := Upload{Location: ":/foo", RemoteSize: 1024}
				s, err := NewStream(testClient, u)
				Ω(err).Should(MatchError(ContainSubstring("parse upload location")))
				Ω(s).Should(BeNil())

				s, err = NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).Should(MatchError(ContainSubstring("parse upload location")))
				Ω(s).Should(BeNil())
			})
		})
		When("checksum algorithm is not supported", func() {
			It("should return error", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStreamWithChecksum(testClient, u, "unknown")
				Ω(err).Should(MatchError(ContainSubstring("algorithm \"unknown\" is not supported")))
				Ω(s).Should(BeNil())
			})
		})
		When("upload with checksum, but checksum extension is not active", func() {
			It("should return error", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())

				n, err := s.Write(make([]byte, 1024))
				Ω(n).Should(Equal(0))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: checksum")),
				))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
			})
		})
	})
})
