package e2e

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UploadStream", func() {
	const dataSize = 100 * 1024

	var srv *tusServer
	var cl *tusgo.Client
	var data []byte
	var u tusgo.Upload

	BeforeEach(func() {
		var err error

		srv = startServer(nil, nil)
		cl = srv.NewClient()
		data = randomData(dataSize)
		u, err = cl.CreateUpload(dataSize, false, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	chunkSizes := []TableEntry{
		Entry("not chunked", int64(tusgo.NoChunked)),
		Entry("chunk size not dividing the data size", int64(1000)),
		Entry("chunk size dividing the data size", int64(dataSize/4)),
		Entry("chunk size equal to the data size", int64(dataSize)),
		Entry("chunk size larger than the data size", int64(dataSize*2)),
	}

	DescribeTable("should transfer the data with ReadFrom",
		func(chunkSize int64) {
			s := newStream(cl, u)
			s.ChunkSize = chunkSize

			n, err := io.Copy(s, readerOnly(bytes.NewReader(data)))

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(dataSize))
			Expect(s.Tell()).To(BeEquivalentTo(dataSize))
			Expect(s.Dirty()).To(BeFalse())
			Expect(srv.StoredData(u.Location)).To(Equal(data))

			got, err := cl.GetUpload(u.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.RemoteOffset).To(BeEquivalentTo(dataSize))
		},
		chunkSizes,
	)

	DescribeTable("should transfer the data with several Write calls",
		func(chunkSize int64) {
			s := newStream(cl, u)
			s.ChunkSize = chunkSize

			for _, part := range [][]byte{data[:1], data[1 : dataSize/3], data[dataSize/3:]} {
				n, err := s.Write(part)
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(Equal(len(part)))
			}

			Expect(s.Tell()).To(BeEquivalentTo(dataSize))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		},
		chunkSizes,
	)

	DescribeTable("should read from the source no more than the upload size",
		func(chunkSize int64) {
			s := newStream(cl, u)
			s.ChunkSize = chunkSize

			n, err := s.ReadFrom(bytes.NewReader(append(data, 1, 2, 3)))

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(dataSize))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		},
		chunkSizes,
	)

	DescribeTable("should write no more than the upload size",
		func(chunkSize int64) {
			s := newStream(cl, u)
			s.ChunkSize = chunkSize

			n, err := s.Write(append(data, 1, 2, 3))

			Expect(err).To(MatchError(io.ErrShortWrite))
			Expect(n).To(Equal(dataSize))
			Expect(srv.StoredData(u.Location)).To(Equal(data))
		},
		chunkSizes,
	)

	It("should resume the transfer in a new stream", func() {
		s := newStream(cl, u)
		_, err := io.CopyN(s, bytes.NewReader(data), dataSize/3)
		Expect(err).NotTo(HaveOccurred())

		// Start over with only the location of the upload, as if the program has restarted
		cl2 := srv.NewClient()
		u2, err := cl2.GetUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())
		s2 := newStream(cl2, u2)
		src := bytes.NewReader(data)
		_, err = src.Seek(s2.Tell(), io.SeekStart)
		Expect(err).NotTo(HaveOccurred())
		_, err = io.Copy(s2, src)

		Expect(err).NotTo(HaveOccurred())
		Expect(s2.Tell()).To(BeEquivalentTo(dataSize))
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})

	It("should detect the offsets out of sync and recover after Sync", func() {
		u2 := u
		s1 := newStream(cl, u)
		s2 := newStream(cl, u2)
		_, err := s1.Write(data[:1000])
		Expect(err).NotTo(HaveOccurred())

		_, err = s2.Write(data[:1000])
		Expect(err).To(MatchError(tusgo.ErrOffsetsNotSynced))

		err = s2.Sync()
		Expect(err).NotTo(HaveOccurred())
		Expect(s2.Tell()).To(BeEquivalentTo(1000))
		src := bytes.NewReader(data)
		_, err = src.Seek(s2.Tell(), io.SeekStart)
		Expect(err).NotTo(HaveOccurred())
		_, err = io.Copy(s2, src)

		Expect(err).NotTo(HaveOccurred())
		Expect(srv.StoredData(u.Location)).To(Equal(data))
	})

	It("should transfer the data to an upload with deferred length", func() {
		deferred, err := cl.CreateUpload(tusgo.SizeUnknown, false, nil)
		Expect(err).NotTo(HaveOccurred())

		deferred.RemoteSize = dataSize
		s := newStream(cl, deferred)
		s.SetUploadSize = true
		s.ChunkSize = dataSize / 4
		_, err = io.Copy(s, bytes.NewReader(data))

		Expect(err).NotTo(HaveOccurred())
		Expect(srv.StoredData(deferred.Location)).To(Equal(data))
	})

	It("should return error when writing to a deleted upload", func() {
		err := cl.DeleteUpload(u.Location)
		Expect(err).NotTo(HaveOccurred())

		_, err = newStream(cl, u).Write(data)

		Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
		Expect(errResponse(err)).To(And(
			HaveHTTPStatus(http.StatusNotFound),
			HaveHTTPBody(ContainSubstring("ERR_UPLOAD_NOT_FOUND")),
		))
	})
})
