package e2e

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bdragon300/tusgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChunkedWriter", func() {
	const (
		dataSize  = 100 * 1024
		chunkSize = 16 * 1024
		// 6 full chunks and the rest
		chunks = (dataSize + chunkSize - 1) / chunkSize
	)

	var srv *tusServer
	var patches *requestCounter
	var cl *tusgo.Client
	var data []byte
	var u tusgo.Upload

	BeforeEach(func() {
		var err error

		patches = &requestCounter{method: http.MethodPatch}
		srv = startServer(nil, patches.Middleware)
		cl = srv.NewClient()
		data = randomData(dataSize)
		u, err = cl.CreateUpload(dataSize, false, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should transfer the data in chunks with io.Copy through ReadFrom", func() {
		s, w := newChunkedWriter(cl, u, chunkSize)

		n, err := io.Copy(w, readerOnly{bytes.NewReader(data)})

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeEquivalentTo(dataSize))
		Expect(s.Tell()).To(BeEquivalentTo(dataSize))
		Expect(w.Buffer()).To(BeEmpty())
		Expect(patches.count.Load()).To(BeEquivalentTo(chunks))
		Expect(srv.StoredData(u.Location)).To(Equal(data))

		got, err := cl.GetUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteOffset).To(BeEquivalentTo(dataSize))
	})

	It("should transfer the data in chunks with io.Copy through Write", func() {
		s, w := newChunkedWriter(cl, u, chunkSize)

		// bytes.Reader implements io.WriterTo, so io.Copy calls Write instead of ReadFrom
		n, err := io.Copy(w, bytes.NewReader(data))

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeEquivalentTo(dataSize))
		Expect(s.Tell()).To(BeEquivalentTo(dataSize))
		Expect(patches.count.Load()).To(BeEquivalentTo(chunks))
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})

	It("should not limit reading by the upload size, and keep the chunk rejected by the server in the buffer", func() {
		const fullChunksSize = (chunks - 1) * chunkSize
		s, w := newChunkedWriter(cl, u, chunkSize)
		src := append(data, randomData(1000)...)
		rd := bytes.NewReader(src)

		// The last chunk overflows the upload, so the server rejects it entirely
		n, err := w.ReadFrom(rd)

		Expect(err).To(MatchError(tusgo.ErrTooLarge))
		Expect(n).To(BeEquivalentTo(len(src)))
		Expect(rd.Len()).To(Equal(0))
		Expect(s.Tell()).To(BeEquivalentTo(fullChunksSize))
		Expect(w.Buffer()).To(Equal(src[fullChunksSize:]))
		Expect(patches.count.Load()).To(BeEquivalentTo(chunks))
		Expect(srv.StoredData(u.Location)).To(Equal(data[:fullChunksSize]))
	})

	It("should transfer the chunks the server accepts and return ErrTooLarge when writing more than the upload size", func() {
		const fullChunksSize = (chunks - 1) * chunkSize
		s, w := newChunkedWriter(cl, u, chunkSize)

		// The last chunk overflows the upload, so the server rejects it entirely
		n, err := w.Write(append(data, 1, 2, 3))

		Expect(err).To(MatchError(tusgo.ErrTooLarge))
		Expect(n).To(Equal(fullChunksSize))
		Expect(s.Tell()).To(BeEquivalentTo(fullChunksSize))
		Expect(patches.count.Load()).To(BeEquivalentTo(chunks))
		Expect(srv.StoredData(u.Location)).To(Equal(data[:fullChunksSize]))
	})

	It("should transfer the data to an upload with deferred length declared before the transfer", func() {
		deferred, err := cl.CreateUpload(tusgo.SizeUnknown, false, nil)
		Expect(err).NotTo(HaveOccurred())
		s, w := newChunkedWriter(cl, deferred, chunkSize)
		Expect(s.DeclareSize(dataSize)).To(Succeed())

		n, err := w.ReadFrom(readerOnly{bytes.NewReader(data)})

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeEquivalentTo(dataSize))
		Expect(srv.StoredData(deferred.Location)).To(Equal(data))
	})

	It("should transfer the data to an upload with deferred length declared after the transfer", func() {
		deferred, err := cl.CreateUpload(tusgo.SizeUnknown, false, nil)
		Expect(err).NotTo(HaveOccurred())
		s, w := newChunkedWriter(cl, deferred, chunkSize)

		n, err := w.ReadFrom(readerOnly{bytes.NewReader(data)})
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeEquivalentTo(dataSize))
		Expect(s.Size()).To(BeEquivalentTo(tusgo.SizeUnknown))

		Expect(s.DeclareSize(dataSize)).To(Succeed())

		got, err := cl.GetUpload(deferred.Location)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteSize).To(BeEquivalentTo(dataSize))
		Expect(got.RemoteOffset).To(BeEquivalentTo(dataSize))
		Expect(srv.StoredData(deferred.Location)).To(Equal(data))
	})

	It("should detect the offsets out of sync and recover after Sync and Reset", func() {
		_, err := newStream(cl, u).Write(data[:1000])
		Expect(err).NotTo(HaveOccurred())
		s, w := newChunkedWriter(cl, u, chunkSize)
		rd := bytes.NewReader(data)

		n, err := w.ReadFrom(rd)
		Expect(err).To(MatchError(tusgo.ErrOffsetsNotSynced))
		Expect(errResponse(err)).To(HaveHTTPStatus(http.StatusConflict))
		Expect(n).To(BeEquivalentTo(chunkSize))
		Expect(w.Buffer()).To(Equal(data[:chunkSize]))

		// The buffered data was meant for the old offset, drop it and continue from the server offset
		delta, err := s.Sync()
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(BeEquivalentTo(1000))
		w.Reset()
		_, err = rd.Seek(s.Tell(), io.SeekStart)
		Expect(err).NotTo(HaveOccurred())

		n, err = w.ReadFrom(rd)

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeEquivalentTo(dataSize - 1000))
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})
})
