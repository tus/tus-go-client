// Package tusgo is a client library for TUS (https://tus.io), the resumable upload protocol over HTTP.
//
// The work is split between two types. [Client] manages the uploads on a server: it creates, deletes and queries them,
// concatenates the partial ones and reports the server capabilities. [UploadStream] transfers the data of a single
// upload; it implements [io.Writer], [io.ReaderFrom] and [io.Seeker].
// [Upload] represents a particular upload object on the server and its state.
//
// The basic approach is following: first, create a fixed-size upload object on the server by Client,
// and then transfer the data to the returned Upload using the UploadStream. To resume an interrupted transfer,
// just call the same method again.
//
// The protocol also defines a number of extensions -- optional features that customize this process:
// deleting an upload, concatenating several uploads into a single one or verifying the transferred data with a
// checksum, etc. A server supports any subset of them, so the features backed by an extension fail with
// [ErrUnsupportedFeature] unless the server announces it, see [Client.Capabilities].
package tusgo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultProtocolVersion = "1.0.0"

// NewClient returns a new [Client] that sends its requests to the given URL.
func NewClient(endpoint *url.URL) *Client {
	if endpoint == nil {
		panic("endpoint is nil")
	}
	return &Client{
		ProtocolVersion: DefaultProtocolVersion,
		ExtraHeaders:    make(http.Header),
		endpoint:        endpoint,
	}
}

// Client manages the uploads on a TUS server: it creates, deletes and queries them, and concatenates partial uploads.
// It does not transfer the upload data itself, use [UploadStream] for that.
// Client is safe for concurrent use by multiple goroutines.
//
// Common errors returned by the Client methods are:
//
//   - [ErrUnsupportedFeature] -- the requested action needs a TUS extension that the server does not support
//   - [ErrProtocol] -- an otherwise successful server response contains malformed data, e.g. a missing required HTTP header
//   - [ErrUnexpectedResponse] -- the server responded with an unexpected status code
type Client struct {
	// ProtocolVersion is the TUS protocol version sent in the requests. By default, it's equal to [DefaultProtocolVersion].
	ProtocolVersion string

	// Capabilities are the features and limits of the server. Use [Client.UpdateCapabilities] to query them from the server.
	Capabilities *ServerCapabilities

	// ExtraHeaders are the additional HTTP headers sent with every request to the server.
	// They may be used for authentication, cache control, etc.
	ExtraHeaders http.Header

	// DoRequest is called on every request, that sends a prepared TUS [http.Request] to the server and returns the
	// [http.Response]. It may be called concurrently if the Client is used by several goroutines.
	// If nil, the library uses the Client field or [http.DefaultClient].
	//
	// This function may be used for testing, logging, collecting the metrics, shaping the connection speed,
	// modifying the request before sending it, etc.
	DoRequest DoRequestFunc

	// Client is the HTTP client used to send requests to the server. If nil, [http.DefaultClient] is used.
	// This field is ignored if DoRequest is set.
	Client *http.Client

	endpoint *url.URL
}

// DoRequestFunc is the callback type for sending a TUS request to the server and returning the response.
// It may be called concurrently if the Client is used by several goroutines. It receives a prepared TUS [http.Request]
// and returns the [http.Response] or an error.
type DoRequestFunc func(req *http.Request) (*http.Response, error)

// GetUpload requests the server for the upload information by its location.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) GetUpload(location string) (Upload, error) {
	res := Upload{Location: location}

	loc, err := url.Parse(location)
	if err != nil {
		return res, fmt.Errorf("parse location: %w", err)
	}
	loc = c.endpoint.ResolveReference(loc)

	req, err := c.createRequest(http.MethodHead, loc.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("send request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		res.Partial = resp.Header.Get("Upload-Concat") == "partial"
		uploadOffset := resp.Header.Get("Upload-Offset")
		// Upload-Offset may not be present if final upload concatenation still in progress on server side
		if uploadOffset == "" {
			if resp.Header.Get("Upload-Concat") != "final" {
				return res, newTusError(ErrProtocol, resp, errors.New("lack of Upload-Offset required header in response"))
			}
			res.RemoteOffset = OffsetUnknown
		} else {
			if res.RemoteOffset, err = strconv.ParseInt(uploadOffset, 10, 64); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Offset header %q: %w", uploadOffset, err))
			}
		}
		// Responses for final concatenated upload may contain Upload-Length header
		if v := resp.Header.Get("Upload-Length"); v != "" {
			if res.RemoteSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Length header %q: %w", v, err))
			}
		}
		if v := resp.Header.Get("Upload-Metadata"); v != "" {
			if res.Metadata, err = DecodeMetadata(v); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Metadata header %q: %w", v, err))
			}
		}
		return res, nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return res, newTusError(ErrUploadDoesNotExist, resp, nil)
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

// CreateUpload creates an upload of the given size, and returns an [Upload] object.
// The server must advertise the "creation" extension.
//
// If remoteSize is [SizeUnknown], the upload is created with a deferred size, i.e. with a size that is not known yet,
// but has to be known by the time the data transfer begins. The server must also advertise the
// "creation-defer-length" extension for this feature, and [UploadStream.SetUploadSize] must be set before the transfer.
//
// If partial arg is true, the "partial" upload will be created, which may be used to upload data in parallel, see
// [Client.ConcatenateUploads]. The given metadata will be attached to the upload and can be retrieved later.
//
// The method returns [ErrUploadTooLarge] if the upload size exceeds [ServerCapabilities.MaxSize] the server is able to
// accept.
func (c *Client) CreateUpload(remoteSize int64, partial bool, metadata map[string]string) (Upload, error) {
	var res Upload

	if err := c.ensureExtension("creation"); err != nil {
		return res, err
	}

	req, err := c.createRequest(http.MethodPost, c.endpoint.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Length", strconv.FormatInt(0, 10))
	if partial {
		req.Header.Set("Upload-Concat", "partial")
	}
	switch {
	case remoteSize == SizeUnknown:
		if err = c.ensureExtension("creation-defer-length"); err != nil {
			return res, err
		}
		req.Header.Set("Upload-Defer-Length", "1")
	case remoteSize > 0:
		req.Header.Set("Upload-Length", strconv.FormatInt(remoteSize, 10))
	default:
		panic(fmt.Sprintf("remoteSize is negative: %d", remoteSize))
	}

	if len(metadata) > 0 {
		var m string
		if m, err = EncodeMetadata(metadata); err != nil {
			return res, fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("send request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		res.Location = resp.Header.Get("Location")
		res.Metadata = metadata
		res.Partial = partial
		res.RemoteSize = remoteSize
		if v := resp.Header.Get("Upload-Expires"); v != "" {
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse RFC1123 datetime in Upload-Expires header %q: %w", v, err))
			}
			res.UploadExpired = &t
		}
		return res, nil
	case http.StatusRequestEntityTooLarge:
		return res, newTusError(ErrUploadTooLarge, resp, nil)
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

// CreateUploadWithData creates an upload on the server and transfers data in the same HTTP request.
// It returns the number of bytes been uploaded and the [Upload] object.
// The whole data is sent in a single request, so the server must advertise the "creation-with-upload" extension.
//
// The remoteSize, partial and metadata parameters have the same meaning as in CreateUpload.
//
// If the server did not accept any data by some reason, the method returns a correct Upload and [ErrZeroProgress] error.
// In that case, use [UploadStream] to transfer the remaining data.
func (c *Client) CreateUploadWithData(data []byte, remoteSize int64, partial bool, metadata map[string]string) (int64, Upload, error) {
	var res Upload

	if err := c.ensureExtension("creation-with-upload"); err != nil {
		return 0, res, err
	}

	s := NewUploadStream(c, &res)
	s.ChunkSize = int64(len(data)) // Data must be uploaded in one request as single chunk
	s.uploadMethod = http.MethodPost
	headers := map[string]string{"Upload-Length": strconv.Itoa(int(remoteSize)), "Upload-Offset": ""}
	if partial {
		headers["Upload-Concat"] = "partial"
	}
	if len(metadata) > 0 {
		m, err := EncodeMetadata(metadata)
		if err != nil {
			return 0, res, fmt.Errorf("encode metadata: %w", err)
		}
		headers["Upload-Metadata"] = m
	}
	res.RemoteSize = remoteSize
	res.Partial = partial
	res.Metadata = metadata

	uploadedBytes, resp, err := s.doRequest(c.endpoint.String(), data, nil, headers) // Upload in one request
	res.RemoteOffset = uploadedBytes
	defer closeResponseBody(resp)
	if err != nil {
		return uploadedBytes, res, fmt.Errorf("send request: %w", err)
	}
	res.Location = resp.Header.Get("Location")

	switch {
	case uploadedBytes == 0 && len(data) > 0:
		return uploadedBytes, res, newTusError(ErrZeroProgress, resp, fmt.Errorf("offset=%d", res.RemoteOffset))
	case uploadedBytes < 0:
		return uploadedBytes, res, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has gone backwards, offset=%d, transferredDataSize=%d", uploadedBytes, len(data)))
	case uploadedBytes > int64(len(data)):
		return uploadedBytes, res, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has been advanced too far: offset=%d, transferredDataSize=%d", uploadedBytes, len(data)))
	}

	return uploadedBytes, res, nil
}

// DeleteUpload deletes the upload at the given location from the server. The server must advertise the
// "termination" extension.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) DeleteUpload(location string) error {
	if err := c.ensureExtension("termination"); err != nil {
		return err
	}

	loc, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("parse location: %w", err)
	}
	loc = c.endpoint.ResolveReference(loc)
	req, err := c.createRequest(http.MethodDelete, loc.String())
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return newTusError(ErrUploadDoesNotExist, resp, nil)
	}
	return newTusError(ErrUnexpectedResponse, resp, nil)
}

// ConcatenateUploads requests the server to concatenate the *finished* "partial" uploads into a single final
// upload, which is returned from method. Optional metadata is attached to the final upload.
// The server must advertise the "concatenation" extension.
//
// The partial uploads are concatenated in the order they are given.
func (c *Client) ConcatenateUploads(partials []Upload, metadata map[string]string) (Upload, error) {
	var res Upload

	if len(partials) == 0 {
		return res, fmt.Errorf("partial uploads list is empty")
	}
	if err := c.ensureExtension("concatenation"); err != nil {
		return res, err
	}

	req, err := c.createRequest(http.MethodPost, c.endpoint.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}

	locations := make([]string, 0)
	for _, f := range partials {
		if !f.Partial {
			return res, fmt.Errorf("upload %q is not partial", f.Location)
		}
		locations = append(locations, f.Location)
	}
	req.Header.Set("Upload-Concat", "final;"+strings.Join(locations, " "))

	if len(metadata) > 0 {
		m, err := EncodeMetadata(metadata)
		if err != nil {
			return res, fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("send request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		res.Location = resp.Header.Get("Location")
		res.Metadata = metadata
		return res, nil
	case http.StatusNotFound, http.StatusGone:
		return res, newTusError(ErrUploadDoesNotExist, resp, nil)
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

// ConcatenateStreams requests the server to concatenate the streams, finished or not, into a single final upload,
// which is returned from the method. Optional metadata is attached to the final upload.
//
// If all streams are finished at the moment, then functions the same as [Client.ConcatenateUploads].
//
// If some streams are unfinished, the server is asked to concatenate them automatically once they will be finished.
// The server must advertise the "concatenation-unfinished" extension for that.
func (c *Client) ConcatenateStreams(streams []*UploadStream, metadata map[string]string) (Upload, error) {
	if len(streams) == 0 {
		return Upload{}, fmt.Errorf("streams list is empty")
	}

	uploads := make([]Upload, 0)
	for i, s := range streams {
		if s.Tell() < s.Len() {
			if err := c.ensureExtension("concatenation-unfinished"); err != nil {
				return Upload{}, fmt.Errorf("stream #%d is not finished: %w", i, err)
			}
		}
		uploads = append(uploads, *s.Upload)
	}

	return c.ConcatenateUploads(uploads, metadata)
}

// UpdateCapabilities queries the server for its features and limits and stores them in the [Client.Capabilities] field.
func (c *Client) UpdateCapabilities() (resp *http.Response, err error) {
	var req *http.Request
	if req, err = c.createRequest(http.MethodOptions, c.endpoint.String()); err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err = c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		c.Capabilities = &ServerCapabilities{}
		if v := resp.Header.Get("Tus-Max-Size"); v != "" {
			if c.Capabilities.MaxSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				err = newTusError(ErrProtocol, resp, fmt.Errorf("cannot parse Tus-Max-Size integer value %q: %w", v, err))
				return
			}
		}
		if v := resp.Header.Get("Tus-Extension"); v != "" {
			c.Capabilities.Extensions = strings.Split(v, ",")
		}
		if v := resp.Header.Get("Tus-Version"); v != "" {
			c.Capabilities.ProtocolVersions = strings.Split(v, ",")
		}
		if v := resp.Header.Get("Tus-Checksum-Algorithm"); v != "" {
			c.Capabilities.ChecksumAlgorithms = strings.Split(v, ",")
		}
	default:
		err = newTusError(ErrUnexpectedResponse, resp, nil)
	}
	return
}

func (c *Client) tusRequest(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodOptions && req.Header.Get("Tus-Resumable") == "" {
		req.Header.Set("Tus-Resumable", c.ProtocolVersion)
	}

	f := c.DoRequest
	if f == nil {
		cl := c.Client
		if cl == nil {
			cl = http.DefaultClient
		}
		f = cl.Do
	}
	resp, err := f(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		versions := resp.Header.Get("Tus-Version")
		return resp, newTusError(ErrProtocol, resp, fmt.Errorf("server does not support protocol version %q, supported versions: %s", c.ProtocolVersion, versions))
	}

	return resp, nil
}

func (c *Client) ensureExtension(extension string) error {
	if c.Capabilities == nil {
		if _, err := c.UpdateCapabilities(); err != nil {
			return fmt.Errorf("obtain server capabilities: %w", err)
		}
	}
	for _, e := range c.Capabilities.Extensions {
		if extension == e {
			return nil
		}
	}
	return newTusError(ErrUnsupportedFeature, nil, errors.New(extension))
}

func (c *Client) createRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if len(c.ExtraHeaders) > 0 {
		req.Header = c.ExtraHeaders.Clone()
	}
	return req, nil
}

// EncodeMetadata encodes a metadata map into the TUS `Upload-Metadata` header format.
func EncodeMetadata(metadata map[string]string) (string, error) {
	var encoded []string

	for k, v := range metadata {
		switch {
		case k == "":
			return "", fmt.Errorf("empty key")
		case strings.Contains(k, " "):
			return "", fmt.Errorf("key %q contains spaces", k)
		case strings.Contains(k, ","):
			return "", fmt.Errorf("key %q contains commas", k)
		}
		encoded = append(encoded, fmt.Sprintf("%s %s", k, base64.StdEncoding.EncodeToString([]byte(v))))
	}

	return strings.Join(encoded, ","), nil
}

// DecodeMetadata decodes a metadata map from the TUS `Upload-Metadata` header format.
func DecodeMetadata(raw string) (map[string]string, error) {
	res := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		// The space separating the key and the value may be omitted if the value is empty
		key, encoded, _ := strings.Cut(item, " ")
		if key == "" {
			return res, fmt.Errorf("metadata item %q has bad format", item)
		}
		val, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return res, err
		}
		res[key] = string(val)
	}

	return res, nil
}

func closeResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}
