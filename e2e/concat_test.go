package e2e

import (
	"bytes"
	"io"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concatenation", func() {
	var srv *tusServer
	var cl *tusgo.Client
	var parts [][]byte

	// createPartial creates a partial upload and transfers the first n bytes of data to it.
	createPartial := func(data []byte, n int) *tusgo.UploadStream {
		GinkgoHelper()
		u := tusgo.Upload{}
		_, err := cl.CreateUpload(&u, int64(len(data)), true, nil)
		Expect(err).NotTo(HaveOccurred())
		s := tusgo.NewUploadStream(cl, &u)
		_, err = io.Copy(s, bytes.NewReader(data[:n]))
		Expect(err).NotTo(HaveOccurred())
		return s
	}

	BeforeEach(func() {
		srv = startServer(nil, nil)
		cl = srv.NewClient()
		parts = [][]byte{randomData(1000), randomData(1), randomData(5000)}
	})

	It("should concatenate the finished partial uploads", func() {
		var partials []tusgo.Upload
		for _, p := range parts {
			partials = append(partials, *createPartial(p, len(p)).Upload)
		}
		final := tusgo.Upload{}

		_, err := cl.ConcatenateUploads(&final, partials, map[string]string{"k": "v"})

		Expect(err).NotTo(HaveOccurred())
		expected := bytes.Join(parts, nil)
		Expect(srv.StoredData(final)).To(Equal(expected))
		got := tusgo.Upload{}
		_, err = cl.GetUpload(&got, final.Location)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteOffset).To(BeEquivalentTo(len(expected)))
		Expect(got.RemoteSize).To(BeEquivalentTo(len(expected)))
		Expect(got.Partial).To(BeFalse())
		Expect(got.Metadata).To(Equal(map[string]string{"k": "v"}))
	})

	It("should concatenate the finished streams", func() {
		var streams []*tusgo.UploadStream
		for _, p := range parts {
			streams = append(streams, createPartial(p, len(p)))
		}
		final := tusgo.Upload{}

		_, err := cl.ConcatenateStreams(&final, streams, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(srv.StoredData(final)).To(Equal(bytes.Join(parts, nil)))
	})

	It("should refuse to concatenate an unfinished stream, since tusd lacks concatenation-unfinished", func() {
		streams := []*tusgo.UploadStream{createPartial(parts[0], len(parts[0])), createPartial(parts[2], 10)}

		_, err := cl.ConcatenateStreams(&tusgo.Upload{}, streams, nil)

		Expect(err).To(MatchError(tusgo.ErrUnsupportedFeature))
	})

	It("should return error if a partial upload is unfinished", func() {
		partials := []tusgo.Upload{*createPartial(parts[0], len(parts[0])).Upload, *createPartial(parts[2], 10).Upload}

		_, err := cl.ConcatenateUploads(&tusgo.Upload{}, partials, nil)

		Expect(err).To(MatchError(tusgo.ErrUnexpectedResponse))
	})

	It("should refuse to write to the final upload", func() {
		partial := createPartial(parts[0], len(parts[0]))
		final := tusgo.Upload{}
		_, err := cl.ConcatenateUploads(&final, []tusgo.Upload{*partial.Upload}, nil)
		Expect(err).NotTo(HaveOccurred())

		final.RemoteSize = int64(len(parts[0])) * 2
		final.RemoteOffset = int64(len(parts[0]))
		_, err = tusgo.NewUploadStream(cl, &final).Write([]byte("extra"))

		Expect(err).To(MatchError(tusgo.ErrCannotUpload))
	})
})
