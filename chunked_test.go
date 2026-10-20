package tusgo

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"testing/iotest"
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

func (mtu *mockTusUploader) makePatchRequest(location string, emptyHeaders []string) *mocha.MockBuilder {
	b := mocha.Request().
		URL(expect.URLPath(location)).Method(http.MethodPatch).
		Header("Tus-Resumable", expect.ToEqual("1.0.0")).
		Header("Content-Type", expect.ToEqual("application/offset+octet-stream")).
		Header("Upload-Offset", expect.Func(func(v any, a expect.Args) (bool, error) {
			if a.RequestInfo.Request.Method != http.MethodPatch {
				return false, nil // mocha runs may run this for any other request
			}
			num, e := strconv.Atoi(v.(string))
			return num >= 0, e
		})).
		Header("Upload-Offset", expect.Func(func(v any, a expect.Args) (bool, error) {
			if a.RequestInfo.Request.Method != http.MethodPatch {
				return false, nil // mocha runs may run this for any other request
			}
			num, e := strconv.Atoi(v.(string))
			return num == mtu.buf.Len(), e
		}))
	for _, h := range emptyHeaders {
		b = b.Header(h, expect.ToBeEmpty())
	}
	return b
}

// tRandomData returns n random bytes
func tRandomData(n int) []byte {
	data, _ := io.ReadAll(io.LimitReader(rand.New(rand.NewSource(time.Now().UnixNano())), int64(n)))
	return data
}

var _ = Describe("ChunkedWriter", func() {
	var testClient *Client
	var testURL *url.URL
	var srvMock *mocha.Mocha
	var emptyHeaders []string

	// newWriter returns a stream for the upload and a ChunkedWriter on top of it
	newWriter := func(u Upload, chunkSize int) (*Stream, *ChunkedWriter) {
		GinkgoHelper()
		s, err := NewStream(testClient, u)
		Ω(err).ShouldNot(HaveOccurred())
		return s, NewChunkedWriter(s, chunkSize)
	}
	// mockUploader adds a PATCH mock for /foo/bar replying 204 to the given number of requests. acceptSizes are passed
	// to mockTusUploader as is.
	mockUploader := func(requests int, acceptSizes ...int) *mockTusUploader {
		GinkgoHelper()
		replies := make([]*reply.StdReply, requests)
		for i := range replies {
			replies[i] = tReply(reply.NoContent())
		}
		up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: acceptSizes}
		srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
		return up
	}

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
	Context("NewChunkedWriter", func() {
		It("should create a writer with an empty buffer of chunk size", func() {
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)

			Ω(w.w).Should(BeIdenticalTo(s))
			Ω(w.buf).Should(HaveLen(256))
			Ω(w.Buffer()).Should(BeEmpty())
		})
		DescribeTable("should panic if chunk size is not positive",
			func(chunkSize int) {
				s, err := NewStream(testClient, Upload{Location: "/foo/bar", RemoteSize: 1024})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(func() { NewChunkedWriter(s, chunkSize) }).Should(PanicWith(ContainSubstring("invalid chunk size")))
			},
			Entry("zero", 0),
			Entry("negative", -1),
		)
		It("should panic if stream is nil", func() {
			Ω(func() { NewChunkedWriter(nil, 256) }).Should(PanicWith("stream is nil"))
		})
	})
	Context("happy path", func() {
		DescribeTable("should upload data in chunks",
			func(dataSize, uploadSize, expectRequests int) {
				up := mockUploader(expectRequests)
				s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: int64(uploadSize)}, 256)
				data := tRandomData(dataSize)

				Ω(w.ReadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(dataSize))
				Ω(s.Size()).Should(BeEquivalentTo(uploadSize))
				Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
				Ω(w.Buffer()).Should(BeEmpty())
				Ω(up.requests).Should(HaveLen(expectRequests))
				for _, r := range up.requests {
					Ω(r.ContentLength).Should(BeNumerically("<=", 256))
				}
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("data aligned to chunk size", 1024, 1024, 4),
			Entry("data unaligned to chunk size", 1000, 1024, 4),
			Entry("data less than chunk size", 100, 1024, 1),
			Entry("data equal to chunk size", 256, 1024, 1),
			Entry("data and upload less than chunk size", 100, 100, 1),
			Entry("data exceeds upload size, which is not checked on the client", 2048, 1024, 8),
		)
		It("should do nothing if the reader is empty", func() {
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 64}, 256)

			Ω(w.ReadFrom(bytes.NewReader(nil))).Should(BeEquivalentTo(0))
			Ω(s.Tell()).Should(BeEquivalentTo(64))
			Ω(w.Buffer()).Should(BeEmpty())
		})
		It("should not limit reading by the upload size, leaving the excess for the server to reject", func() {
			replies := []*reply.StdReply{
				tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				tReply(reply.Status(http.StatusRequestEntityTooLarge)),
			}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{-1, -1, -1, 232}}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1000}, 256)
			data := tRandomData(2048)
			rd := bytes.NewReader(data)

			// 256@0, 256@256, 256@512, 256@768 -> 232 confirmed, 24@1000 -> 413
			n, err := w.ReadFrom(rd)
			Ω(n).Should(BeEquivalentTo(1024))
			Ω(err).Should(MatchError(ErrTooLarge))
			Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusRequestEntityTooLarge))
			Ω(s.Tell()).Should(BeEquivalentTo(1000))
			Ω(w.Buffer()).Should(Equal(data[1000:1024]))
			Ω(rd.Len()).Should(Equal(1024))
			Ω(up.requests).Should(HaveLen(5))
			Ω(data[:1000]).Should(Equal(up.buf.Bytes()))
		})
		It("should upload data until the reader is exhausted if upload size is unknown", func() {
			up := mockUploader(4)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: SizeUnknown}, 256)
			data := tRandomData(1000)

			Ω(w.ReadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(1000))
			Ω(s.Size()).Should(BeEquivalentTo(SizeUnknown))
			Ω(s.Tell()).Should(BeEquivalentTo(1000))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(up.requests).Should(HaveLen(4))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should send checksum of every chunk in request header", func() {
			testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
			emptyHeaders = []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
			up := mockUploader(4)
			s, err := NewStreamWithChecksum(testClient, Upload{Location: "/foo/bar", RemoteSize: 1024}, "sha1")
			Ω(err).ShouldNot(HaveOccurred())
			w := NewChunkedWriter(s, 256)
			data := tRandomData(1024)

			Ω(w.ReadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(1024))
			Ω(data).Should(Equal(up.buf.Bytes()))
			Ω(up.requests).Should(HaveLen(4))
			for i, r := range up.requests {
				sum := sha1.Sum(data[i*256 : i*256+256])
				Ω(r.Header.Get("Upload-Checksum")).Should(Equal("sha1 " + base64.StdEncoding.EncodeToString(sum[:])))
			}
		})
	})
	Context("server accepts only a part of a chunk", func() {
		DescribeTable("should resend the rest of the chunk in the same call",
			func(dataSize int, acceptSizes []int, expectRequests int) {
				up := mockUploader(expectRequests, acceptSizes...)
				s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
				data := tRandomData(dataSize)

				Ω(w.ReadFrom(bytes.NewReader(data))).Should(BeEquivalentTo(dataSize))
				Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
				Ω(w.Buffer()).Should(BeEmpty())
				Ω(up.requests).Should(HaveLen(expectRequests))
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			// 256@0 -> 100 confirmed, 156@100, then 3 full chunks
			Entry("first chunk", 1024, []int{100}, 5),
			// 256@0 -> nothing confirmed, 256@0, then 3 full chunks
			Entry("first chunk, nothing accepted", 1024, []int{0}, 5),
			// 256@0 -> 255 confirmed, 1@255, then 3 full chunks
			Entry("first chunk except the last byte", 1024, []int{255}, 5),
			// 256@0, 256@256 -> 100 confirmed, 156@356, then 2 full chunks
			Entry("middle chunk", 1024, []int{-1, 100}, 5),
			// 3 full chunks, 232@768 -> 100 confirmed, 132@868
			Entry("last unaligned chunk", 1000, []int{-1, -1, -1, 100}, 5),
			// 256@0 -> nothing confirmed twice, 50 confirmed, 206@50 -> nothing confirmed twice, then the rest of the
			// chunk and 3 full chunks. The counter of requests with nothing confirmed is reset by the progress.
			Entry("nothing accepted in non-consecutive requests", 1024, []int{0, 0, 50, 0, 0}, 9),
		)
		DescribeTable("should return ErrIncompleteWrite and keep the rest in the buffer if nothing is accepted "+
			"in 3 consecutive requests, and resend it on the next call",
			func(acceptSizes []int, expectOffset int64, expectRequests int) {
				up := mockUploader(expectRequests, acceptSizes...)
				s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
				data := tRandomData(1024)
				rd := bytes.NewReader(data)

				n, err := w.ReadFrom(rd)
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(MatchError(ErrIncompleteWrite))
				Ω(s.Tell()).Should(Equal(expectOffset))
				Ω(w.Buffer()).Should(Equal(data[expectOffset:256]))

				// The same reader is passed, the rest of the chunk is sent first
				Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(768))
				Ω(s.Tell()).Should(BeEquivalentTo(1024))
				Ω(w.Buffer()).Should(BeEmpty())
				Ω(up.requests).Should(HaveLen(expectRequests))
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			// 256@0 x3 -> nothing confirmed; next call: 256@0, then 3 full chunks
			Entry("nothing accepted", []int{0, 0, 0}, int64(0), 7),
			// 256@0 -> 100 confirmed, 156@100 x3 -> nothing confirmed; next call: 156@100, then 3 full chunks
			Entry("a part accepted", []int{100, 0, 0, 0}, int64(100), 8),
		)
	})
	Context("retry to upload data after error", func() {
		It("should resend the buffered chunk after an error in the middle", func() {
			replies := []*reply.StdReply{
				tReply(reply.NoContent()), tReply(reply.NoContent()), reply.InternalServerError(), tReply(reply.NoContent()), tReply(reply.NoContent()),
			}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			rd := bytes.NewReader(data)

			n, err := w.ReadFrom(rd)
			Ω(err).Should(MatchError(ErrUnexpectedResponse))
			Ω(n).Should(BeEquivalentTo(768))
			Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusInternalServerError))
			Ω(s.Tell()).Should(BeEquivalentTo(512))
			Ω(w.Buffer()).Should(Equal(data[512:768]))

			Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(256))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should resend the buffered chunk after an error at the end of unaligned data", func() {
			replies := []*reply.StdReply{
				tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), reply.InternalServerError(), tReply(reply.NoContent()),
			}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1000)
			rd := bytes.NewReader(data)

			n, err := w.ReadFrom(rd)
			Ω(err).Should(MatchError(ErrUnexpectedResponse))
			Ω(n).Should(BeEquivalentTo(1000))
			Ω(s.Tell()).Should(BeEquivalentTo(768))
			Ω(w.Buffer()).Should(Equal(data[768:]))

			Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(0))
			Ω(s.Tell()).Should(BeEquivalentTo(1000))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should keep the data read before a read error and send it on the next call", func() {
			up := mockUploader(5)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			errRead := errors.New("read error")

			n, err := w.ReadFrom(io.MultiReader(bytes.NewReader(data[:100]), iotest.ErrReader(errRead)))
			Ω(n).Should(BeEquivalentTo(100))
			Ω(err).Should(MatchError(errRead))
			Ω(s.Tell()).Should(BeEquivalentTo(0))
			Ω(w.Buffer()).Should(Equal(data[:100]))

			// 100@0, then 256@100, 256@356, 256@612, 156@868
			Ω(w.ReadFrom(bytes.NewReader(data[100:]))).Should(BeEquivalentTo(924))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(5))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
	})
	Context("Flush", func() {
		It("should do nothing if the buffer is empty", func() {
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)

			Ω(w.Flush()).Should(Equal(0))
			Ω(s.Tell()).Should(BeEquivalentTo(0))
		})
		// The first 4 requests leave data[100:256] in the buffer: 256@0 -> 100 confirmed, 156@100 x3 -> nothing confirmed
		It("should send the buffered data in a single request", func() {
			up := mockUploader(5, 100, 0, 0, 0)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			_, err := w.ReadFrom(bytes.NewReader(data))
			Ω(err).Should(MatchError(ErrIncompleteWrite))

			Ω(w.Flush()).Should(Equal(156))
			Ω(s.Tell()).Should(BeEquivalentTo(256))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(up.requests).Should(HaveLen(5))
			Ω(data[:256]).Should(Equal(up.buf.Bytes()))
		})
		It("should return ErrIncompleteWrite and keep the rest if server accepts a part of the buffer", func() {
			up := mockUploader(5, 100, 0, 0, 0, 50)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			_, err := w.ReadFrom(bytes.NewReader(data))
			Ω(err).Should(MatchError(ErrIncompleteWrite))

			n, err := w.Flush()
			Ω(n).Should(Equal(50))
			Ω(err).Should(MatchError(ErrIncompleteWrite))
			Ω(s.Tell()).Should(BeEquivalentTo(150))
			Ω(w.Buffer()).Should(Equal(data[150:256]))
			Ω(data[:150]).Should(Equal(up.buf.Bytes()))
		})
		It("should return error and keep the buffer if the request fails", func() {
			replies := []*reply.StdReply{
				tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				tReply(reply.Status(http.StatusConflict)),
			}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{100, 0, 0, 0}}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			_, err := w.ReadFrom(bytes.NewReader(data))
			Ω(err).Should(MatchError(ErrIncompleteWrite))

			n, err := w.Flush()
			Ω(n).Should(Equal(0))
			Ω(err).Should(MatchError(ErrOffsetsNotSynced))
			Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusConflict))
			Ω(s.Tell()).Should(BeEquivalentTo(100))
			Ω(w.Buffer()).Should(Equal(data[100:256]))
		})
	})
	Context("Reset", func() {
		It("should drop the buffered data, so that the next ReadFrom continues reading the reader", func() {
			up := mockUploader(7, 100, 0, 0, 0)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			rd := bytes.NewReader(data)
			// 256@0 -> 100 confirmed, 156@100 x3 -> nothing confirmed
			_, err := w.ReadFrom(rd)
			Ω(err).Should(MatchError(ErrIncompleteWrite))

			w.Reset()
			Ω(w.Buffer()).Should(BeEmpty())

			// data[100:256] is dropped, 256@100, 256@356, 256@612
			Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(768))
			Ω(s.Tell()).Should(BeEquivalentTo(868))
			Ω(up.requests).Should(HaveLen(7))
			Ω(append(data[:100:100], data[256:]...)).Should(Equal(up.buf.Bytes()))
		})
	})
	Context("Advance", func() {
		DescribeTable("should discard the first n bytes of the buffered data",
			func(n int64, expectOk bool, expectFrom int) {
				_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
				data := tRandomData(256)
				w.bufPtr = data[100:]

				Ω(w.Advance(n)).Should(Equal(expectOk))
				Ω(w.Buffer()).Should(Equal(data[expectFrom:]))
			},
			Entry("zero", int64(0), true, 100),
			Entry("a part of the buffer", int64(50), true, 150),
			Entry("the whole buffer", int64(156), true, 256),
		)
		DescribeTable("should empty the buffer and return false if n is out of the buffered data",
			func(n int64) {
				_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
				w.bufPtr = tRandomData(256)[100:]

				Ω(w.Advance(n)).Should(BeFalse())
				Ω(w.Buffer()).Should(BeEmpty())
			},
			Entry("one byte beyond the buffer", int64(157)),
			Entry("far beyond the buffer", int64(1024)),
			Entry("negative", int64(-1)),
		)
		DescribeTable("on empty buffer",
			func(n int64, expectOk bool) {
				_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)

				Ω(w.Advance(n)).Should(Equal(expectOk))
				Ω(w.Buffer()).Should(BeEmpty())
			},
			Entry("zero", int64(0), true),
			Entry("positive", int64(1), false),
			Entry("negative", int64(-1), false),
		)

		// The first 4 requests leave data[100:256] in the buffer: 256@0 -> 100 confirmed, 156@100 x3 -> nothing
		// confirmed. Then the server gets more data, whose confirmation is lost, which is emulated by writing to up.buf.
		// The HEAD mock is matched before the PATCH mock, whose Upload-Offset matchers fail on requests without
		// this header.
		It("should skip the data the server has already received after Sync, without rewinding the reader", func() {
			up := mockUploader(8, 100, 0, 0, 0)
			eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
			srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).
				Reply(tReply(reply.Status(http.StatusOK)).
					Header("Upload-Offset", "150").
					Header("Upload-Length", "1024")),
			)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			rd := bytes.NewReader(data)
			_, err := w.ReadFrom(rd)
			Ω(err).Should(MatchError(ErrIncompleteWrite))
			up.buf.Write(data[100:150])

			d, err := s.Sync()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(d).Should(BeEquivalentTo(50))
			Ω(w.Advance(d)).Should(BeTrue())
			Ω(w.Buffer()).Should(Equal(data[150:256]))

			// 106@150, then 3 full chunks
			Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(768))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(up.requests).Should(HaveLen(8))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should require rewinding the reader if the server offset is beyond the buffered data after Sync", func() {
			up := mockUploader(7, 100, 0, 0, 0)
			eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata", "Upload-Checksum", "Upload-Offset"}
			srvMock.AddMocks(tRequest(http.MethodHead, "/foo/bar", eh).Priority(-1).
				Reply(tReply(reply.Status(http.StatusOK)).
					Header("Upload-Offset", "300").
					Header("Upload-Length", "1024")),
			)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			rd := bytes.NewReader(data)
			_, err := w.ReadFrom(rd)
			Ω(err).Should(MatchError(ErrIncompleteWrite))
			up.buf.Write(data[100:300])

			d, err := s.Sync()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(d).Should(BeEquivalentTo(200))
			Ω(w.Advance(d)).Should(BeFalse())
			Ω(w.Buffer()).Should(BeEmpty())

			// 256@300, 256@556, 212@812
			Ω(rd.Seek(s.Tell(), io.SeekStart)).Should(BeEquivalentTo(300))
			Ω(w.ReadFrom(rd)).Should(BeEquivalentTo(724))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(7))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
	})
	Context("Write", func() {
		DescribeTable("should upload data in chunks",
			func(dataSize, uploadSize, expectRequests int) {
				up := mockUploader(expectRequests)
				s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: int64(uploadSize)}, 256)
				data := tRandomData(dataSize)

				Ω(w.Write(data)).Should(Equal(dataSize))
				Ω(s.Tell()).Should(BeEquivalentTo(dataSize))
				Ω(w.Buffer()).Should(BeEmpty())
				Ω(up.requests).Should(HaveLen(expectRequests))
				for _, r := range up.requests {
					Ω(r.ContentLength).Should(BeNumerically("<=", 256))
				}
				Ω(data).Should(Equal(up.buf.Bytes()))
			},
			Entry("data aligned to chunk size", 1024, 1024, 4),
			Entry("data unaligned to chunk size", 1000, 1024, 4),
			Entry("data less than chunk size", 100, 1024, 1),
			Entry("upload size is unknown", 1000, SizeUnknown, 4),
		)
		It("should upload data with io.Copy from an io.WriterTo source", func() {
			up := mockUploader(4)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)

			// bytes.Reader implements io.WriterTo, so io.Copy calls Write instead of ReadFrom
			Ω(io.Copy(w, bytes.NewReader(data))).Should(BeEquivalentTo(1024))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(4))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should resend the rest of the chunk if the server accepts a part of it", func() {
			// 256@0, 256@256 -> 100 confirmed, 156@356, then 2 full chunks
			up := mockUploader(5, -1, 100)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)

			Ω(w.Write(data)).Should(Equal(1024))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(5))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should upload the data the server accepts and return ErrTooLarge when the server rejects the excess", func() {
			replies := []*reply.StdReply{
				tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()), tReply(reply.NoContent()),
				tReply(reply.Status(http.StatusRequestEntityTooLarge)),
			}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0)), acceptSizes: []int{-1, -1, -1, 232}}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1000}, 256)
			data := tRandomData(1024)

			// 256@0, 256@256, 256@512, 256@768 -> 232 confirmed, 24@1000 -> 413
			n, err := w.Write(data)
			Ω(n).Should(Equal(1000))
			Ω(err).Should(MatchError(ErrTooLarge))
			Ω(s.Tell()).Should(BeEquivalentTo(1000))
			Ω(up.requests).Should(HaveLen(5))
			Ω(data[:1000]).Should(Equal(up.buf.Bytes()))
		})
		It("should send the data even if the upload is full, and return ErrTooLarge if the server rejects it", func() {
			replies := []*reply.StdReply{tReply(reply.Status(http.StatusRequestEntityTooLarge))}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 1024))}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024, RemoteOffset: 1024}, 256)

			n, err := w.Write(tRandomData(100))
			Ω(n).Should(Equal(0))
			Ω(err).Should(MatchError(ErrTooLarge))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(1))
		})
		It("should do nothing if data is empty", func() {
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)

			Ω(w.Write(nil)).Should(Equal(0))
			Ω(s.Tell()).Should(BeEquivalentTo(0))
		})
		It("should return ErrIncompleteWrite if nothing is accepted in 3 consecutive requests, and continue on the next call", func() {
			// 256@0, then 256@256 x3 -> nothing confirmed; next call: 256@256, 256@512, 256@768
			up := mockUploader(7, -1, 0, 0, 0)
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)

			n, err := w.Write(data)
			Ω(n).Should(Equal(256))
			Ω(err).Should(MatchError(ErrIncompleteWrite))
			Ω(s.Tell()).Should(BeEquivalentTo(256))
			Ω(w.Buffer()).Should(BeEmpty())

			Ω(w.Write(data[n:])).Should(Equal(768))
			Ω(s.Tell()).Should(BeEquivalentTo(1024))
			Ω(up.requests).Should(HaveLen(7))
			Ω(data).Should(Equal(up.buf.Bytes()))
		})
		It("should return the confirmed bytes and error if the request fails", func() {
			replies := []*reply.StdReply{tReply(reply.NoContent()), tReply(reply.Status(http.StatusConflict))}
			up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
			srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
			s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)

			n, err := w.Write(data)
			Ω(n).Should(Equal(256))
			Ω(err).Should(MatchError(ErrOffsetsNotSynced))
			Ω(tErrResponse(err)).Should(HaveHTTPStatus(http.StatusConflict))
			Ω(s.Tell()).Should(BeEquivalentTo(256))
			Ω(w.Buffer()).Should(BeEmpty())
			Ω(data[:256]).Should(Equal(up.buf.Bytes()))
		})
		It("should not modify the data buffered by ReadFrom", func() {
			// 256@0 -> 100 confirmed, 156@100 x3 -> nothing confirmed; Write: 256@100
			up := mockUploader(5, 100, 0, 0, 0)
			_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			data := tRandomData(1024)
			_, err := w.ReadFrom(bytes.NewReader(data))
			Ω(err).Should(MatchError(ErrIncompleteWrite))

			other := tRandomData(256)
			Ω(w.Write(other)).Should(Equal(256))
			Ω(w.Buffer()).Should(Equal(data[100:256]))
			Ω(up.requests).Should(HaveLen(5))
		})
	})
	Context("error path", func() {
		DescribeTable("should keep the response body in the error and close the original body",
			func(status int, header http.Header, expectErr error) {
				_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)

				tExpectBodyKeptInError(testClient, status, header, expectErr, func() error {
					_, err := w.ReadFrom(bytes.NewReader(make([]byte, 1024)))
					return err
				})
			},
			Entry("http error", http.StatusConflict, nil, ErrOffsetsNotSynced),
			Entry("offset advanced too far", http.StatusNoContent, http.Header{"Upload-Offset": {"2048"}}, ErrProtocol),
			Entry("malformed Upload-Expires", http.StatusNoContent, http.Header{"Upload-Offset": {"256"}, "Upload-Expires": {"tomorrow"}}, ErrProtocol),
		)
		It("should close the response bodies if the server accepts nothing in consecutive requests", func() {
			_, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
			responder := &fakeResponder{status: http.StatusNoContent, header: http.Header{"Upload-Offset": {"0"}}, body: "server says"}
			testClient.DoRequest = responder.Do
			data := tRandomData(1024)

			n, err := w.ReadFrom(bytes.NewReader(data))
			Ω(n).Should(BeEquivalentTo(256))
			Ω(err).Should(MatchError(ErrIncompleteWrite))
			Ω(w.Buffer()).Should(Equal(data[:256]))
			Ω(responder.bodies).Should(HaveLen(maxConsecutiveZeroWrites))
			for _, b := range responder.bodies {
				Ω(b.closed).Should(BeTrue())
			}
		})
		DescribeTable("http errors handling",
			func(expectStatus int, expectErr error) {
				replies := []*reply.StdReply{tReply(reply.Status(expectStatus))}
				up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				srvMock.AddMocks(up.makePatchRequest("/foo/bar", emptyHeaders).ReplyFunction(up.handler()))
				s, w := newWriter(Upload{Location: "/foo/bar", RemoteSize: 1024}, 256)
				data := tRandomData(1024)

				n, err := w.ReadFrom(bytes.NewReader(data))
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(MatchError(expectErr))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(expectStatus))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(w.Buffer()).Should(Equal(data[:256]))
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
		)
		When("server returned 460 Checksum Mismatch and checksum is used", func() {
			It("should return ErrChecksumMismatch and keep the buffer", func() {
				testClient.caps.Extensions = append(testClient.caps.Extensions, "checksum")
				replies := []*reply.StdReply{tReply(reply.Status(460))}
				up := &mockTusUploader{replies: replies, buf: bytes.NewBuffer(make([]byte, 0))}
				eh := []string{"Upload-Concat", "Upload-Defer-Length", "Upload-Length", "Upload-Metadata"}
				srvMock.AddMocks(up.makePatchRequest("/foo/bar", eh).ReplyFunction(up.handler()))
				s, err := NewStreamWithChecksum(testClient, Upload{Location: "/foo/bar", RemoteSize: 1024}, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				w := NewChunkedWriter(s, 256)
				data := tRandomData(1024)

				n, err := w.ReadFrom(bytes.NewReader(data))
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(MatchError(ErrChecksumMismatch))
				Ω(tErrResponse(err)).Should(HaveHTTPStatus(460))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(w.Buffer()).Should(Equal(data[:256]))
				Ω(up.buf.Len()).Should(Equal(0))
			})
		})
		When("upload with checksum, but checksum extension is not active", func() {
			It("should return error and keep the buffer", func() {
				s, err := NewStreamWithChecksum(testClient, Upload{Location: "/foo/bar", RemoteSize: 1024}, "sha1")
				Ω(err).ShouldNot(HaveOccurred())
				w := NewChunkedWriter(s, 256)
				data := tRandomData(1024)

				n, err := w.ReadFrom(bytes.NewReader(data))
				Ω(n).Should(BeEquivalentTo(256))
				Ω(err).Should(And(
					MatchError(ErrUnsupportedFeature),
					MatchError(ContainSubstring("unsupported feature: checksum")),
				))
				Ω(s.Tell()).Should(BeEquivalentTo(0))
				Ω(w.Buffer()).Should(Equal(data[:256]))
			})
		})
	})
})
