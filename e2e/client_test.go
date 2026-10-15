package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
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

			u, err := cl.CreateUpload(10, false, nil)
			Expect(err).NotTo(HaveOccurred())

			err = cl.DeleteUpload(u.Location)

			Expect(err).To(MatchError(tusgo.ErrUnsupportedFeature))
		})

		It("should refuse to concatenate uploads if the server does not support concatenation", func() {
			srv := startServer(func(cfg *handler.Config) { cfg.DisableConcatenation = true }, nil)
			cl := srv.NewClient()
			u, err := cl.CreateUpload(10, false, nil)
			Expect(err).NotTo(HaveOccurred())
			u.Partial = true

			_, err = cl.ConcatenateUploads([]tusgo.Upload{u}, nil)

			Expect(err).To(MatchError(tusgo.ErrUnsupportedFeature))
		})
	})

	Context("sending requests", func() {
		It("should use http.DefaultClient if neither Client nor DoRequest is set", func() {
			srv := startServer(nil, nil)
			cl := tusgo.NewClient(srv.BaseURL)

			_, err := cl.UpdateCapabilities()

			Expect(err).NotTo(HaveOccurred())
			Expect(cl.Capabilities.ProtocolVersions).To(Equal([]string{"1.0.0"}))
		})

		It("should send all requests, including the stream ones, via DoRequest", func() {
			srv := startServer(nil, nil)
			cl := srv.NewClient()
			var methods []string
			cl.DoRequest = func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				return srv.Client().Do(req)
			}

			u, err := cl.CreateUpload(1000, false, nil)
			Expect(err).NotTo(HaveOccurred())
			s := tusgo.NewUploadStream(cl, &u)
			Expect(s.Sync()).To(Succeed())
			_, err = s.Write(randomData(1000))

			Expect(err).NotTo(HaveOccurred())
			Expect(methods).To(Equal([]string{http.MethodOptions, http.MethodPost, http.MethodHead, http.MethodPatch}))
		})

		It("should cancel the transfer by the request context set in DoRequest", func() {
			srv := startServer(nil, nil)
			cl := srv.NewClient()
			u, err := cl.CreateUpload(1000, false, nil)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			streamCl := srv.NewClient()
			streamCl.DoRequest = func(req *http.Request) (*http.Response, error) {
				return srv.Client().Do(req.WithContext(ctx))
			}
			_, err = tusgo.NewUploadStream(streamCl, &u).Write(randomData(1000))

			Expect(err).To(MatchError(context.Canceled))
			got, err := cl.GetUpload(u.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.RemoteOffset).To(BeEquivalentTo(0))
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

			created, err := cl.CreateUpload(1024, false, meta)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.Location).To(HavePrefix(srv.BaseURL.String()))
			got, err := cl.GetUpload(created.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(tusgo.Upload{
				Location:     created.Location,
				RemoteSize:   1024,
				RemoteOffset: 0,
				Metadata:     meta,
			}))
		})

		It("should create a partial upload", func() {
			created, err := cl.CreateUpload(1024, true, nil)

			Expect(err).NotTo(HaveOccurred())
			got, err := cl.GetUpload(created.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Partial).To(BeTrue())
		})

		It("should get an upload by the location relative to the client endpoint", func() {
			created, err := cl.CreateUpload(1024, false, nil)
			Expect(err).NotTo(HaveOccurred())

			loc, err := url.Parse(created.Location)
			Expect(err).NotTo(HaveOccurred())

			got, err := cl.GetUpload(path.Base(loc.Path))

			Expect(err).NotTo(HaveOccurred())
			Expect(got.Location).To(Equal(path.Base(loc.Path)))
		})

		It("should return error if the upload size exceeds the server limit", func() {
			_, err := cl.CreateUpload(1024*1024+1, false, nil)

			Expect(err).To(MatchError(tusgo.ErrUploadTooLarge))
		})

		It("should return error when getting a non-existent upload", func() {
			_, err := cl.GetUpload(srv.BaseURL.String() + "nonexistent")

			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
		})

		It("should delete an upload", func() {
			u, err := cl.CreateUpload(1024, false, nil)
			Expect(err).NotTo(HaveOccurred())

			err = cl.DeleteUpload(u.Location)

			Expect(err).NotTo(HaveOccurred())
			_, err = cl.GetUpload(u.Location)
			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
			err = cl.DeleteUpload(u.Location)
			Expect(err).To(MatchError(tusgo.ErrUploadDoesNotExist))
			Expect(errResponse(err)).To(And(
				HaveHTTPStatus(http.StatusNotFound),
				HaveHTTPBody(ContainSubstring("ERR_UPLOAD_NOT_FOUND")),
			))
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

			n, u, err := cl.CreateUploadWithData(data, int64(len(data)), false, map[string]string{"k": "v"})

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(BeEquivalentTo(len(data)))
			Expect(u.RemoteOffset).To(BeEquivalentTo(len(data)))
			Expect(srv.StoredData(u)).To(Equal(data))
			got, err := cl.GetUpload(u.Location)
			Expect(err).NotTo(HaveOccurred())
			Expect(got.RemoteOffset).To(BeEquivalentTo(len(data)))
			Expect(got.Metadata).To(Equal(map[string]string{"k": "v"}))
		})

		It("should create an upload with the beginning of its data and continue with a stream", func() {
			data := randomData(1000)

			_, u, err := cl.CreateUploadWithData(data[:300], int64(len(data)), false, nil)
			Expect(err).NotTo(HaveOccurred())
			s := tusgo.NewUploadStream(cl, &u)
			_, err = io.Copy(s, bytes.NewReader(data[s.Tell():]))

			Expect(err).NotTo(HaveOccurred())
			Expect(srv.StoredData(u)).To(Equal(data))
		})
	})
})
