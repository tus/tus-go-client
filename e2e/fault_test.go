package e2e

import (
	"bytes"
	"net/http"

	"github.com/bdragon300/tusgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fault recovery", func() {
	const (
		dataSize  = 10 * 1000
		chunkSize = 2048
		// 4 full chunks and the rest
		chunks = (dataSize + chunkSize - 1) / chunkSize
	)

	var data []byte

	BeforeEach(func() {
		data = randomData(dataSize)
	})

	Context("server accepts only a part of the sent data", func() {
		It("should return ErrIncompleteWrite in Stream and resume from the accepted bytes", func() {
			srv := startServer(nil, injectFault(http.MethodPatch, 1, acceptOnly(dataSize/3)))
			cl := srv.NewClient()
			u, err := cl.CreateUpload(dataSize, false, nil)
			Expect(err).NotTo(HaveOccurred())
			s := newStream(cl, u)

			n, err := s.Write(data)
			Expect(err).To(MatchError(tusgo.ErrIncompleteWrite))
			Expect(n).To(Equal(dataSize / 3))
			Expect(s.Tell()).To(BeEquivalentTo(dataSize / 3))

			n, err = s.Write(data[s.Tell():])

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(dataSize - dataSize/3))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		})

		It("should resend the rest of the chunk in the same ReadFrom call in ChunkedWriter", func() {
			patches := &requestCounter{method: http.MethodPatch}
			srv := startServer(nil, chain(patches.Middleware, injectFault(http.MethodPatch, 2, acceptOnly(100))))
			cl := srv.NewClient()
			u, err := cl.CreateUpload(dataSize, false, nil)
			Expect(err).NotTo(HaveOccurred())
			s, w := newChunkedWriter(cl, u, chunkSize)

			n, err := w.ReadFrom(readerOnly{bytes.NewReader(data)})

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(dataSize))
			Expect(s.Tell()).To(BeEquivalentTo(dataSize))
			Expect(patches.count.Load()).To(BeEquivalentTo(chunks + 1))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		})
	})

	Context("server fails to handle a request", func() {
		It("should keep the chunk in ChunkedWriter buffer and resend it on the next ReadFrom call", func() {
			srv := startServer(nil, injectFault(http.MethodPatch, 3, replyStatus(http.StatusInternalServerError)))
			cl := srv.NewClient()
			u, err := cl.CreateUpload(dataSize, false, nil)
			Expect(err).NotTo(HaveOccurred())
			s, w := newChunkedWriter(cl, u, chunkSize)
			rd := readerOnly{bytes.NewReader(data)}

			n, err := w.ReadFrom(rd)
			Expect(err).To(MatchError(tusgo.ErrUnexpectedResponse))
			Expect(errResponse(err)).To(HaveHTTPStatus(http.StatusInternalServerError))
			Expect(n).To(BeEquivalentTo(3 * chunkSize))
			Expect(s.Tell()).To(BeEquivalentTo(2 * chunkSize))
			Expect(w.Buffer()).To(Equal(data[2*chunkSize : 3*chunkSize]))

			n, err = w.ReadFrom(rd)

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(dataSize - 3*chunkSize))
			Expect(w.Buffer()).To(BeEmpty())
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		})
	})

	Context("response is lost after the server has stored the data", func() {
		It("should adopt the server offset after Sync, drop the stored chunk with Reset and continue in ChunkedWriter", func() {
			srv := startServer(nil, injectFault(http.MethodPatch, 2, dropResponse()))
			cl := srv.NewClient()
			u, err := cl.CreateUpload(dataSize, false, nil)
			Expect(err).NotTo(HaveOccurred())
			s, w := newChunkedWriter(cl, u, chunkSize)
			rd := readerOnly{bytes.NewReader(data)}

			n, err := w.ReadFrom(rd)
			Expect(err).To(HaveOccurred())
			Expect(n).To(BeEquivalentTo(2 * chunkSize))
			Expect(s.Tell()).To(BeEquivalentTo(chunkSize))
			Expect(w.Buffer()).To(Equal(data[chunkSize : 2*chunkSize]))

			// The server has already stored the buffered chunk, so it must not be sent again
			delta, err := s.Sync()
			Expect(err).NotTo(HaveOccurred())
			Expect(delta).To(BeEquivalentTo(chunkSize))
			w.Reset()

			n, err = w.ReadFrom(rd)

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(dataSize - 2*chunkSize))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		})
	})
})
