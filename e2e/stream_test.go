package e2e

import (
	"net/http"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stream", func() {
	const dataSize = 100 * 1024

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

	It("should transfer the data in a single request", func() {
		s := newStream(cl, u)

		n, err := s.Write(data)

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(dataSize))
		Expect(s.Tell()).To(BeEquivalentTo(dataSize))
		Expect(patches.count.Load()).To(BeEquivalentTo(1))
		Expect(srv.StoredData(u.Location)).To(Equal(data))

		got, err := cl.GetUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteOffset).To(BeEquivalentTo(dataSize))
	})

	It("should transfer the data with several Write calls, one request per call", func() {
		s := newStream(cl, u)

		parts := [][]byte{data[:1], data[1 : dataSize/3], data[dataSize/3:]}
		for _, part := range parts {
			n, err := s.Write(part)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(len(part)))
		}

		Expect(s.Tell()).To(BeEquivalentTo(dataSize))
		Expect(patches.count.Load()).To(BeEquivalentTo(len(parts)))
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})

	It("should return ErrTooLarge when writing more than the upload size", func() {
		s := newStream(cl, u)

		n, err := s.Write(append(data, 1, 2, 3))

		Expect(err).To(MatchError(tusgo.ErrTooLarge))
		Expect(errResponse(err)).To(HaveHTTPStatus(http.StatusRequestEntityTooLarge))
		Expect(n).To(Equal(0))
		Expect(s.Tell()).To(BeEquivalentTo(0))

		got, err := cl.GetUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteOffset).To(BeEquivalentTo(0))
	})

	It("should transfer the data to an upload with deferred length", func() {
		deferred, err := cl.CreateUpload(tusgo.SizeUnknown, false, nil)
		Expect(err).NotTo(HaveOccurred())

		s := newStream(cl, deferred)
		err = s.DeclareSize(dataSize)
		Expect(err).NotTo(HaveOccurred())
		Expect(s.Size()).To(BeEquivalentTo(dataSize))

		n, err := s.Write(data)

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(dataSize))
		Expect(srv.StoredData(deferred.Location)).To(Equal(data))
	})

	It("should detect the offsets out of sync and recover after Sync", func() {
		s1 := newStream(cl, u)
		s2 := newStream(cl, u)
		_, err := s1.Write(data[:1000])
		Expect(err).NotTo(HaveOccurred())

		_, err = s2.Write(data[:1000])
		Expect(err).To(MatchError(tusgo.ErrOffsetsNotSynced))

		delta, err := s2.Sync()
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(BeEquivalentTo(1000))
		Expect(s2.Tell()).To(BeEquivalentTo(1000))
		Expect(s2.Size()).To(BeEquivalentTo(dataSize))
		_, err = s2.Write(data[s2.Tell():])

		Expect(err).NotTo(HaveOccurred())
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})

	It("should return error when writing to a deleted upload", func() {
		err := cl.DeleteUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())

		_, err = newStream(cl, u).Write(data)

		Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
		Expect(errResponse(err)).To(HaveHTTPStatus(http.StatusNotFound))
	})
})
