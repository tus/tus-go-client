package checksum

import (
	"bytes"
	"io"
	"net/http"
)

// DeferTrailerReader is an io.Reader that reads a request body and fills in the HTTP trailers as soon as the body
// has been drained. It suits the cases where the trailer values are not known until the whole body has been read, for
// example to send the checksum of a large body without staging it in an intermediate buffer.
type DeferTrailerReader struct {
	body    io.Reader
	readers map[string]io.Reader
	request *http.Request
}

// NewDeferTrailerReader returns a new DeferTrailerReader that reads the request body from body and takes the value
// of every trailer from its reader in the trailers map. It registers the trailer names in request, so that they are
// announced in the Trailer header of the request being sent.
func NewDeferTrailerReader(body io.Reader, trailers map[string]io.Reader, request *http.Request) *DeferTrailerReader {
	if request.Trailer == nil {
		request.Trailer = make(http.Header)
	}
	// Fill out trailers with nils in order to make http.Request add a `Trailer: ` header to a request
	for k := range trailers {
		request.Trailer[k] = nil
	}

	return &DeferTrailerReader{
		body:    body,
		readers: trailers,
		request: request,
	}
}

// Read reads up to len(p) bytes of the request body into p. Once the body has been drained, it reads every trailer
// value from its reader and assigns it to the request.
// It returns the number of bytes read (0 <= n <= len(p)) and any error encountered, which is io.EOF once the whole
// body has been read.
func (h DeferTrailerReader) Read(p []byte) (n int, err error) {
	n, err = h.body.Read(p)
	if err == io.EOF {
		buf := bytes.NewBuffer(make([]byte, 0))
		for k, r := range h.readers {
			buf.Reset()
			if _, e := buf.ReadFrom(r); e != nil && e != io.EOF {
				return n, e
			}
			h.request.Trailer.Set(k, buf.String())
		}
	}
	return
}
