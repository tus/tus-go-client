package tusgo

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bdragon300/tusgo/checksum"
)

// NewUploadStream returns a new UploadStream that transfers the data to the existing upload on the server.
// Local offset and size of the upload are taken from the given Upload object.
func NewUploadStream(client *Client, upload Upload) (*UploadStream, error) {
	if client == nil {
		panic("client is nil")
	}
	loc, err := url.Parse(upload.Location)
	if err != nil {
		return nil, fmt.Errorf("parse upload location %q: %v", upload.Location, err)
	}
	loc = client.endpoint.ResolveReference(loc)

	const chunkSize = 2 * 1024 * 1024
	return &UploadStream{
		ChunkSize: chunkSize,
		client:    client,
		location:  loc,

		uploadStateOffset:  upload.RemoteOffset,
		uploadSize:         upload.RemoteSize,
		uploadStateExpires: upload.UploadExpired,
	}, nil
}

// NewUploadStreamWithChecksum is like [NewUploadStream], but the returned stream attaches a checksum of the
// transferred data, computed with the given hash algorithm, to every request, so the server can verify its integrity.
//
// The server must advertise the "checksum" or "checksum-trailer" extensions.
//
// The algorithms supported by the library are listed in [checksum.Algorithms].
func NewUploadStreamWithChecksum(client *Client, upload Upload, algorithm string) (*UploadStream, error) {
	us, err := NewUploadStream(client, upload)
	if err != nil {
		return nil, err
	}

	alg, ok := checksum.GetAlgorithm(algorithm)
	if !ok {
		return nil, fmt.Errorf("algorithm %q is not supported by library", algorithm)
	}

	f := checksum.Algorithms[alg]
	us.checksumHash = f()
	us.rawChecksumHashName = algorithm

	return us, nil
}

// NoChunked when assigned to UploadStream.ChunkSize, disables the chunking of the transferred data.
const NoChunked = 0

// UploadStream is a write-only stream that transfers the data of an upload to a TUS server. It implements io.Writer,
// io.ReaderFrom and io.Seeker. UploadStream is not thread-safe.
//
// UploadStream keeps the current data offset of the upload which must be in sync with the server offset all the time
// while the data is transferred. Server returns a new offset in every response, and it must be sent in the next
// request. Once they become out of sync (for example, if we missed a response), the server rejects the next request
// with error [ErrOffsetsNotSynced]. Call Sync to adopt the server offset and try again.
//
// By default, the data is transferred in chunks of ChunkSize bytes, one request per chunk, staged through an
// intermediate buffer called the dirty buffer. The stream is "dirty" while the dirty buffer holds a chunk of data
// that has not yet been confirmed by the server, and Dirty returns true.
//
// The basic working approach is following:
//
//  1. Create a fixed-size upload on the server by Client, get the Upload object representing the upload, see [Client.GetUpload]
//  2. Pass this Upload object to NewUploadStream
//  3. Transfer the data with an UploadStream. For example, use [io.Copy] to transfer the data from the reader
//  4. If transfer was interrupted, just call the same method again. If UploadStream lost sync with the server
//     offset and returned ErrOffsetsNotSynced (e.g. server response got missed on the way),
//     call Sync method and try again.
//
// Common errors returned by the UploadStream are:
//
//   - [ErrUnsupportedFeature] -- the requested action needs a TUS extension that the server does not support
//   - [ErrProtocol] -- an otherwise successful server response contains malformed data, e.g. a missing required HTTP header
//   - [ErrUnexpectedResponse] -- the server responded with an unexpected status code
//   - [ErrOffsetsNotSynced] -- the local offset and the server offset differ, call Sync to adopt the server offset and
//     try again
//   - [ErrChecksumMismatch] -- the server has detected a data corruption, if the checksum verification is used
//   - [ErrCannotUpload] -- the data cannot be written to an existing upload, typically because the upload is already
//     full, is a final concatenated upload, or does not accept the data for another reason
//   - [ErrZeroProgress] -- the server did not accept any of the data and did not return an error, which may indicate
//     that client is sending data too fast or the server has a bug or is misconfigured.
type UploadStream struct {
	// ChunkSize is the size of a chunk of data sent in a single request and the size of the dirty buffer.
	// Set it to NoChunked to disable the chunking, which also disables the use of the dirty buffer. Default is 2 MiB.
	ChunkSize int64

	// SetUploadSize enables stream to send the data size to the server with the first request.
	// Set it to true when transferring the data to uploads created without size (remoteSize has been set to SizeUnknown).
	// Before the first request, Upload.RemoteSize must be set to the actual size of the upload.
	//
	// The server must support the "creation-defer-length" extension.
	SetUploadSize bool

	// dirtyBuffer is the intermediate buffer used in chunked mode that holds a chunk of data to transfer to the server.
	// This buffer is used to make the transfer resumable, keeping the data that has not yet been
	// confirmed by the server. len(dirtyBuffer) == ChunkSize.
	//
	// Once been filled, dirtyBuffer is immutable until its contents is fully transferred to the server in one or
	// more requests.
	dirtyBuffer []byte
	// dirtyUnread is a pointer to the part of dirtyBuffer to be transferred in the next request.
	// Once the server confirms the certain amount of data in a request, dirtyUnread is advanced by that amount.
	// The len(dirtyUnread) == 0 means that the whole dirtyBuffer has been transferred and the stream is "clean".
	dirtyUnread []byte

	checksumHash        hash.Hash
	rawChecksumHashName string
	client              *Client

	// Current upload state, changed as the data is transferred
	location           *url.URL
	uploadSize         int64
	uploadStateOffset  int64
	uploadStateExpires *time.Time
}

// ReadFrom reads r until EOF or until the upload is full and transfers the data to the server, starting at
// Upload.RemoteOffset and advancing it as the server confirms the data. It returns the number of bytes read from r.
//
// How the data is transferred depends on ChunkSize. In chunked mode, r is read in chunks into a "dirty buffer" of
// ChunkSize bytes, and sent to the server, one request per chunk.
// In streaming mode (ChunkSize == NoChunked), r is passed as the body of a single request.
//
// If an error occurs in chunked mode, call ReadFrom again with the same r to resume the upload. If the failed chunk
// has not been fully transferred, its remaining data is kept in the dirty buffer and the stream becomes dirty (see Dirty).
// The next ReadFrom call sends the buffered data first and then continues reading r, so r need not be rewound.
// To discard the buffered data instead, call ForceClean.
func (us *UploadStream) ReadFrom(r io.Reader) (int64, error) {
	if err := us.validate(); err != nil {
		return 0, err
	}

	counterRd := &counterReader{Rd: r}
	if us.ChunkSize == NoChunked {
		// Streaming mode, pass the reader directly
		advance, newOffset, expires, resp, err := us.uploadData(nil, counterRd, us.uploadStateOffset, nil)
		defer closeResponseBody(resp)
		if err != nil {
			return counterRd.BytesRead, err
		}

		us.uploadStateOffset = newOffset
		us.uploadStateExpires = expires

		if advance == 0 && counterRd.BytesRead > 0 {
			return counterRd.BytesRead, newTusError(ErrZeroProgress, resp, fmt.Errorf("offset=%d", us.uploadStateOffset))
		}
		if int64(advance) < counterRd.BytesRead {
			// We cannot retry the transfer in ReadFrom, because in general, the r is not seekable, so return an error to the caller.
			// TODO: move the streamed mode to a separate Stream (#26)
			return counterRd.BytesRead, io.ErrShortWrite
		}
		return counterRd.BytesRead, nil
	}

	// Chunked mode
	_, err := us.uploadChunked(counterRd)
	return counterRd.BytesRead, err
}

// Write transfers p, starting at the current offset (see Tell method), which is kept up to date while the transfer goes on.
// It returns the number of bytes the server has accepted.
//
// In chunked mode the data is copied to the dirty buffer chunk by chunk, but, unlike ReadFrom, the stream is always
// left "clean" afterward, whether the transfer has succeeded or not, because p can always be passed again.
// A dirty buffer left over from an earlier call is discarded rather than transferred.
//
// In streaming mode (ChunkSize == NoChunked), the data is piped directly into the request body, and the stream
// never becomes "dirty" (the data that can possibly remain in dirty buffer is ignored).
//
// If p does not fit into the space left in the upload, as much data as fits is transferred and io.ErrShortWrite is
// returned.
func (us *UploadStream) Write(p []byte) (int, error) {
	if err := us.validate(); err != nil {
		return 0, err
	}
	defer func() { us.ForceClean() }() // Always mark stream as clean, since p is seekable

	if us.ChunkSize == NoChunked {
		// Streaming mode, pass the reader directly
		advance, newOffset, expires, resp, err := us.uploadData(nil, bytes.NewReader(p), us.uploadStateOffset, nil)
		defer closeResponseBody(resp)
		if err != nil {
			return advance, err
		}

		us.uploadStateOffset = newOffset
		us.uploadStateExpires = expires

		if advance < len(p) && newOffset == us.uploadSize {
			return advance, io.ErrShortWrite // Upload is full, but there is still some data left in p.
		} else if advance == 0 {
			return 0, newTusError(ErrZeroProgress, resp, fmt.Errorf("offset=%d", us.uploadStateOffset))
		}

		return advance, nil
	}

	// Chunked mode
	n, err := us.uploadChunked(bytes.NewReader(p))
	if err != nil {
		return int(n), err
	}
	if n < int64(len(p)) && us.uploadStateOffset == us.uploadSize {
		return int(n), io.ErrShortWrite // Upload is full, but there is still some data left in p.
	}

	return int(n), nil
}

// Sync requests the current offset from the server and adopts it as the stream offset.
//
// Call it before starting a transfer or after the stream has returned ErrOffsetsNotSynced.
func (us *UploadStream) Sync() error {
	u, err := us.client.GetUpload(us.location.String())
	if err != nil {
		return fmt.Errorf("get upload: %w", err)
	}

	us.uploadStateOffset = u.RemoteOffset

	return nil
}

// Seek moves the current stream offset to the given position, interpreted according to whence:
// io.SeekStart means relative to the start of the upload, io.SeekCurrent to the current offset and io.SeekEnd to the
// end of the upload. It returns the new offset.
//
// Note that this only moves the local offset: the server keeps its own one, and it may reject the requests made at a
// position it does not expect. Use Sync to adopt the server offset instead.
func (us *UploadStream) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64

	if us.uploadSize == SizeUnknown {
		return us.uploadStateOffset, fmt.Errorf("cannot seek in an upload with unknown size")
	}

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = us.uploadStateOffset + offset
	case io.SeekEnd:
		newOffset = us.uploadSize + offset
	default:
		panic(fmt.Sprintf("unknown whence value: %d", whence))
	}
	if newOffset >= us.uploadSize {
		newOffset = us.uploadSize
	}
	if newOffset < 0 {
		return newOffset, fmt.Errorf("offset %d is negative", newOffset)
	}
	us.uploadStateOffset = newOffset
	return newOffset, nil
}

// Tell returns the current local stream offset. It may not be in sync with the server offset, in this case
// the uploading methods return [ErrOffsetsNotSynced] and Sync must be called to adopt the server offset.
func (us *UploadStream) Tell() int64 {
	return us.uploadStateOffset
}

// Len returns the size of the upload.
func (us *UploadStream) Len() int64 {
	return us.uploadSize
}

// Dirty reports whether the stream is "dirty", that is, whether its dirty buffer holds a chunk of data that has
// failed to be transferred to the server.
func (us *UploadStream) Dirty() bool {
	return len(us.dirtyUnread) > 0
}

// ForceClean discards the contents of the dirty buffer, making the stream "clean". The data that has not been
// transferred is lost.
func (us *UploadStream) ForceClean() {
	us.dirtyUnread = nil
}

func (us *UploadStream) uploadChunked(r io.Reader) (int64, error) {
	var uploaded int64

	// Preliminary limit the reader to read only the space left in the upload
	remoteFreeSpace := us.uploadSize - us.uploadStateOffset
	r = io.LimitReader(r, remoteFreeSpace)
	us.setupDirtyBuffer()
	for us.uploadStateOffset < us.uploadSize {
		if !us.Dirty() {
			n, err := io.ReadFull(r, us.dirtyBuffer)
			switch {
			case errors.Is(err, io.EOF): // Reader is empty
				return uploaded, nil
			case errors.Is(err, io.ErrUnexpectedEOF): // Reader has ended early, upload the last chunk
			case err != nil:
				return uploaded, fmt.Errorf("read: %w", err)
			}
			us.dirtyUnread = us.dirtyBuffer[:n]
		}

		advance, newOffset, expires, resp, err := us.uploadData(us.dirtyUnread, nil, us.uploadStateOffset, nil)
		if err != nil {
			defer closeResponseBody(resp)
			return uploaded, fmt.Errorf("upload chunk, offset=%d: %w", us.uploadStateOffset, err)
		}
		if advance == 0 {
			defer closeResponseBody(resp)
			return uploaded, newTusError(ErrZeroProgress, resp, fmt.Errorf("offset=%d", us.uploadStateOffset))
		}
		closeResponseBody(resp)

		us.dirtyUnread = us.dirtyUnread[advance:]
		us.uploadStateOffset = newOffset
		us.uploadStateExpires = expires
		uploaded += int64(advance)
	}

	return uploaded, nil
}

func (us *UploadStream) setupDirtyBuffer() {
	if int64(len(us.dirtyBuffer)) != us.ChunkSize {
		us.dirtyBuffer = nil
		us.dirtyUnread = nil
	}
	if len(us.dirtyBuffer) == 0 && us.ChunkSize != NoChunked {
		us.dirtyBuffer = make([]byte, us.ChunkSize)
	}
}

func (us *UploadStream) uploadData(chunk []byte, stream io.Reader, offset int64, extraHeaders map[string]string) (int, int64, *time.Time, *http.Response, error) {
	// Limit the data source to the space left in the upload
	remoteFreeSpace := int(us.uploadSize - offset)
	switch {
	case remoteFreeSpace == 0:
		// TODO: return an error (and expire here is nil, but in fact we don't know)
		return 0, offset, nil, nil, nil // Upload is full, nothing to transfer
	case stream == nil && remoteFreeSpace < len(chunk):
		// Chunked mode
		chunk = chunk[:remoteFreeSpace]
	case stream != nil:
		// Streaming mode
		stream = io.LimitReader(stream, int64(remoteFreeSpace))
	}

	newOffset, expires, resp, err := us.doRequest(http.MethodPatch, us.location.String(), chunk, stream, offset, extraHeaders)
	if err != nil {
		return 0, offset, expires, resp, fmt.Errorf("request: %w", err)
	}

	advance := int(newOffset - offset)
	switch {
	case advance < 0:
		return advance, newOffset, expires, resp, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has gone backwards, offset=%d, newOffset=%d", offset, newOffset))
	case stream == nil && advance > len(chunk) || stream != nil && advance > remoteFreeSpace:
		return advance, newOffset, expires, resp, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has been advanced too far: offset=%d, newOffset=%d, transferredDataSize=%d", offset, newOffset, len(chunk)))
	}
	return advance, newOffset, expires, resp, nil
}

func (us *UploadStream) doRequest(method, requestURL string, chunk []byte, stream io.Reader, offset int64, extraHeaders map[string]string) (int64, *time.Time, *http.Response, error) {
	if err := us.validate(); err != nil {
		return offset, nil, nil, err
	}

	// Perform actions that can generate an error before invoking a reader
	if us.checksumHash != nil && stream != nil {
		if err := us.client.ensureExtension("checksum-trailer"); err != nil {
			return offset, nil, nil, err
		}
	}
	req, err := us.client.createRequest(method, requestURL)
	if err != nil {
		return offset, nil, nil, fmt.Errorf("create request: %w", err)
	}

	if us.checksumHash != nil {
		us.checksumHash.Reset()
		if stream == nil {
			us.checksumHash.Write(chunk)
			sum := us.checksumHash.Sum(make([]byte, 0))
			req.Header.Set("Upload-Checksum", fmt.Sprintf("%s %s", us.rawChecksumHashName, base64.StdEncoding.EncodeToString(sum)))
		} else {
			trailers := map[string]io.Reader{"Upload-Checksum": checksum.NewHashBase64ReadWriter(us.checksumHash, us.rawChecksumHashName+" ")}
			stream = checksum.NewDeferTrailerReader(io.TeeReader(stream, us.checksumHash), trailers, req)
		}
	}

	if stream == nil {
		req.Body = io.NopCloser(bytes.NewReader(chunk))
		req.ContentLength = int64(len(chunk))
	} else {
		req.Body = io.NopCloser(stream)
		req.ContentLength = -1 // Request body size is unknown in streamed mode. See https://pkg.go.dev/net/http#Request.ContentLength
	}
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))

	if us.SetUploadSize && offset == 0 {
		req.Header.Set("Upload-Length", strconv.FormatInt(us.uploadSize, 10))
	}

	if len(extraHeaders) > 0 {
		for k, v := range extraHeaders {
			if v == "" {
				req.Header.Del(k)
			} else {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := us.client.tusRequest(req)
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
		return offset, nil, resp, newTusError(ErrUploadTooLarge, resp, nil)
	case 460: // Non-standard HTTP code '460 Checksum Mismatch'
		if us.checksumHash != nil {
			return offset, nil, resp, newTusError(ErrChecksumMismatch, resp, nil)
		}
		// If checksum verification is not used, treat it as an unexpected response
	}

	return offset, nil, resp, newTusError(ErrUnexpectedResponse, resp, nil)
}

func (us *UploadStream) validate() error {
	if us.uploadSize == SizeUnknown {
		return fmt.Errorf("upload size must be set before uploading starts")
	}
	if us.uploadSize < 0 {
		panic(fmt.Sprintf("upload size is negative %d", us.uploadSize))
	}
	if us.SetUploadSize {
		if err := us.client.ensureExtension("creation-defer-length"); err != nil {
			return err
		}
	}
	if us.checksumHash != nil {
		if err := us.client.ensureExtension("checksum"); err != nil {
			return err
		}
	}
	if us.ChunkSize < 0 && us.ChunkSize != NoChunked {
		panic("ChunkSize must be either a positive number or NoChunked")
	}
	return nil
}
