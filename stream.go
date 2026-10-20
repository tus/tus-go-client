package tusgo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bdragon300/tusgo/v2/checksum"
)

// NewStream returns a new Stream initialized with the given Upload object.
func NewStream(client *Client, upload Upload) (*Stream, error) {
	if client == nil {
		panic("client is nil")
	}
	loc, err := url.Parse(upload.Location)
	if err != nil {
		return nil, fmt.Errorf("parse upload location %q: %v", upload.Location, err)
	}
	loc = client.endpoint.ResolveReference(loc)

	return &Stream{
		client:   client,
		location: loc,

		uploadStateOffset:  upload.RemoteOffset,
		uploadSize:         upload.RemoteSize,
		uploadStateExpires: upload.Expires,
	}, nil
}

// NewStreamWithChecksum returns a new Stream initialized with the given Upload object and with enabled
// checksum verification using the specified algorithm name (which is normalized, removing all non-alphanumeric
// characters and converting to lowercase, see [checksum.AlgorithmList.Get]).
// The returned stream calculates and attaches a checksum of the transferred data to every request, so the server
// can verify its integrity.
//
// The server must advertise the "checksum" or "checksum-trailer" extensions.
//
// The algorithms supported by the library are listed in [checksum.Algorithms].
func NewStreamWithChecksum(client *Client, upload Upload, algorithm string) (*Stream, error) {
	rs, err := NewStream(client, upload)
	if err != nil {
		return nil, err
	}

	f, ok := checksum.Algorithms.Get(algorithm)
	if !ok {
		return nil, fmt.Errorf("algorithm %q is not supported by library", algorithm)
	}

	rs.checksumHash = f()
	rs.rawChecksumHashName = algorithm

	return rs, nil
}

// Stream is an unbuffered io.Writer that transfers data to upload on the server.
//
// Stream also keeps the client data offset in sync with the server offset while the data is transferred.
// Client offset is available in [Tell] method. When offsets become out of sync
// (for example, if we missed the last server response), the server rejects the next request with error
// [ErrOffsetsNotSynced]. In this case, call Sync method to sync the client offset with the server offset and try again.
//
// This is a low-level API that transfers data in a single request. For more convenient way to transfer data in chunks,
// use [ChunkedWriter].
//
// Stream does not implement io.ReaderFrom intentionally to avoid unnecessary side effects when using utils like io.Copy.
// The UploadFrom method acts exactly the ReadFrom would.
//
// Common errors returned by the Stream are:
//
//   - [ErrOffsetsNotSynced] -- the local offset and the server offset differ, call Sync to adopt the server offset and
//     try again
//   - [ErrTooLarge] -- the data exceeds the upload size
//   - [ErrIncompleteWrite] -- the server has accepted only a part of the data, call the same method again with the
//     remaining data to continue the transfer
//   - [ErrCannotUpload] -- the data cannot be written to an existing upload, typically because the upload is already
//     full, is a final concatenated upload, or does not accept the data for another reason
//   - [ErrChecksumMismatch] -- the server has detected a data corruption, if the checksum verification is used
//   - [ErrUnsupportedFeature] -- the requested action needs a TUS extension that the server does not support
//   - [ErrProtocol] -- an otherwise successful server response contains malformed data, e.g. a missing required HTTP header
//   - [ErrUnexpectedResponse] -- the server has responded with an unexpected status code, which may indicate that
//     the server does not support the TUS protocol, or the response is originated by a proxy server or CDN.
type Stream struct {
	checksumHash        hash.Hash
	rawChecksumHashName string
	client              *Client

	// Current upload state, changed as the data is transferred
	location           *url.URL
	uploadSize         int64
	uploadStateOffset  int64
	uploadStateExpires *time.Time
}

// UploadFrom reads data from r and sends it to the server in a single TUS request without buffering, starting at the
// current server offset. It returns the number of bytes has been read from r.
//
// If the server has accepted only a part of the data, UploadFrom returns the bytes read from r and [ErrIncompleteWrite].
// In this case, the data that been read from r but not accepted by the server is lost.
// So, to continue the transfer, call UploadFrom again with a new reader (or rewind r if possible) that provides the
// remaining data. Or use [ChunkedWriter] to transfer the data in chunks.
//
// If data is read from r exceeds the upload size, UploadFrom returns [ErrTooLarge] and the number of bytes read from r.
//
// If Stream was created with checksum verification enabled, the server must support the "checksum-trailer" extension.
func (rs *Stream) UploadFrom(r io.Reader) (int64, error) {
	counterRd := &counterReader{Rd: r}
	offset := rs.uploadStateOffset
	newOffset, expires, resp, err := rs.doRequest(http.MethodPatch, rs.location.String(), nil, counterRd, offset, nil)
	defer closeResponseBody(resp)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}

	advance := newOffset - offset
	switch {
	case advance < 0:
		return counterRd.BytesRead, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has gone backwards, offset=%d, newOffset=%d", offset, newOffset))
	case advance > counterRd.BytesRead:
		return counterRd.BytesRead, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has been advanced too far: offset=%d, advance=%d, bytesRead=%d", offset, advance, counterRd.BytesRead))
	}

	rs.uploadStateOffset = newOffset
	rs.uploadStateExpires = expires

	if advance < counterRd.BytesRead {
		return counterRd.BytesRead, newTusError(ErrIncompleteWrite, resp, fmt.Errorf("server did not accept some data, accepted=%d, offset=%d", advance, offset))
	}

	return counterRd.BytesRead, nil
}

// Write sends p to the server in a single TUS request, starting at the current server offset.
// It returns the number of bytes the server has accepted.
//
// If the server has accepted only a part of p, Write returns the accepted number of bytes and [ErrIncompleteWrite].
// In this case, call Write again with the rest of p to continue the transfer.
//
// If p exceeds the upload size, Write returns [ErrTooLarge] and the number of bytes accepted by the server.
//
// If Stream was created with checksum verification enabled, the server must support the "checksum" extension.
func (rs *Stream) Write(p []byte) (int, error) {
	offset := rs.uploadStateOffset
	newOffset, expires, resp, err := rs.doRequest(http.MethodPatch, rs.location.String(), p, nil, offset, nil)
	defer closeResponseBody(resp)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}

	advance := int(newOffset - offset)
	switch {
	case advance < 0:
		return 0, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has gone backwards, offset=%d, newOffset=%d", offset, newOffset))
	case advance > len(p):
		return 0, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has been advanced too far: offset=%d, advance=%d, size=%d", offset, advance, len(p)))
	}

	rs.uploadStateOffset = newOffset
	rs.uploadStateExpires = expires

	if advance < len(p) {
		return advance, newTusError(ErrIncompleteWrite, resp, fmt.Errorf("server did not accept some data, accepted=%d, offset=%d", advance, offset))
	}
	return advance, nil
}

// Sync requests the current offset from the server and adopts it as the stream offset. Returns the delta showing
// where the stream offset has been moved: positive is forward, negative is backward, zero means the stream offset was
// already in sync with the server offset.
//
// Call it before starting the transfer and if the stream has returned [ErrOffsetsNotSynced].
func (rs *Stream) Sync() (int64, error) {
	u, err := rs.client.GetUpload(rs.location.String())
	if err != nil {
		return 0, fmt.Errorf("get upload: %w", err)
	}

	oldOffset := rs.uploadStateOffset
	rs.uploadStateOffset = u.RemoteOffset
	rs.uploadSize = u.RemoteSize
	rs.uploadStateExpires = u.Expires

	return rs.uploadStateOffset - oldOffset, nil
}

// Seek moves the current stream offset to the given position, interpreted according to whence:
// io.SeekStart means relative to the start of the upload, io.SeekCurrent to the current offset and io.SeekEnd to the
// end of the upload. It returns the new offset.
//
// Note that this only sets the local offset variable without affecting the offset on the server.
// Before the first transfer they must be equal, otherwise server rejects the requests with [ErrOffsetsNotSynced].
// As alternative, call Sync to adopt the server offset automatically.
func (rs *Stream) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = rs.uploadStateOffset + offset
	case io.SeekEnd:
		if rs.uploadSize == SizeUnknown {
			return rs.uploadStateOffset, fmt.Errorf("cannot seek from end of upload with unknown size")
		}
		newOffset = rs.uploadSize + offset
	default:
		return rs.uploadStateOffset, fmt.Errorf("unknown whence value: %d", whence)
	}
	if rs.uploadSize != SizeUnknown && newOffset >= rs.uploadSize {
		newOffset = rs.uploadSize
	}
	if newOffset < 0 {
		return newOffset, fmt.Errorf("offset %d is negative", newOffset)
	}
	rs.uploadStateOffset = newOffset
	return newOffset, nil
}

// Tell returns the current stream offset.
func (rs *Stream) Tell() int64 {
	return rs.uploadStateOffset
}

// Size returns the size of the upload.
//
// If the upload was created with deferred size, the value is SizeUnknown until the size is declared with DeclareSize.
func (rs *Stream) Size() int64 {
	return rs.uploadSize
}

// DeclareSize sets the total size of an upload created with a deferred size, that is, with remoteSize set to
// [SizeUnknown] in [Client.CreateUpload]. On success, [Stream.Size] returns size.
//
// The size can be declared only once per upload: the server rejects any later attempt to change it, even from
// another Stream. DeclareSize sends a request without data, so the stream offset does not change.
//
// The server must support the "creation-defer-length" extension.
func (rs *Stream) DeclareSize(size int64) error {
	if size < 0 {
		return fmt.Errorf("size is negative %d", size)
	}
	if err := rs.client.ensureExtension("creation-defer-length"); err != nil {
		return err
	}
	headers := map[string]string{"Upload-Length": strconv.FormatInt(size, 10)}
	offset := rs.uploadStateOffset
	newOffset, expires, resp, err := rs.doRequest(http.MethodPatch, rs.location.String(), nil, nil, offset, headers)
	defer closeResponseBody(resp)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	if newOffset != offset {
		return newTusError(ErrProtocol, resp, fmt.Errorf("server offset has changed after declaring size: offset=%d, newOffset=%d", offset, newOffset))
	}
	rs.uploadStateExpires = expires
	rs.uploadSize = size

	return nil
}

func (rs *Stream) doRequest(method, requestURL string, chunk []byte, stream io.Reader, offset int64, extraHeaders map[string]string) (int64, *time.Time, *http.Response, error) {
	// Perform actions that can generate an error before invoking a reader
	if rs.checksumHash != nil {
		if stream != nil {
			if err := rs.client.ensureExtension("checksum-trailer"); err != nil {
				return offset, nil, nil, err
			}
		} else {
			if err := rs.client.ensureExtension("checksum"); err != nil {
				return offset, nil, nil, err
			}
		}
	}
	req, err := rs.client.createRequest(method, requestURL)
	if err != nil {
		return offset, nil, nil, fmt.Errorf("create request: %w", err)
	}

	if rs.checksumHash != nil {
		rs.checksumHash.Reset()
		if stream != nil {
			trailers := map[string]io.Reader{"Upload-Checksum": checksum.NewHashBase64ReadWriter(rs.checksumHash, rs.rawChecksumHashName+" ")}
			stream = checksum.NewDeferTrailerReader(io.TeeReader(stream, rs.checksumHash), trailers, req)
		} else if len(chunk) > 0 {
			rs.checksumHash.Write(chunk)
			sum := rs.checksumHash.Sum(make([]byte, 0))
			req.Header.Set("Upload-Checksum", fmt.Sprintf("%s %s", rs.rawChecksumHashName, base64.StdEncoding.EncodeToString(sum)))
		}
	}

	if stream == nil {
		if len(chunk) == 0 {
			req.Body = http.NoBody
		} else {
			req.Body = io.NopCloser(bytes.NewReader(chunk))
		}
		req.ContentLength = int64(len(chunk))
	} else {
		req.Body = io.NopCloser(stream)
		req.ContentLength = -1 // Request body size is unknown in streamed mode. See https://pkg.go.dev/net/http#Request.ContentLength
	}
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))

	if len(extraHeaders) > 0 {
		for k, v := range extraHeaders {
			if v == "" {
				req.Header.Del(k)
			} else {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := rs.client.tusRequest(req)
	if err != nil {
		return offset, nil, resp, fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated: // For "Creation With Upload" feature
		if method != http.MethodPost {
			return offset, nil, resp, newTusError(ErrUnexpectedResponse, resp, nil)
		}
		fallthrough
	case http.StatusNoContent:
		var newExpires *time.Time
		var newOffset int64

		if newOffset, err = strconv.ParseInt(resp.Header.Get("Upload-Offset"), 10, 64); err != nil {
			return newOffset, nil, resp, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Offset header %q: %w", resp.Header.Get("Upload-Offset"), err))
		}
		// Upload-Expires is optional, its absence means the upload does not expire
		if v := resp.Header.Get("Upload-Expires"); v != "" {
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return newOffset, nil, resp, newTusError(ErrProtocol, resp, fmt.Errorf("parse RFC1123 datetime in Upload-Expires header %q: %w", v, err))
			}
			newExpires = &t
		}

		return newOffset, newExpires, resp, nil
	case http.StatusConflict:
		return offset, nil, resp, newTusError(ErrOffsetsNotSynced, resp, nil)
	case http.StatusForbidden:
		return offset, nil, resp, newTusError(ErrCannotUpload, resp, nil)
	case http.StatusNotFound, http.StatusGone:
		return offset, nil, resp, newTusError(ErrUploadDoesNotExist, resp, nil)
	case http.StatusRequestEntityTooLarge:
		return offset, nil, resp, newTusError(ErrTooLarge, resp, nil)
	case 460: // Non-standard HTTP code '460 Checksum Mismatch'
		if rs.checksumHash != nil {
			return offset, nil, resp, newTusError(ErrChecksumMismatch, resp, nil)
		}
		// If checksum verification is not used, treat it as an unexpected response
	}

	return offset, nil, resp, newTusError(ErrUnexpectedResponse, resp, nil)
}
