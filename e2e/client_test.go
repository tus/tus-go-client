package e2e

import (
	"bytes"
	"io"
	"net/url"
	"path"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tus/tusd/v2/pkg/handler"
)

var _ = Describe("Client", func() {
	Context("capabilities", func() {
		It("should obtain the server capabilities", func() {
			srv := startServer(func(cfg *handler.Config) { cfg.MaxSize = 1024 }, nil)
			cl := srv.NewClient()

			_, err := cl.UpdateCapabilities()

			Expect(err).NotTo(HaveOccurred())
			Expect(cl.Capabilities).To(Equal(&tusgo.ServerCapabilities{
				Extensions: []string{
					"creation", "creation-with-upload", "termination", "concatenation", "creation-defer-length",
				},
				MaxSize:          1024,
				ProtocolVersions: []string{"1.0.0"},
			}))
		})

		It("should refuse to delete an upload if the server does not support termination", func() {
			srv := startServer(func(cfg *handler.Config) { cfg.DisableTermination = true }, nil)
			cl := srv.NewClient()
			u := tusgo.Upload{}
			_, err := cl.CreateUpload(&u, 10, false, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = cl.DeleteUpload(u)

			Expect(err).To(MatchError(tusgo.ErrUnsupportedFeature))
		})

		It("should refuse to concatenate uploads if the server does not support concatenation", func() {
			srv := startServer(func(cfg *handler.Config) { cfg.DisableConcatenation = true }, nil)
			cl := srv.NewClient()
			u := tusgo.Upload{}
			_, err := cl.CreateUpload(&u, 10, false, nil)
			Expect(err).NotTo(HaveOccurred())
			u.Partial = true

			_, err = cl.ConcatenateUploads(&tusgo.Upload{}, []tusgo.Upload{u}, nil)

			Expect(err).To(MatchError(tusgo.ErrUnsupportedFeature))
		})
	})

	Context("upload lifecycle", func() {
		var srv *tusServer
		var cl *tusgo.Client

		BeforeEach(func() {
			srv = startServer(func(cfg *handler.Config) { cfg.MaxSize = 1024 * 1024 }, nil)
			cl = srv.NewClient()
		})

		It("should create an upload and get it back", func() {
			meta := map[string]string{"filename": "file.txt", "empty": "", "comma": "a, b"}
			created := tusgo.Upload{}

			_, err := cl.CreateUpload(&created, 1024, false, meta)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.Location).To(HavePrefix(srv.BaseURL.String()))
			got := tusgo.Upload{}
			_, err = cl.GetUpload(&got, created.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(tusgo.Upload{
				Location:     created.Location,
				RemoteSize:   1024,
				RemoteOffset: 0,
				Metadata:     meta,
			}))
		})

		It("should create a partial upload", func() {
			created := tusgo.Upload{}

			_, err := cl.CreateUpload(&created, 1024, true, nil)

			Expect(err).NotTo(HaveOccurred())
			got := tusgo.Upload{}
			_, err = cl.GetUpload(&got, created.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Partial).To(BeTrue())
		})

		It("should get an upload by the location relative to BaseURL", func() {
			created := tusgo.Upload{}
			_, err := cl.CreateUpload(&created, 1024, false, nil)
			Expect(err).NotTo(HaveOccurred())

			loc, err := url.Parse(created.Location)
			Expect(err).NotTo(HaveOccurred())

			got := tusgo.Upload{}
			_, err = cl.GetUpload(&got, path.Base(loc.Path))

			Expect(err).NotTo(HaveOccurred())
			Expect(got.Location).To(Equal(path.Base(loc.Path)))
		})

		It("should return error if the upload size exceeds the server limit", func() {
			_, err := cl.CreateUpload(&tusgo.Upload{}, 1024*1024+1, false, nil)

			Expect(err).To(MatchError(tusgo.ErrUploadTooLarge))
		})

		It("should return error when getting a non-existent upload", func() {
			_, err := cl.GetUpload(&tusgo.Upload{}, srv.BaseURL.String()+"nonexistent")

			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
		})

		It("should delete an upload", func() {
			u := tusgo.Upload{}
			_, err := cl.CreateUpload(&u, 1024, false, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = cl.DeleteUpload(u)

			Expect(err).NotTo(HaveOccurred())
			_, err = cl.GetUpload(&tusgo.Upload{}, u.Location)
			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
			_, err = cl.DeleteUpload(u)
			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
		})
	})

	Context("creation with upload", func() {
		var srv *tusServer
		var cl *tusgo.Client

		BeforeEach(func() {
			srv = startServer(nil, nil)
			cl = srv.NewClient()
		})

		It("should create an upload with all its data", func() {
			data := randomData(1000)
			u := tusgo.Upload{}

			n, _, err := cl.CreateUploadWithData(&u, data, int64(len(data)), false, map[string]string{"k": "v"})

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(len(data)))
			Expect(u.RemoteOffset).To(BeEquivalentTo(len(data)))
			Expect(srv.StoredData(u)).To(Equal(data))
			got := tusgo.Upload{}
			_, err = cl.GetUpload(&got, u.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.RemoteOffset).To(BeEquivalentTo(len(data)))
			Expect(got.Metadata).To(Equal(map[string]string{"k": "v"}))
		})

		It("should create an upload with the beginning of its data and continue with a stream", func() {
			data := randomData(1000)
			u := tusgo.Upload{}

			_, _, err := cl.CreateUploadWithData(&u, data[:300], int64(len(data)), false, nil)
			Expect(err).NotTo(HaveOccurred())
			s := tusgo.NewUploadStream(cl, &u)
			_, err = io.Copy(s, bytes.NewReader(data[s.Tell():]))

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(u)).To(Equal(data))
		})
	})
})
