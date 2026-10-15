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

	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vitorsalgado/mocha/v3"
)

type mockTusUploader struct {
	requests []*http.Request
	replies  []*reply.StdReply
	buf      *bytes.Buffer
	// acceptSizes limits the number of body bytes the server confirms in each successful request, consumed in
	// lockstep with replies. A missing or negative value means the whole body is confirmed.
	acceptSizes []int
}

func (mtu *mockTusUploader) handler() func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
	return func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
		if len(mtu.replies) == 0 {
			panic("no more mock replies left")
		}
		mtu.requests = append(mtu.requests, r)
		resp, err := mtu.replies[0].Build(r, m, p)
		if err != nil {
			return resp, err
		}
		mtu.replies = mtu.replies[1:]
		acceptSize := -1
		if len(mtu.acceptSizes) > 0 {
			acceptSize = mtu.acceptSizes[0]
			mtu.acceptSizes = mtu.acceptSizes[1:]
		}
		if resp.Status == http.StatusNoContent {
			if acceptSize < 0 {
				if _, err = io.Copy(mtu.buf, r.Body); err != nil {
					return resp, err
				}
			} else {
				if _, err = io.CopyN(mtu.buf, r.Body, int64(acceptSize)); err != nil && err != io.EOF {
					return resp, err
				}
				if _, err = io.Copy(io.Discard, r.Body); err != nil {
					return resp, err
				}
			}
			resp.Header["Upload-Offset"] = []string{strconv.Itoa(mtu.buf.Len())}
		}
		return resp, nil
	}
}

func (mtu *mockTusUploader) makeRequest(method, location string, emptyHeaders []string) *mocha.MockBuilder {
	b := mocha.Request().
		URL(expect.URLPath(location)).Method(method).
		Header("Tus-Resumable", expect.ToEqual("1.0.0")).
		Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
		Header("Upload-Offset", expect.Func(func(v any, a expect.Args) (bool, error) {
			num, e := strconv.Atoi(v.(string))
			return num >= 0, e
		})).
		Header("Upload-Offset", expect.Func(func(v any, a expect.Args) (bool, error) {
			num, e := strconv.Atoi(v.(string))
			return num == mtu.buf.Len(), e
		}))
	for _, h := range emptyHeaders {
		b = b.Header(h, expect.ToBeEmpty())
	}
	return b
}

// tReadFrom transfers data to the stream using ReadFrom
func tReadFrom(s *UploadStream, data []byte) (int64, error) { return s.ReadFrom(bytes.NewReader(data)) }

// tWrite transfers data to the stream using Write
func tWrite(s *UploadStream, data []byte) (int64, error) {
	n, err := s.Write(data)
	return int64(n), err
}

var _ = Describe("UploadStream", func() {
	var testClient *Client
	var testURL *url.URL
	var srvMock *mocha.Mocha
	var emptyHeaders []string

	BeforeEach(func() {
		srvMock = mocha.New(GinkgoT())
		srvMock.Start()
		testURL, _ = url.Parse(srvMock.URL())
		testClient = NewClient(testURL)
		testClient.advertisedExtensions = []string{}
		emptyHeaders = []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum"}
	})
	AfterEach(func() {
		if srvMock != nil {
			srvMock.AssertCalled(GinkgoT())
			Ω(srvMock.Close()).Should(Succeed())
		}
	})
	Context("happy path", func() {
		DescribeTable("should close the response body",
			func(copyCb func(s *UploadStream, data []byte) (int64, error), chunkSize int64) {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = chunkSize

				tExpectBodyClosed(testClient, http.StatusNoContent, http.Header{"Upload-Offset": {"1024"}}, func() error {
					n, err := copyCb(s, make([]byte, 1024))
					Ω(n).Should(BeEquivalentTo(1024))
					return err
				})
			},
			Entry("ReadFrom, chunked", tReadFrom, int64(1024)),
			Entry("Write, chunked", tWrite, int64(1024)),
			Entry("ReadFrom, no chunked", tReadFrom, int64(NoChunked)),
			Entry("Write, no chunked", tWrite, int64(NoChunked)),
		)
		Context("NewUploadStream", func() {
			It("should correct set initial values", func() {
				expires := time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 64, UploadExpired: &expires}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				loc, _ := url.Parse(srvMock.URL() + "/foo/bar")
				Ω(*s).Should(Equal(UploadStream{
					ChunkSize:           2 * 1024 * 1024,
					SetUploadSize:       false,
					checksumHash:        nil,
					rawChecksumHashName: "",
					client:              testClient,
					dirtyBuffer:         nil,
					dirtyUnread:         nil,
					location:            loc,
					uploadSize:          1024,
					uploadStateOffset:   64,
					uploadStateExpires:  &expires,
				}))
			})
			It("should not change the given Upload while transferring data", func() {
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.Write(make([]byte, 1024))).Should(Equal(1024))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(u).Should(Equal(Upload{Location: "/foo/bar", RemoteSize: 1024}))
			})
			It("should resolve the location relative to the client endpoint", func() {
				u := Upload{Location: "http://example.com/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.location.String()).Should(Equal("http://example.com/foo/bar"))
			})
		})
		Context("NewUploadStreamWithChecksum", func() {
			DescribeTable("should set checksum algorithm",
				func(algorithm string) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStreamWithChecksum(testClient, u, algorithm)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.checksumHash).Should(BeAssignableToTypeOf(sha1.New()))
					Ω(s.rawChecksumHashName).Should(Equal(algorithm))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(0))
				},
				Entry("normalized name", "sha1"),
				Entry("name to be normalized", "SHA-1"),
			)
		})
		DescribeTable("ordinary upload data without interrupts or errors",
			func(copyCb func(s *UploadStream, data []byte) (int64, error), dataSize, uploadSize int) {
				replies := []*reply.StdReply{
					tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: int64(uploadSize)}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = 256
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(dataSize)))

				Ω(copyCb(s, data)).Should(BeEquivalentTo(dataSize))
				Ω(s.Len()).Should(BeEquivalentTo(uploadSize))
				Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
				Ω(s.uploadStateExpires).Should(BeNil()) // No Upload-Expires header means the upload does not expire
				Ω(s.Dirty()).Should(BeFalse())
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("ReadFrom data aligned", tReadFrom, 1024, 1024),
			Entry("Write data aligned", tWrite, 1024, 1024),
			Entry("ReadFrom data unaligned", tReadFrom, 1000, 1024),
			Entry("Write data unaligned", tWrite, 1000, 1024),
			Entry("ReadFrom data less than chunk size", tReadFrom, 100, 1024),
			Entry("Write data less than chunk size", tWrite, 100, 1024),
			Entry("ReadFrom data equal than chunk size", tReadFrom, 256, 1024),
			Entry("Write data equal than chunk size", tWrite, 256, 1024),
			Entry("ReadFrom data and upload less than chunk size", tReadFrom, 100, 100),
			Entry("Write data and upload less than chunk size", tWrite, 100, 100),
		)
		When("reader passed to ReadFrom is empty and offset is not 0", func() {
			It("should do nothing and keep offset the same", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 64}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = 256
				data := make([]byte, 0)
				rd := bytes.NewReader(data)

				Ω(s.ReadFrom(rd)).Should(BeEquivalentTo(0))
				Ω(s.Dirty()).Should(BeFalse())
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(64))
			})
		})
		Context("retry to upload data after error", func() {
			When("ReadFrom, error in the middle", func() {
				It("retrying should work correctly", func() {
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), reply.InternalServerError(), tReply(reply.NoContent()), tReply(reply.NoContent()),
					}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = 256
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
					rd := bytes.NewReader(data)

					// First attempt before error
					copied, err := s.ReadFrom(rd)
					Ω(err).Should(MatchError(ErrUnexpectedResponse))
					Ω(copied).Should(BeEquivalentTo(768))
					Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusInternalServerError))
					Ω(s.Dirty()).Should(BeTrue())
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(512))

					// Second attempt after error
					Ω(s.ReadFrom(rd)).Should(BeEquivalentTo(256))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))

					Ω(data).Should(Equal(up.buf.Bytes()))
				})
			})
			When("ReadFrom, error at the end, data is not aligned", func() {
				It("retrying should work correctly", func() {
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), reply.InternalServerError(), tReply(reply.NoContent()),
					}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = 256
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1000))
					rd := bytes.NewReader(data)

					// First attempt before error
					copied, err := s.ReadFrom(rd)
					Ω(err).Should(MatchError(ErrUnexpectedResponse))
					Ω(copied).Should(BeEquivalentTo(1000))
					Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusInternalServerError))
					Ω(s.Dirty()).Should(BeTrue())
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(768))

					// Second attempt after error
					Ω(s.ReadFrom(rd)).Should(BeEquivalentTo(0))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1000))

					Ω(data).Should(Equal(up.buf.Bytes()))
				})
			})
			When("Write, error in the middle", func() {
				It("retrying should work correctly", func() {
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), reply.InternalServerError(), tReply(reply.NoContent()), tReply(reply.NoContent()),
					}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = 256
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

					// First attempt before error
					copied, err := s.Write(data)
					Ω(err).Should(MatchError(ErrUnexpectedResponse))
					Ω(copied).Should(Equal(512))
					Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusInternalServerError))
					Ω(s.Dirty()).Should(BeFalse()) // Write does not leave stream in dirty state
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(512))

					// Second attempt after error
					Ω(s.Write(data[512:])).Should(Equal(512))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))

					Ω(data).Should(Equal(up.buf.Bytes()))
				})
			})
		})
		Context("data to be uploaded is oversize", func() {
			When("ReadFrom, chunked mode", func() {
				DescribeTable("should read only bytes left at remote",
					func(remoteSize int64) {
						replies := []*reply.StdReply{
							tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
						}
						up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
						srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

						u := Upload{Location: "/foo/bar", RemoteSize: remoteSize, RemoteOffset: 256}
						s, err := NewUploadStream(testClient, u)
						Ω(err).ShouldNot(HaveOccurred())
						s.ChunkSize = 256
						data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 2048))
						buf := bytes.NewBuffer(data)
						Ω(io.CopyN(up.buf, buf, 256)).Should(BeEquivalentTo(256)) // Prefill, Upload-Offset now is 256

						Ω(s.ReadFrom(buf)).Should(BeEquivalentTo(remoteSize - 256))
						Ω(s.Len()).Should(BeEquivalentTo(remoteSize))
						Ω(s.Tell()).Should(BeEquivalentTo(remoteSize))
						Ω(data[:remoteSize]).Should(Equal(up.buf.Bytes()))
						Ω(buf.Len()).Should(BeEquivalentTo(2048 - int(remoteSize))) // bytes has not been read from buf
						Ω(s.Dirty()).Should(BeFalse())
					},
					Entry("remote size aligned to chunk size", int64(1024)),
					Entry("remote size unaligned to chunk size", int64(1000)),
				)
			})
			When("ReadFrom, non-chunked mode", func() {
				It("should read only bytes left at remote", func() {
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
					}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1000, RemoteOffset: 256}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = NoChunked
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 2048))
					buf := bytes.NewBuffer(data)
					Ω(io.CopyN(up.buf, buf, 256)).Should(BeEquivalentTo(256)) // Prefill, Upload-Offset now is 256

					Ω(s.ReadFrom(buf)).Should(BeEquivalentTo(1000 - 256))
					Ω(s.Len()).Should(BeEquivalentTo(1000))
					Ω(s.Tell()).Should(BeEquivalentTo(1000))
					Ω(data[:1000]).Should(Equal(up.buf.Bytes()))
					Ω(buf.Len()).Should(BeEquivalentTo(2048 - 1000)) // bytes has not been read from buf
					Ω(s.Dirty()).Should(BeFalse())
				})
			})
			When("Write, chunked mode", func() {
				DescribeTable("should read only bytes left at remote and return ErrShortWrite",
					func(remoteSize int64) {
						replies := []*reply.StdReply{
							tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
						}
						up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
						srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

						u := Upload{Location: "/foo/bar", RemoteSize: remoteSize, RemoteOffset: 256}
						s, err := NewUploadStream(testClient, u)
						Ω(err).ShouldNot(HaveOccurred())
						s.ChunkSize = 256
						data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 2048))
						buf := bytes.NewBuffer(data)
						Ω(io.CopyN(up.buf, buf, 256)).Should(BeEquivalentTo(256)) // Prefill, Upload-Offset now is 256

						n, err := s.Write(buf.Bytes())
						Ω(n).Should(BeEquivalentTo(remoteSize - 256))
						Ω(err).Should(MatchError(io.ErrShortWrite))
						Ω(s.Len()).Should(BeEquivalentTo(remoteSize))
						Ω(s.Tell()).Should(BeEquivalentTo(remoteSize))
						Ω(data[:remoteSize]).Should(Equal(up.buf.Bytes()))
						Ω(s.Dirty()).Should(BeFalse())
					},
					Entry("remote size aligned to chunk size", int64(1024)),
					Entry("remote size unaligned to chunk size", int64(1000)),
				)
			})
			When("Write, non-chunked mode", func() {
				It("should read only bytes left at remote and return ErrShortWrite", func() {
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
					}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1000, RemoteOffset: 256}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = NoChunked
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 2048))
					buf := bytes.NewBuffer(data)
					Ω(io.CopyN(up.buf, buf, 256)).Should(BeEquivalentTo(256)) // Prefill, Upload-Offset now is 256

					n, err := s.Write(buf.Bytes())
					Ω(n).Should(BeEquivalentTo(1000 - 256))
					Ω(err).Should(MatchError(io.ErrShortWrite))
					Ω(s.Len()).Should(BeEquivalentTo(1000))
					Ω(s.Tell()).Should(BeEquivalentTo(1000))
					Ω(data[:1000]).Should(Equal(up.buf.Bytes()))
					Ω(s.Dirty()).Should(BeFalse())
				})
			})
		})
		Context("server confirms only a part of received data in some requests", func() {
			When("Chunked mode", func() {
				DescribeTable("should retry rest of data",
					func(copyCb func(s *UploadStream, data []byte) (int64, error), dataSize int, acceptSizes []int, expectRequests int) {
						replies := make([]*reply.StdReply, expectRequests)
						for i := range replies {
							replies[i] = tReply(reply.NoContent())
						}
						up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: acceptSizes}
						srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

						u := Upload{Location: "/foo/bar", RemoteSize: 1024}
						s, err := NewUploadStream(testClient, u)
						Ω(err).ShouldNot(HaveOccurred())
						s.ChunkSize = 256
						data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(dataSize)))

						Ω(copyCb(s, data)).Should(BeEquivalentTo(dataSize))
						Ω(s.Len()).Should(BeEquivalentTo(1024))
						Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
						Ω(s.Dirty()).Should(BeFalse())
						Ω(up.requests).Should(HaveLen(expectRequests))
						Ω(data).Should(Equal(up.buf.Bytes()))
					},
					// 256@0 -> 100 confirmed, 156@100, then 3 full chunks
					Entry("ReadFrom, first chunk confirmed partially", tReadFrom, 1024, []int{100}, 5),
					Entry("Write, first chunk confirmed partially", tWrite, 1024, []int{100}, 5),
					// 256@0 -> 255 confirmed, 1@255, then 3 full chunks
					Entry("ReadFrom, chunk confirmed except the last byte", tReadFrom, 1024, []int{255}, 5),
					Entry("Write, chunk confirmed except the last byte", tWrite, 1024, []int{255}, 5),
					// Every 256 bytes chunk is transferred by 3 requests confirming 100, 100 and 56 bytes
					Entry("ReadFrom, every request confirmed partially", tReadFrom, 1024, []int{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}, 12),
					Entry("Write, every request confirmed partially", tWrite, 1024, []int{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}, 12),
					// 3 full chunks, then 232@768 -> 100 confirmed, 132@868
					Entry("ReadFrom, last unaligned chunk confirmed partially", tReadFrom, 1000, []int{-1, -1, -1, 100}, 5),
					Entry("Write, last unaligned chunk confirmed partially", tWrite, 1000, []int{-1, -1, -1, 100}, 5),
				)
			})
			When("Non-chunked mode", func() {
				It("ReadFrom should not retry and should return an error after the first request", func() {
					replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.NoContent())}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{100}}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = NoChunked
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
					rd := bytes.NewReader(data)

					// 1024@0 -> 100 confirmed, 924@100
					n, err := s.ReadFrom(rd)
					Ω(n).Should(BeEquivalentTo(1024))
					Ω(err).Should(MatchError(io.ErrShortWrite))
					Ω(rd.Len()).Should(Equal(0))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(100))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(up.requests).Should(HaveLen(1))
					Ω(data[:100]).Should(Equal(up.buf.Bytes()))
				})
				It("Write should ignore the unconfirmed rest of data and return confirmed bytes", func() {
					replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.NoContent())}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{100}}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = NoChunked
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

					// 1024@0 -> 100 confirmed, 924@100
					Ω(s.Write(data)).Should(BeEquivalentTo(100))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(100))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(up.requests).Should(HaveLen(1))
					Ω(data[:100]).Should(Equal(up.buf.Bytes()))
				})
			})
		})
		DescribeTable("upload data no chunked",
			func(copyCb func(s *UploadStream, data []byte) (int64, error)) {
				replies := []*reply.StdReply{tReply(reply.NoContent())}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = NoChunked
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(s.Dirty()).Should(BeFalse())
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("ReadFrom", tReadFrom),
			Entry("Write", tWrite),
		)
		DescribeTable("upload data with defer length",
			func(copyCb func(s *UploadStream, data []byte) (int64, error)) {
				testClient.advertisedExtensions = append(testClient.advertisedExtensions, "creation-defer-length")
				replies := []*reply.StdReply{
					tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Metadata", "Upload-Checksum"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).
					ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = 256
				s.SetUploadSize = true
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(s.Dirty()).Should(BeFalse())
				Ω(data).Should(Equal(up.buf.Bytes()))
				Ω(up.requests[0].Header.Get("Upload-Length")).Should(Equal("1024"))
				for _, v := range up.requests[1:] {
					Ω(v.Header.Get("Upload-Length")).Should(BeEmpty())
				}
			},
			Entry("ReadFrom", tReadFrom),
			Entry("Write", tWrite),
		)
		Context("upload data by chunks with checksum", func() {
			DescribeTable("should set checksum in request header",
				func(copyCb func(s *UploadStream, data []byte) (int64, error)) {
					testClient.advertisedExtensions = append(testClient.advertisedExtensions, "checksum")
					replies := []*reply.StdReply{
						tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
					}
					eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStreamWithChecksum(testClient, u, "sha1")
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = 256
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

					Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(data).Should(Equal(up.buf.Bytes()))
					for i, r := range up.requests {
						sum := sha1.Sum(data[i*256 : i*256+256])
						b64sum := base64.StdEncoding.EncodeToString(sum[:])
						Ω(r.Header.Get("Upload-Checksum")).Should(Equal("sha1 " + b64sum))
					}
				},
				Entry("ReadFrom", tReadFrom),
				Entry("Write", tWrite),
			)
		})
		Context("upload data no chunked with checksum", func() {
			DescribeTable("should upload in one shot and set checksum in request trailer",
				func(copyCb func(s *UploadStream, data []byte) (int64, error)) {
					testClient.advertisedExtensions = append(testClient.advertisedExtensions, "checksum", "checksum-trailer")
					replies := []*reply.StdReply{tReply(reply.NoContent())}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStreamWithChecksum(testClient, u, "sha1")
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = NoChunked
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
					sum := sha1.Sum(data)
					b64sum := base64.StdEncoding.EncodeToString(sum[:])

					Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))
					Ω(s.Dirty()).Should(BeFalse())
					Ω(data).Should(Equal(up.buf.Bytes()))
					Ω(up.requests[0].Trailer.Get("Upload-Checksum")).Should(Equal("sha1 " + b64sum))
				},
				Entry("ReadFrom", tReadFrom),
				Entry("Write", tWrite),
			)
		})
		Context("expired upload", func() {
			DescribeTable("should set UploadExpired",
				func(copyCb func(s *UploadStream, data []byte) (int64, error)) {
					testClient.advertisedExtensions = append(testClient.advertisedExtensions, "expiration")
					rpl := tReply(reply.NoContent()).Header("Upload-Expires", "Wed, 25 Jun 2014 16:00:00 GMT")
					replies := []*reply.StdReply{rpl, rpl, rpl, rpl}
					up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
					srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

					u := Upload{Location: "/foo/bar", RemoteSize: 1024}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())
					s.ChunkSize = 256
					data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

					Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
					dt := time.Date(2014, 6, 25, 16, 0, 0, 0, time.UTC)
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(1024))
					Ω(s.uploadStateExpires).ShouldNot(BeNil())
					Ω(dt.Equal(*s.uploadStateExpires)).Should(BeTrue())
					Ω(s.Dirty()).Should(BeFalse())
					Ω(data).Should(Equal(up.buf.Bytes()))
				},
				Entry("ReadFrom", tReadFrom),
				Entry("Write", tWrite),
			)
		})
		DescribeTable("should send the request via DoRequest along with ExtraHeaders",
			func(copyCb func(s *UploadStream, data []byte) (int64, error), chunkSize int64, expectRequests int) {
				replies := []*reply.StdReply{
					tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					ReplyFunction(up.handler()))
				spy := &doRequestSpy{}
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = http.Header{"X-Extra": {"extra"}}

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = chunkSize
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(data).Should(Equal(up.buf.Bytes()))
				expectCalls := make([]doRequestCall, expectRequests)
				for i := range expectCalls {
					expectCalls[i] = doRequestCall{http.MethodPatch, srvMock.URL() + "/foo/bar"}
				}
				Ω(spy.calls).Should(Equal(expectCalls))
			},
			Entry("ReadFrom, chunked", tReadFrom, int64(256), 4),
			Entry("Write, chunked", tWrite, int64(256), 4),
			Entry("ReadFrom, no chunked", tReadFrom, int64(NoChunked), 1),
			Entry("Write, no chunked", tWrite, int64(NoChunked), 1),
		)
		DescribeTable("should send ExtraHeaders and leave them untouched",
			func(copyCb func(s *UploadStream, data []byte) (int64, error), chunkSize int64, expectRequests int) {
				replies := []*reply.StdReply{
					tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).
					Header("X-Extra", expect.ToEqual("extra")).
					ReplyFunction(up.handler()))
				testClient.ExtraHeaders = http.Header{"X-Extra": {"extra"}}

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = chunkSize
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				Ω(copyCb(s, data)).Should(BeEquivalentTo(1024))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(data).Should(Equal(up.buf.Bytes()))
				Ω(up.requests).Should(HaveLen(expectRequests))
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
			},
			Entry("ReadFrom, chunked", tReadFrom, int64(256), 4),
			Entry("Write, chunked", tWrite, int64(256), 4),
			Entry("ReadFrom, no chunked", tReadFrom, int64(NoChunked), 1),
			Entry("Write, no chunked", tWrite, int64(NoChunked), 1),
		)
		Context("Sync", func() {
			It("should close the response body", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				tExpectBodyClosed(testClient, http.StatusOK, http.Header{"Upload-Offset": {"512"}}, s.Sync)
			})
			It("should send the request via DoRequest along with ExtraHeaders", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).
					Header("X-Custom", expect.ToEqual("custom")).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.Status(http.StatusOK)).Header("Upload-Offset", "512")),
				)
				spy := &doRequestSpy{}
				testClient.DoRequest = spy.DoRequest
				testClient.ExtraHeaders = http.Header{"X-Extra": {"extra"}}
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 8}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.Sync()).Should(Succeed())
				Ω(spy.calls).Should(Equal([]doRequestCall{{http.MethodHead, srvMock.URL() + "/foo/bar"}}))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(512))
			})
			It("should send ExtraHeaders and leave them untouched", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).
					Header("X-Extra", expect.ToEqual("extra")).
					Reply(tReply(reply.Status(http.StatusOK)).Header("Upload-Offset", "512")),
				)
				testClient.ExtraHeaders = http.Header{"X-Extra": {"extra"}}
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 8}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.Sync()).Should(Succeed())
				Ω(testClient.ExtraHeaders).Should(Equal(http.Header{"X-Extra": {"extra"}}))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(512))
			})
			It("should sync local offset with remote offset", func() {
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
				srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).
					Reply(tReply(reply.Status(http.StatusOK)).Header("Upload-Offset", "512")),
				)
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 8}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s.Sync()).Should(Succeed())
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(512))
				Ω(s.Dirty()).Should(BeFalse())
			})
		})
		Context("Seek", func() {
			DescribeTable("should move local offset",
				func(initialOffset, offset int64, whence int, expectOffset int64) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: initialOffset}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.Seek(offset, whence)).Should(Equal(expectOffset))
					Ω(s.Len()).Should(BeEquivalentTo(1024))
					Ω(s.Tell()).Should(BeEquivalentTo(expectOffset))
					Ω(s.Tell()).Should(Equal(expectOffset))
				},
				Entry("io.SeekStart", int64(100), int64(512), io.SeekStart, int64(512)),
				Entry("io.SeekStart to zero", int64(100), int64(0), io.SeekStart, int64(0)),
				Entry("io.SeekStart to the end", int64(100), int64(1024), io.SeekStart, int64(1024)),
				Entry("io.SeekCurrent forward", int64(100), int64(50), io.SeekCurrent, int64(150)),
				Entry("io.SeekCurrent backward", int64(100), int64(-50), io.SeekCurrent, int64(50)),
				Entry("io.SeekCurrent zero offset", int64(100), int64(0), io.SeekCurrent, int64(100)),
				Entry("io.SeekCurrent to zero", int64(100), int64(-100), io.SeekCurrent, int64(0)),
				Entry("io.SeekEnd zero offset", int64(100), int64(0), io.SeekEnd, int64(1024)),
				Entry("io.SeekEnd backward", int64(100), int64(-24), io.SeekEnd, int64(1000)),
				Entry("io.SeekEnd to zero", int64(100), int64(-1024), io.SeekEnd, int64(0)),
			)
			DescribeTable("should clamp offset to upload size",
				func(initialOffset, offset int64, whence int) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: initialOffset}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(s.Seek(offset, whence)).Should(Equal(int64(1024)))
					Ω(s.Tell()).Should(Equal(int64(1024)))
				},
				Entry("io.SeekStart", int64(100), int64(1025), io.SeekStart),
				Entry("io.SeekCurrent", int64(1000), int64(100), io.SeekCurrent),
				Entry("io.SeekEnd", int64(100), int64(1), io.SeekEnd),
			)
			It("should seek in an empty upload", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 0}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s.Seek(10, io.SeekStart)).Should(Equal(int64(0)))
				Ω(s.Tell()).Should(Equal(int64(0)))
			})
		})
	})
	Context("error path", func() {
		DescribeTable("should keep the response body in the error and close the original body",
			func(copyCb func(s *UploadStream, data []byte) (int64, error), chunkSize int64, status int, header http.Header, expectErr error) {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = chunkSize

				tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
					_, err := copyCb(s, make([]byte, 1024))
					return err
				})
			},
			Entry("ReadFrom, chunked, http error", tReadFrom, int64(256), http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("Write, chunked, http error", tWrite, int64(256), http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("ReadFrom, no chunked, http error", tReadFrom, int64(NoChunked), http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("Write, no chunked, http error", tWrite, int64(NoChunked), http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("ReadFrom, chunked, zero progress", tReadFrom, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"0"}}, ErrZeroProgress),
			Entry("Write, chunked, zero progress", tWrite, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"0"}}, ErrZeroProgress),
			Entry("ReadFrom, no chunked, zero progress", tReadFrom, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"0"}}, ErrZeroProgress),
			Entry("Write, no chunked, zero progress", tWrite, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"0"}}, ErrZeroProgress),
			Entry("ReadFrom, chunked, offset advanced too far", tReadFrom, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("Write, chunked, offset advanced too far", tWrite, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("ReadFrom, no chunked, offset advanced too far", tReadFrom, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("Write, no chunked, offset advanced too far", tWrite, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("ReadFrom, chunked, malformed Upload-Expires", tReadFrom, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"256"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
			Entry("Write, chunked, malformed Upload-Expires", tWrite, int64(256), http.StatusNoContent, http.Header{"Upload-Offset": {"256"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
			Entry("ReadFrom, no chunked, malformed Upload-Expires", tReadFrom, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"1024"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
			Entry("Write, no chunked, malformed Upload-Expires", tWrite, int64(NoChunked), http.StatusNoContent, http.Header{"Upload-Offset": {"1024"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
		)
		DescribeTable("http errors handling",
			func(expectStatus int, expectErr error) {
				replies := []*reply.StdReply{tReply(reply.Status(expectStatus))}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = 256
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				n, err := s.ReadFrom(bytes.NewReader(data))
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(MatchError(expectErr))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(expectStatus))
				Ω(s.Dirty()).Should(BeTrue())
				Ω(up.buf.Len()).Should(Equal(0))
			},
			Entry("409", http.StatusConflict, ErrOffsetsNotSynced),
			Entry("403", http.StatusForbidden, ErrCannotUpload),
			Entry("410", http.StatusGone, ErrUploadDoesNotExist),
			Entry("404", http.StatusNotFound, ErrUploadDoesNotExist),
			Entry("413", http.StatusRequestEntityTooLarge, ErrUploadTooLarge),
			Entry("460", 460, ErrUnexpectedResponse),
			Entry("401", http.StatusUnauthorized, ErrUnexpectedResponse),
			Entry("200", http.StatusOK, ErrUnexpectedResponse),
		)
		When("server returned 460 Checksum Mismatch and checksum is used", func() {
			It("should return ErrChecksumMismatch", func() {
				testClient.advertisedExtensions = append(testClient.advertisedExtensions, "checksum")
				replies := []*reply.StdReply{tReply(reply.Status(460))}
				up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
				srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", eh).ReplyFunction(up.handler()))

				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = 256
				data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

				n, err := s.ReadFrom(bytes.NewReader(data))
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(MatchError(ErrChecksumMismatch))
				Ω(s.Len()).Should(BeEquivalentTo(1024))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(460))
				Ω(s.Dirty()).Should(BeTrue())
				Ω(up.buf.Len()).Should(Equal(0))
			})
		})
		Context("server responds successfully but does not advance the offset", func() {
			When("Chunked mode", func() {
				DescribeTable("should return ErrZeroProgress",
					func(copyCb func(s *UploadStream, data []byte) (int64, error), acceptSizes []int, expectN int64, expectOffset int64, expectDirty bool) {
						replies := make([]*reply.StdReply, len(acceptSizes))
						for i := range replies {
							replies[i] = tReply(reply.NoContent())
						}
						up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: acceptSizes}
						srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

						u := Upload{Location: "/foo/bar", RemoteSize: 1024}
						s, err := NewUploadStream(testClient, u)
						Ω(err).ShouldNot(HaveOccurred())
						s.ChunkSize = 256
						data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))

						n, err := copyCb(s, data)
						Ω(n).Should(Equal(expectN))
						Ω(err).Should(And(
							MatchError(ErrZeroProgress),
							MatchError(ContainSubstring("offset="+strconv.FormatInt(expectOffset, 10))),
						))
						Ω(s.Len()).Should(BeEquivalentTo(1024))
						Ω(s.Tell()).Should(BeEquivalentTo(expectOffset))
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusNoContent))
						Ω(s.Dirty()).Should(Equal(expectDirty))
						Ω(up.requests).Should(HaveLen(len(acceptSizes))) // No retries after zero progress
						Ω(data[:expectOffset]).Should(Equal(up.buf.Bytes()))
					},
					// ReadFrom returns the bytes read from reader, the unconfirmed chunk is kept in the dirty buffer
					Entry("ReadFrom, first request", tReadFrom, []int{0}, int64(256), int64(0), true),
					Entry("ReadFrom, after the first chunk", tReadFrom, []int{-1, 0}, int64(512), int64(256), true),
					Entry("ReadFrom, after the partially confirmed chunk", tReadFrom, []int{100, 0}, int64(256), int64(100), true),
					// Write returns the bytes confirmed by server and always leaves the stream clean
					Entry("Write, first request", tWrite, []int{0}, int64(0), int64(0), false),
					Entry("Write, after the first chunk", tWrite, []int{-1, 0}, int64(256), int64(256), false),
					Entry("Write, after the partially confirmed chunk", tWrite, []int{100, 0}, int64(100), int64(100), false),
				)
			})
			When("Non-chunked mode", func() {
				DescribeTable("should return ErrZeroProgress",
					func(copyCb func(s *UploadStream, data []byte) (int64, error), expectN int64) {
						replies := []*reply.StdReply{tReply(reply.NoContent())}
						up := mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{0}}
						srvMock.AddMocks(up.makeRequest(http.MethodPatch, "/foo/bar", emptyHeaders).ReplyFunction(up.handler()))

						u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 256}
						s, err := NewUploadStream(testClient, u)
						Ω(err).ShouldNot(HaveOccurred())
						s.ChunkSize = NoChunked
						data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024))
						Ω(up.buf.Write(data[:256])).Should(Equal(256)) // Prefill, Upload-Offset now is 256

						n, err := copyCb(s, data[256:])
						Ω(n).Should(Equal(expectN))
						Ω(err).Should(And(
							MatchError(ErrZeroProgress),
							MatchError(ContainSubstring("offset=256")),
						))
						Ω(s.Len()).Should(BeEquivalentTo(1024))
						Ω(s.Tell()).Should(BeEquivalentTo(256))
						Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusNoContent))
						Ω(s.Dirty()).Should(BeFalse())
						Ω(up.requests).Should(HaveLen(1))
						Ω(data[:256]).Should(Equal(up.buf.Bytes()))
					},
					// ReadFrom returns the bytes read from reader, Write returns the bytes confirmed by server
					Entry("ReadFrom", tReadFrom, int64(768)),
					Entry("Write", tWrite, int64(0)),
				)
			})
		})
		When("upload location is malformed", func() {
			It("should return error from constructors", func() {
				u := Upload{Location: ":/foo", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).Should(MatchError(ContainSubstring("parse upload location")))
				Ω(s).Should(BeNil())

				s, err = NewUploadStreamWithChecksum(testClient, u, "sha1")
				Ω(err).Should(MatchError(ContainSubstring("parse upload location")))
				Ω(s).Should(BeNil())
			})
		})
		When("checksum algorithm is not supported", func() {
			It("should return error", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStreamWithChecksum(testClient, u, "unknown")
				Ω(err).Should(MatchError(ContainSubstring("algorithm \"unknown\" is not supported")))
				Ω(s).Should(BeNil())
			})
		})
		When("upload size is unknown", func() {
			It("should panic", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				rd := io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024)
				_, err = s.ReadFrom(rd)
				Ω(err).Should(MatchError("upload size must be set before uploading starts"))
			})
		})
		When("upload with defer length, but creation-defer-length extension is not active", func() {
			It("should return error", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())
				s.SetUploadSize = true
				rd := io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024)
				n, err := s.ReadFrom(rd)
				Ω(n).Should(BeEquivalentTo(0))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: creation-defer-length")),
				))
			})
		})
		When("upload with checksum, but checksum extension is not active", func() {
			It("should return error", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				rd := io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024)
				n, err := s.ReadFrom(rd)
				Ω(n).Should(BeEquivalentTo(0))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: checksum")),
				))
			})
		})
		When("upload with checksum and no chunked, but checksum-trailer extension is not active", func() {
			It("should return error", func() {
				testClient.advertisedExtensions = append(testClient.advertisedExtensions, "checksum")
				u := Upload{Location: "/foo/bar", RemoteSize: 1024}
				s, err := NewUploadStreamWithChecksum(testClient, u, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				s.ChunkSize = NoChunked
				rd := io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), 1024)
				n, err := s.ReadFrom(rd)
				Ω(n).Should(BeEquivalentTo(0))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: checksum-trailer")),
				))
			})
		})
		Context("Seek", func() {
			DescribeTable("should return error and keep offset if resulting offset is negative",
				func(offset int64, whence int) {
					u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 100}
					s, err := NewUploadStream(testClient, u)
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s.Seek(offset, whence)
					Ω(err).Should(MatchError(ContainSubstring("is negative")))
					Ω(s.Tell()).Should(Equal(int64(100)))
				},
				Entry("io.SeekStart", int64(-1), io.SeekStart),
				Entry("io.SeekCurrent", int64(-101), io.SeekCurrent),
				Entry("io.SeekEnd", int64(-1025), io.SeekEnd),
			)
			It("should return error and keep offset if upload size is unknown", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: SizeUnknown, RemoteOffset: 100}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				n, err := s.Seek(10, io.SeekStart)
				Ω(err).Should(MatchError(ContainSubstring("unknown size")))
				Ω(n).Should(Equal(int64(100)))
				Ω(s.Tell()).Should(Equal(int64(100)))
			})
			It("should panic on unknown whence", func() {
				u := Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 100}
				s, err := NewUploadStream(testClient, u)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(func() { _, _ = s.Seek(10, 42) }).Should(PanicWith("unknown whence value: 42"))
				Ω(s.Tell()).Should(Equal(int64(100)))
			})
		})
	})
})
