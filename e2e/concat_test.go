package e2e

import (
	"bytes"

	"github.com/bdragon300/tusgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concatenation", func() {
	var srv *tusServer
	var cl *tusgo.Client
	var parts [][]byte

	// createPartial creates a partial upload, transfers the first n bytes of data to it and returns its location.
	createPartial := func(data []byte, n int) string {
		GinkgoHelper()
		u, err := cl.CreateUpload(int64(len(data)), true, nil)
		Expect(err).NotTo(HaveOccurred())
		s := newStream(cl, u)
		_, err = s.Write(data[:n])
		Expect(err).NotTo(HaveOccurred())
		return u.Location
	}

	BeforeEach(func() {
		srv = startServer(nil, nil)
		cl = srv.NewClient()
		parts = [][]byte{randomData(1000), randomData(1), randomData(5000)}
	})

	It("should concatenate the finished partial uploads", func() {
		var locations []string
		for _, p := range parts {
			locations = append(locations, createPartial(p, len(p)))
		}

		final, err := cl.ConcatenatePartialUploads(locations, map[string]string{"k": "v"})

		Expect(err).NotTo(HaveOccurred())
		expected := bytes.Join(parts, nil)
		Expect(srv.StoredData(final)).To(Equal(expected))
		got, err := cl.GetUpload(final)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.RemoteOffset).To(BeEquivalentTo(len(expected)))
		Expect(got.RemoteSize).To(BeEquivalentTo(len(expected)))
		Expect(got.Partial).To(BeFalse())
		Expect(got.Metadata).To(Equal(map[string]string{"k": "v"}))
	})

	It("should return error if a partial upload is unfinished, since tusd lacks concatenation-unfinished", func() {
		locations := []string{createPartial(parts[0], len(parts[0])), createPartial(parts[2], 10)}

		_, err := cl.ConcatenatePartialUploads(locations, nil)

		Expect(err).To(MatchError(tusgo.ErrUnexpectedResponse))
	})

	It("should refuse to write to the final upload", func() {
		partial := createPartial(parts[0], len(parts[0]))

		final, err := cl.ConcatenatePartialUploads([]string{partial}, nil)
		Expect(err).NotTo(HaveOccurred())

		u := tusgo.Upload{Location: final, RemoteSize: int64(len(parts[0])) * 2, RemoteOffset: int64(len(parts[0]))}
		_, err = newStream(cl, u).Write([]byte("extra"))

		Expect(err).To(MatchError(tusgo.ErrCannotUpload))
	})
})
