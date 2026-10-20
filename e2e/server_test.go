package e2e

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"

	"github.com/bdragon300/tusgo/v2"
	"golang.org/x/exp/slog"

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

	tusHandler := http.StripPrefix(basePath, h)
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
	cl := tusgo.NewClient(s.BaseURL)
	cl.Client = s.Client()
	return cl
}

// StoredData returns the data the server has stored for the upload at the given location.
func (s *tusServer) StoredData(location string) []byte {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join(s.Dir, path.Base(location)))
	Expect(err).NotTo(HaveOccurred())
	return data
}

// newStream returns a new stream transferring the data to the upload in a single request per Write call.
func newStream(cl *tusgo.Client, u tusgo.Upload) *tusgo.Stream {
	GinkgoHelper()
	s, err := tusgo.NewStream(cl, u)
	Expect(err).NotTo(HaveOccurred())
	return s
}

// newChunkedWriter returns a new stream and a writer on top of it transferring the data in chunks of chunkSize bytes.
func newChunkedWriter(cl *tusgo.Client, u tusgo.Upload, chunkSize int) (*tusgo.Stream, *tusgo.ChunkedWriter) {
	GinkgoHelper()
	s := newStream(cl, u)
	return s, tusgo.NewChunkedWriter(s, chunkSize)
}

// readerOnly hides all methods of the underlying reader except Read, such as io.WriterTo, so that
// io.Copy(dst, readerOnly{r}) calls dst.ReadFrom.
type readerOnly struct {
	io.Reader
}

// errResponse returns the response kept by the tusgo.TusError found in err chain, or nil.
func errResponse(err error) *http.Response {
	var te tusgo.TusError
	if errors.As(err, &te) {
		return te.Response()
	}
	return nil
}

// randomData returns n random bytes.
func randomData(n int) []byte {
	GinkgoHelper()
	data := make([]byte, n)
	_, err := rand.Read(data)
	Expect(err).NotTo(HaveOccurred())
	return data
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

// chain combines the middlewares, the first one is the outermost.
func chain(mws ...middleware) middleware {
	return func(next http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
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

// replyStatus replies with the given status code without passing the request to tusd.
func replyStatus(code int) fault {
	return func(_ http.Handler, w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Tus-Resumable", "1.0.0")
		w.WriteHeader(code)
	}
}

// dropResponse passes the request to tusd, so that the data is stored, but closes the connection instead of sending the
// response. The client gets a network error and does not learn the new offset.
func dropResponse() fault {
	return func(next http.Handler, w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(httptest.NewRecorder(), r)
		// The handler runs in a server goroutine, where a failed assertion cannot stop the spec, so just panic,
		// which makes the server close the connection as well
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		_ = conn.Close()
	}
}
