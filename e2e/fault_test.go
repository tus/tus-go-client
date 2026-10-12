package e2e

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fault recovery", func() {
	const (
		dataSize  = 10 * 1000
		chunkSize = 1000
	)

	var data []byte

	// setup starts the server with the given middleware and creates an upload of dataSize on it.
	setup := func(mw middleware) (*tusServer, *tusgo.UploadStream) {
		GinkgoHelper()
		srv := startServer(nil, mw)
		cl := srv.NewClient()
		u, _, err := cl.CreateUpload(dataSize, false, nil)
		Expect(err).NotTo(HaveOccurred())
		s := tusgo.NewUploadStream(cl, &u)
		s.ChunkSize = chunkSize
		return srv, s
	}

	BeforeEach(func() {
		data = randomData(dataSize)
	})

	Context("server accepts only a part of the sent data", func() {
		It("should resend the rest of the chunk in chunked mode", func() {
			patches := &requestCounter{method: http.MethodPatch}
			fault := injectFault(http.MethodPatch, 3, acceptOnly(chunkSize/3))
			srv, s := setup(func(next http.Handler) http.Handler { return patches.Middleware(fault(next)) })

			_, err := io.Copy(s, readerOnly(bytes.NewReader(data)))

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(*s.Upload)).To(Equal(data))
			Expect(patches.count.Load()).To(BeEquivalentTo(dataSize/chunkSize + 1))
		})

		It("should return io.ErrShortWrite in not chunked mode and resume after Sync", func() {
			srv, s := setup(injectFault(http.MethodPatch, 1, acceptOnly(dataSize/3)))
			s.ChunkSize = tusgo.NoChunked
			src := bytes.NewReader(data)

			_, err := io.Copy(s, src)
			Expect(err).To(MatchError(io.ErrShortWrite))
			Expect(s.Tell()).To(BeEquivalentTo(dataSize / 3))

			_, err = s.Sync()
			Expect(err).NotTo(HaveOccurred())
			_, err = src.Seek(s.Tell(), io.SeekStart)
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(s, src)

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(*s.Upload)).To(Equal(data))
		})
	})

	Context("connection breaks after the server has committed the chunk", func() {
		It("should report the offsets out of sync on ReadFrom retry and resume after Sync", func() {
			srv, s := setup(injectFault(http.MethodPatch, 3, dropResponse()))
			src := bytes.NewReader(data)

			_, err := io.Copy(s, readerOnly(src))
			Expect(err).To(HaveOccurred())
			Expect(s.Dirty()).To(BeTrue())
			Expect(s.Tell()).To(BeEquivalentTo(2 * chunkSize))

			// The dirty buffer is resent at the stale offset, but the server already has this chunk
			_, err = io.Copy(s, readerOnly(src))
			Expect(err).To(MatchError(tusgo.ErrOffsetsNotSynced))

			_, err = s.Sync()
			Expect(err).NotTo(HaveOccurred())
			Expect(s.Tell()).To(BeEquivalentTo(3 * chunkSize))
			s.ForceClean()
			_, err = src.Seek(s.Tell(), io.SeekStart)
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(s, src)

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(*s.Upload)).To(Equal(data))
		})

		It("should report the offsets out of sync on Write retry and resume after Sync", func() {
			srv, s := setup(injectFault(http.MethodPatch, 3, dropResponse()))

			n, err := s.Write(data)
			Expect(err).To(HaveOccurred())
			Expect(n).To(Equal(2 * chunkSize))
			Expect(s.Dirty()).To(BeFalse())

			_, err = s.Write(data[n:])
			Expect(err).To(MatchError(tusgo.ErrOffsetsNotSynced))

			_, err = s.Sync()
			Expect(err).NotTo(HaveOccurred())
			Expect(s.Tell()).To(BeEquivalentTo(3 * chunkSize))
			_, err = s.Write(data[s.Tell():])

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(*s.Upload)).To(Equal(data))
		})
	})
})
