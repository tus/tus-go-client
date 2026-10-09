package tusgo

import (
	"bytes"
	"context"
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

// NewUploadStream returns a new UploadStream that transfers the data using the given client. It accepts an Upload,
// taking the ownership of it and keeping its RemoteOffset up to date while the data is transferred.
// The stream inherits the context of the client.
func NewUploadStream(client *Client, upload *Upload) *UploadStream {
	if upload == nil {
		panic("upload is nil")
	}
	const chunkSize = 2 * 1024 * 1024
	return &UploadStream{
		ChunkSize:    chunkSize,
		Upload:       upload,
		client:       client,
		uploadMethod: http.MethodPatch,
		ctx:          client.ctx,
	}
}

// NoChunked when assigned to UploadStream.ChunkSize, disables the chunking of the transferred data.
const NoChunked = 0

// UploadStream is a write-only stream that transfers the data of an upload to a TUS server. It implements io.Writer,
// io.ReaderFrom and io.Seeker.
//
// The basic approach is following:
//
//  1. Create a fixed-size upload object on the server by Client, which produces an Upload object representing the upload
//  2. If the transfer was interrupted, call UploadStream.Sync to sync the stream offset with the server offset or
//     get the upload object from the server with Client.GetUpload, that also contains the current server offset
//  3. Transfer the data with an UploadStream
//  4. To resume an interrupted transfer, just call the same method again
//
// The server keeps the current data offset for every upload. TUS protocol demands that server data offset and offset
// in client requests must be always in sync. So, UploadStream keeps this offset in Upload.RemoteOffset field up to
// date while the data is transferred. If the two offsets differ, the server rejects the request and the stream
// returns [ErrOffsetsNotSynced] -- then call Sync to sync offsets and try again.
//
// By default, the data is transferred in chunks of ChunkSize bytes, one request per chunk, staged through an
// intermediate buffer called the dirty buffer. Setting ChunkSize to NoChunked disables the chunking: the data source
// is piped directly into the request body.
//
// To verify the transferred data with a checksum, obtain a stream with the WithChecksumAlgorithm method. The server
// must support the "checksum" extension and the chosen hash algorithm.
// With chunking disabled (when ChunkSize set to NoChunked), the checksum is calculated when the data reader is drained
// and is added to HTTP trailer; the server must also support the "checksum-trailer" extension in this case.
//
// The "Deferred length" feature is used for an upload created with an unknown size: the server expects the size to be
// reported in the first request that transfers the data. To do so, set Upload.RemoteSize to the actual size and
// SetUploadSize to true before the first write; the request made at offset 0 then carries the upload size.
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

	// LastResponse is a read-only field that keeps the last response this UploadStream has received from the server.
	// It is useful, for example, to inspect the response that has caused an error.
	LastResponse *http.Response

	// SetUploadSize enables stream to send the data size to the server with the first request.
	// Set it to true when transferring the data to uploads created without size (remoteSize has been set to SizeUnknown).
	// Before the first request, Upload.RemoteSize must be set to the actual size of the upload.
	//
	// The server must support the "creation-defer-length" extension.
	SetUploadSize bool

	// Upload is the upload the stream transfers the data of. The stream keeps its RemoteOffset up to date, so the
	// upload must not be modified from the outside while the stream exists.
	Upload *Upload

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
	uploadMethod        string
	ctx                 context.Context
}

// WithContext returns a copy of the stream that sends its requests with the given context. The copy is clean and has
// no last response.
func (us *UploadStream) WithContext(ctx context.Context) *UploadStream {
	res := *us
	res.LastResponse = nil
	res.dirtyBuffer = nil
	res.dirtyUnread = nil
	res.ctx = ctx
	return &res
}

// WithChecksumAlgorithm returns a copy of the stream that asks the server to verify the transferred data with the
// given hash algorithm. The copy is clean and has no last response.
//
// The name is matched against the algorithms listed in [checksum.Algorithms], ignoring the case and any non-alphanumeric
// characters, so "SHA-256" and "sha256" denote the same algorithm. It panics if the algorithm is not supported.
// The server must support the "checksum" extension and advertise the algorithm in
// [ServerCapabilities.ChecksumAlgorithms].
func (us *UploadStream) WithChecksumAlgorithm(name string) *UploadStream {
	res := *us
	res.LastResponse = nil
	res.dirtyBuffer = nil
	res.dirtyUnread = nil

	alg, ok := checksum.GetAlgorithm(name)
	if !ok {
		panic(fmt.Sprintf("checksum algorithm %q does not supported", name))
	}

	f := checksum.Algorithms[alg]
	res.checksumHash = f()
	res.rawChecksumHashName = name

	return &res
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
	// Server may confirm less data than has been read from r and sent, so send it until the server confirms all the data.
	if us.ChunkSize == NoChunked {
		// Streaming mode, pass the reader directly
		advance, newOffset, response, err := us.uploadData(nil, counterRd, nil)
		if response != nil {
			us.LastResponse = response
		}
		if err != nil {
			return counterRd.BytesRead, err
		}
		us.Upload.RemoteOffset = newOffset
		if advance == 0 && counterRd.BytesRead > 0 {
			return counterRd.BytesRead, newTusErrorWithErr(ErrZeroProgress, fmt.Errorf("offset=%d", us.Upload.RemoteOffset))
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

// Write transfers p, starting at the offset Upload.RemoteOffset, which is kept up to date while the transfer goes on.
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
		advance, newOffset, response, err := us.uploadData(nil, bytes.NewReader(p), nil)
		if response != nil {
			us.LastResponse = response
		}
		if err != nil {
			return advance, err
		}
		us.Upload.RemoteOffset = newOffset
		if advance < len(p) && newOffset == us.Upload.RemoteSize {
			return advance, io.ErrShortWrite // Upload is full, but there is still some data left in p.
		} else if advance == 0 {
			return 0, newTusErrorWithErr(ErrZeroProgress, fmt.Errorf("offset=%d", us.Upload.RemoteOffset))
		}

		return advance, nil
	}

	// Chunked mode
	n, err := us.uploadChunked(bytes.NewReader(p))
	if err != nil {
		return int(n), err
	}
	if n < int64(len(p)) && us.Upload.RemoteOffset == us.Upload.RemoteSize {
		return int(n), io.ErrShortWrite // Upload is full, but there is still some data left in p.
	}

	return int(n), nil
}

// Sync requests the current offset of the upload from the server and adopts it as the stream offset.
//
// Call it before starting a transfer or after the stream has returned ErrOffsetsNotSynced. The data source is not
// affected, so it has to be rewound to Tell separately before the transfer is resumed.
func (us *UploadStream) Sync() (response *http.Response, err error) {
	f := Upload{}
	if response, err = us.client.GetUpload(&f, us.Upload.Location); err == nil {
		us.Upload.RemoteOffset = f.RemoteOffset
	}
	us.LastResponse = response
	return
}

// Seek moves the stream offset, i.e. Upload.RemoteOffset, to the given position, interpreted according to whence:
// io.SeekStart means relative to the start of the upload, io.SeekCurrent to the current offset and io.SeekEnd to the
// last byte of the upload. It returns the new offset.
//
// Note that this only moves the local offset: the server keeps its own one, and it may reject the requests made at a
// position it does not expect. Use Sync to adopt the server offset instead.
func (us *UploadStream) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = us.Upload.RemoteOffset + offset
	case io.SeekEnd:
		newOffset = us.Upload.RemoteSize - 1 + offset
	default:
		panic(fmt.Sprintf("unknown whence value: %d", whence))
	}
	if offset >= us.Upload.RemoteSize {
		return newOffset, fmt.Errorf("offset %d exceeds the upload size %d bytes", newOffset, us.Upload.RemoteSize)
	}
	if offset < 0 {
		return newOffset, fmt.Errorf("offset %d is negative", newOffset)
	}
	us.Upload.RemoteOffset = newOffset
	return newOffset, nil
}

// Tell returns the current stream offset, i.e. Upload.RemoteOffset.
func (us *UploadStream) Tell() int64 {
	return us.Upload.RemoteOffset
}

// Len returns the size of the upload, i.e. Upload.RemoteSize.
func (us *UploadStream) Len() int64 {
	return us.Upload.RemoteSize
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
	remoteFreeSpace := us.Upload.RemoteSize - us.Upload.RemoteOffset
	r = io.LimitReader(r, remoteFreeSpace)
	us.setupDirtyBuffer()
	for us.Upload.RemoteOffset < us.Upload.RemoteSize {
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

		advance, newOffset, response, err := us.uploadData(us.dirtyUnread, nil, nil)
		if response != nil {
			us.LastResponse = response
		}
		if err != nil {
			return uploaded, fmt.Errorf("upload chunk, offset=%d: %w", us.Upload.RemoteOffset, err)
		}
		if advance == 0 {
			return uploaded, newTusErrorWithErr(ErrZeroProgress, fmt.Errorf("offset=%d", us.Upload.RemoteOffset))
		}

		us.dirtyUnread = us.dirtyUnread[advance:]
		us.Upload.RemoteOffset = newOffset
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

func (us *UploadStream) uploadData(chunk []byte, stream io.Reader, extraHeaders map[string]string) (int, int64, *http.Response, error) {
	offset := us.Upload.RemoteOffset
	loc, err := url.Parse(us.Upload.Location)
	if err != nil {
		return 0, offset, nil, fmt.Errorf("parse location: %w", err)
	}
	u := us.client.BaseURL.ResolveReference(loc).String()

	// Limit the data source to the space left in the upload
	remoteFreeSpace := int(us.Upload.RemoteSize - offset)
	switch {
	case remoteFreeSpace == 0:
		return 0, offset, nil, nil // Upload is full, nothing to transfer
	case stream == nil && remoteFreeSpace < len(chunk):
		// Chunked mode
		chunk = chunk[:remoteFreeSpace]
	case stream != nil:
		// Streaming mode
		stream = io.LimitReader(stream, int64(remoteFreeSpace))
	}

	newOffset, response, err := us.doRequest(u, chunk, stream, extraHeaders)
	advance := int(newOffset - offset)
	if err != nil {
		return advance, offset, response, fmt.Errorf("request: %w", err)
	}

	switch {
	case advance < 0:
		return advance, newOffset, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("server offset has gone backwards, offset=%d, newOffset=%d", offset, newOffset))
	case stream == nil && advance > len(chunk) || stream != nil && advance > remoteFreeSpace:
		return advance, newOffset, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("server offset has been advanced too far: offset=%d, newOffset=%d, transferredDataSize=%d", offset, newOffset, len(chunk)))
	}
	return advance, newOffset, response, nil
}

func (us *UploadStream) doRequest(requestURL string, chunk []byte, stream io.Reader, extraHeaders map[string]string) (offset int64, response *http.Response, err error) {
	if err = us.validate(); err != nil {
		return
	}

	offset = us.Upload.RemoteOffset

	// Perform actions that can generate an error before invoking a reader
	if us.checksumHash != nil && stream != nil {
		if err = us.client.ensureExtension("checksum-trailer"); err != nil {
			return
		}
	}
	var req *http.Request
	if req, err = us.client.GetRequest(us.uploadMethod, requestURL, nil, us.client, us.client.client); err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
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
		req.Header.Set("Upload-Length", strconv.FormatInt(us.Upload.RemoteSize, 10))
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

	if us.ctx != nil {
		req = req.WithContext(us.ctx)
	}
	if response, err = us.client.tusRequest(us.ctx, req); err != nil {
		return 0, response, fmt.Errorf("request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusCreated: // For "Creation With Upload" feature
		if us.uploadMethod != http.MethodPost {
			err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
			return
		}
		fallthrough
	case http.StatusNoContent:
		if offset, err = strconv.ParseInt(response.Header.Get("Upload-Offset"), 10, 64); err != nil {
			err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Offset header %q: %w", response.Header.Get("Upload-Offset"), err))
			return
		}
		if v := response.Header.Get("Upload-Expires"); v != "" {
			var t time.Time
			if t, err = time.Parse(time.RFC1123, v); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Expires RFC1123 header %q: %w", v, err))
				return
			}
			us.Upload.UploadExpired = &t
		}
	case http.StatusConflict:
		err = newTusErrorWithResponse(ErrOffsetsNotSynced, response)
	case http.StatusForbidden:
		err = newTusErrorWithResponse(ErrCannotUpload, response)
	case http.StatusNotFound, http.StatusGone:
		err = newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	case http.StatusRequestEntityTooLarge:
		err = newTusErrorWithResponse(ErrUploadTooLarge, response)
	case 460: // Non-standard HTTP code '460 Checksum Mismatch'
		if us.checksumHash != nil {
			err = newTusErrorWithResponse(ErrChecksumMismatch, response)
			return
		}
		fallthrough
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}
	return
}

func (us *UploadStream) validate() error {
	if us.Upload.RemoteSize == SizeUnknown {
		return fmt.Errorf("upload size must be set before uploading starts")
	}
	if us.Upload.RemoteSize < 0 {
		panic(fmt.Sprintf("upload size is negative %d", us.Upload.RemoteSize))
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
