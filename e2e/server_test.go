package e2e

import (
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/exp/slog"

	"github.com/bdragon300/tusgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
)

const basePath = "/files/"

// tusServer is a tusd instance serving the uploads from a temporary directory.
type tusServer struct {
	*httptest.Server
	// Dir is the directory where tusd stores the uploads.
	Dir string
	// BaseURL is the URL the uploads are created at.
	BaseURL *url.URL
}

// middleware wraps the tusd handler, e.g. to inject a fault.
type middleware func(next http.Handler) http.Handler

// startServer starts a new tusd instance, which is shut down after the current spec. The configure function, if not
// nil, may adjust the tusd config, and the mw, if not nil, wraps the tusd handler.
func startServer(configure func(cfg *handler.Config), mw middleware) *tusServer {
	GinkgoHelper()

	dir := GinkgoT().TempDir()
	composer := handler.NewStoreComposer()
	filestore.New(dir).UseIn(composer)
	filelocker.New(dir).UseIn(composer)

	cfg := handler.Config{
		BasePath:      basePath,
		StoreComposer: composer,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if configure != nil {
		configure(&cfg)
	}
	h, err := handler.NewHandler(cfg)
	Expect(err).NotTo(HaveOccurred())

	var tusHandler http.Handler = http.StripPrefix(basePath, h)
	if mw != nil {
		tusHandler = mw(tusHandler)
	}
	mux := http.NewServeMux()
	mux.Handle(basePath, tusHandler)
	srv := httptest.NewServer(mux)
	DeferCleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL + basePath)
	Expect(err).NotTo(HaveOccurred())
	return &tusServer{Server: srv, Dir: dir, BaseURL: baseURL}
}

// NewClient returns a new client working with the server.
func (s *tusServer) NewClient() *tusgo.Client {
	return tusgo.NewClient(s.Client(), s.BaseURL)
}

// StoredData returns the data the server has stored for the upload.
func (s *tusServer) StoredData(u tusgo.Upload) []byte {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join(s.Dir, path.Base(u.Location)))
	Expect(err).NotTo(HaveOccurred())
	return data
}

// randomData returns n random bytes.
func randomData(n int) []byte {
	GinkgoHelper()
	data := make([]byte, n)
	_, err := rand.Read(data)
	Expect(err).NotTo(HaveOccurred())
	return data
}

// readerOnly hides all methods of r except Read. Use it to make io.Copy call UploadStream.ReadFrom: otherwise, for
// a source implementing io.WriterTo, like bytes.Reader, io.Copy calls its WriteTo, that ends up in UploadStream.Write.
func readerOnly(r io.Reader) io.Reader {
	return struct{ io.Reader }{r}
}

// requestCounter is a middleware that counts the requests with the given method.
type requestCounter struct {
	method string
	count  atomic.Int64
}

func (rc *requestCounter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == rc.method {
			rc.count.Add(1)
		}
		next.ServeHTTP(w, r)
	})
}

// fault replaces the handling of a request, it may call next to pass the request to tusd.
type fault func(next http.Handler, w http.ResponseWriter, r *http.Request)

// injectFault returns a middleware that applies f to the n-th (counting from 1) request with the given method. All other
// requests are passed to tusd unchanged.
func injectFault(method string, n int64, f fault) middleware {
	var counter atomic.Int64
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == method && counter.Add(1) == n {
				f(next, w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// acceptOnly makes tusd see only the first n bytes of the request body, so that it accepts less data than the client
// has sent and reports the correspondingly smaller offset.
func acceptOnly(n int64) fault {
	return func(next http.Handler, w http.ResponseWriter, r *http.Request) {
		r.Body = io.NopCloser(io.LimitReader(r.Body, n))
		next.ServeHTTP(w, r)
	}
}

// dropResponse lets tusd process the request completely, but then closes the connection instead of sending the
// response, as if the connection was broken after the server has committed the data.
func dropResponse() fault {
	return func(next http.Handler, w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		next.ServeHTTP(&discardResponseWriter{header: make(http.Header)}, r)
		conn, _, err := http.NewResponseController(w).Hijack()
		Expect(err).NotTo(HaveOccurred())
		Expect(conn.Close()).To(Succeed())
	}
}

type discardResponseWriter struct {
	header http.Header
}

func (d *discardResponseWriter) Header() http.Header         { return d.header }
func (d *discardResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (d *discardResponseWriter) WriteHeader(int)             {}
